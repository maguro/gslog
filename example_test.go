// Copyright 2024 The original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gslog_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"cloud.google.com/go/logging"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/encoding/protojson"
	spb "google.golang.org/protobuf/types/known/structpb"

	"m4o.io/gslog"
	"m4o.io/gslog/k8s"
	"m4o.io/gslog/otel"
)

func ExampleNewClient() {
	ctx := context.Background()
	client, err := logging.NewClient(ctx, "my-project")
	if err != nil {
		// TODO: Handle error.
	}

	lg := client.Logger("my-log")

	h := gslog.NewGcpHandler(lg)
	l := slog.New(h)

	l.Info("How now brown cow?")
}

var (
	pw           = Password("pass-12334")
	pwObfuscated = slog.StringValue("<secret>")
	u            = &User{
		ID:        "user-12234",
		FirstName: "Jan",
		LastName:  "Doe",
		Email:     "jan@example.com",
		Password:  pw,
		Age:       32,
		Height:    5.91,
		Engineer:  true,
	}
)

type Manager struct {
}

type Password string

func (p Password) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote("<secret>")), nil
}

func (p Password) LogValue() slog.Value {
	return pwObfuscated
}

type User struct {
	ID        string   `json:"id"`
	FirstName string   `json:"first_name"`
	LastName  string   `json:"last_name"`
	Email     string   `json:"email"`
	Password  Password `json:"password"`
	Age       uint8    `json:"age"`
	Height    float32  `json:"height"`
	Engineer  bool     `json:"engineer"`
	Manager   *Manager `json:"manager"`
}

func PrintJsonPayload(e logging.Entry) {
	b, _ := protojson.Marshal(e.Payload.(*spb.Struct))
	// another JSON round-trip because protojson randomizes output
	var j map[string]interface{}
	_ = json.Unmarshal(b, &j)
	b, _ = json.Marshal(j)
	fmt.Println(string(b))
}

func ExampleLogger_payloadMapping() {
	h := gslog.NewGcpHandler(gslog.LoggerFunc(PrintJsonPayload))
	l := slog.New(h)
	l = l.WithGroup("pub")
	l = l.With(slog.Any("user", u))

	l.Info("How now brown cow?")

	// Output: {"message":"How now brown cow?","pub":{"user":{"age":32,"email":"jan@example.com","engineer":true,"first_name":"Jan","height":5.91,"id":"user-12234","last_name":"Doe","manager":null,"password":"\u003csecret\u003e"}}}
}

func PrintLabels(e logging.Entry) {
	keys := make([]string, 0)
	for k := range e.Labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		if sb.Len() > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(k + "=" + e.Labels[k])
	}

	fmt.Println(sb.String())
}

func ExampleLogger_withLabels() {
	h := gslog.NewGcpHandler(gslog.LoggerFunc(PrintLabels))
	l := slog.New(h)

	ctx := context.Background()
	ctx = gslog.WithLabels(ctx, gslog.Label("a", "one"), gslog.Label("b", "two"))

	l.Log(ctx, slog.LevelInfo, "How now brown cow?")

	// Output: a=one, b=two
}

func ExampleLogger_withK8sPodinfo() {
	h := gslog.NewGcpHandler(gslog.LoggerFunc(PrintLabels), k8s.WithPodinfoLabels("testdata/etc/podinfo"))
	l := slog.New(h)

	ctx := context.Background()
	ctx = gslog.WithLabels(ctx, gslog.Label("a", "one"), gslog.Label("b", "two"))

	l.Log(ctx, slog.LevelInfo, "How now brown cow?")

	// Output: a=one, b=two, k8s-pod/app=hello-world, k8s-pod/environment=stg, k8s-pod/tier=backend, k8s-pod/track=stable
}

func ExampleLogger_withOpentelemetryBaggage() {
	h := gslog.NewGcpHandler(gslog.LoggerFunc(PrintJsonPayload), otel.WithOtelBaggage())
	l := slog.New(h)

	ctx := context.Background()
	ctx = baggage.ContextWithBaggage(ctx, otel.MustParse("a=one,b=two;p1;p2=val2"))

	l.Log(ctx, slog.LevelInfo, "How now brown cow?")

	// Output: {"message":"How now brown cow?","otel-baggage/a":"one","otel-baggage/b":{"properties":{"p1":null,"p2":"val2"},"value":"two"}}
}

func PrintTracing(e logging.Entry) {
	var sb strings.Builder

	sb.WriteString("traceparent: 00-")
	sb.WriteString(e.Trace)
	sb.WriteString("-")
	sb.WriteString(e.SpanID)
	sb.WriteString("-")
	if e.TraceSampled {
		sb.WriteString("01")
	} else {
		sb.WriteString("00")
	}

	fmt.Println(sb.String())
}

func ExampleLogger_withOpentelemetryTrace() {
	h := gslog.NewGcpHandler(gslog.LoggerFunc(PrintTracing), otel.WithOtelTracing())
	l := slog.New(h)

	traceId, _ := trace.TraceIDFromHex("52fc1643a9381fc674742bb0067101e7")
	spanId, _ := trace.SpanIDFromHex("d3e9e8c51cb190df")

	ctx := context.Background()
	ctx = trace.ContextWithRemoteSpanContext(ctx, trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceId,
		SpanID:     spanId,
		TraceFlags: trace.FlagsSampled,
	}))

	l.Log(ctx, slog.LevelInfo, "How now brown cow?")

	// Output: traceparent: 00-52fc1643a9381fc674742bb0067101e7-d3e9e8c51cb190df-01
}