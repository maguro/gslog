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
	"os"
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

// A gslog.GcpHandler is created with a GCP logging.Logger.  The handler will
// map slog.Record records to logging.Entry entries, subsequently passing the
// resulting entries to its configured logging.Logger instance's Log() method.
func ExampleNewGcpHandler() {
	ctx := context.Background()
	client, err := logging.NewClient(ctx, "my-project")
	if err != nil {
		// TODO: Handle error.
	}

	lg := client.Logger("my-log")

	lg.Flush()

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

type Manager struct{}

// Password is a specialised type whose fmt.Stringer, json.Marshaler, and
// slog.LogValuer implementations return an obfuscated value.
type Password string

func (p Password) String() string {
	return "<secret>"
}

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

// PrintJsonPayload is a gslog.Logger stub that prints the logging.Entry
// Payload field as a JSON string.
func PrintJsonPayload(e logging.Entry) {
	b, _ := protojson.Marshal(e.Payload.(*spb.Struct))
	// another JSON round-trip because protojson randomizes output
	var j map[string]interface{}
	_ = json.Unmarshal(b, &j)
	b, _ = json.Marshal(j)
	fmt.Println(string(b))
}

// The gslog.GcpHandler maps the slog.Record and the handler's nested group
// attributes into a JSON object, with the logged message keyed at the root
// with the key "message".
func ExampleGcpHandler_Handle_payloadMapping() {
	h := gslog.NewGcpHandler(gslog.LoggerFunc(PrintJsonPayload))
	l := slog.New(h)
	l = l.WithGroup("pub")
	l = l.With(slog.Any("user", u))

	l.Info("How now brown cow?")

	// Output: {"message":"How now brown cow?","pub":{"user":{"age":32,"email":"jan@example.com","engineer":true,"first_name":"Jan","height":5.91,"id":"user-12234","last_name":"Doe","manager":null,"password":"\u003csecret\u003e"}}}
}

// PrintLabels is a gslog.Logger stub that prints the logging.Entry's
// Labels field.
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

// The gslog.GcpHandler will add any labels found in the context to the
// logging.Entry's Labels field.
func ExampleGcpHandler_Handle_withLabels() {
	h := gslog.NewGcpHandler(gslog.LoggerFunc(PrintLabels))
	l := slog.New(h)

	ctx := context.Background()
	ctx = gslog.WithLabels(ctx, gslog.Label("a", "one"), gslog.Label("b", "two"))

	l.Log(ctx, slog.LevelInfo, "How now brown cow?")

	// Output: a=one, b=two
}

// When configured via k8s.WithPodinfoLabels(), gslog.GcpHandler will include
// labels from the configured Kubernetes Downward API podinfo labels file to
// the logging.Entry's Labels field.
//
// The labels are prefixed with "k8s-pod/" to adhere to the Google Cloud
// Logging conventions for Kubernetes Pod labels.
func ExampleNewGcpHandler_withK8sPodinfo() {
	h := gslog.NewGcpHandler(gslog.LoggerFunc(PrintLabels), k8s.WithPodinfoLabels("k8s/testdata/etc/podinfo"))
	l := slog.New(h)

	ctx := context.Background()
	ctx = gslog.WithLabels(ctx, gslog.Label("a", "one"), gslog.Label("b", "two"))

	l.Log(ctx, gslog.LevelCritical, "Danger, Will Robinson!")

	// Output: a=one, b=two, k8s-pod/app=hello-world, k8s-pod/environment=stg, k8s-pod/tier=backend, k8s-pod/track=stable
}

// When configured via otel.WithOtelBaggage(), gslog.GcpHandler will include
// any baggage.Baggage attached to the context as attributes.
//
// The baggage keys are prefixed with "otel-baggage/" to mitigate collision
// with other log attributes and have precedence over any collisions with
// preexisting attributes.
func ExampleNewGcpHandler_withOpentelemetryBaggage() {
	h := gslog.NewGcpHandler(gslog.LoggerFunc(PrintJsonPayload), otel.WithOtelBaggage())
	l := slog.New(h)

	ctx := context.Background()
	ctx = baggage.ContextWithBaggage(ctx, otel.MustParse("a=one,b=two;p1;p2=val2"))

	l.Log(ctx, slog.LevelInfo, "How now brown cow?")

	// Output: {"message":"How now brown cow?","otel-baggage/a":"one","otel-baggage/b":{"properties":{"p1":null,"p2":"val2"},"value":"two"}}
}

// PrintTracing is a gslog.Logger stub that prints the logging.Entry's
// tracing fields.
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

// When configured via otel.WithOtelTracing(), gslog.GcpHandler will include
// any OpenTelemetry trace.SpanContext information associated with the context
// in the logging.Entry's tracing fields.
func ExampleNewGcpHandler_withOpentelemetryTrace() {
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

// PrintSourceLocation is a gslog.Logger stub that prints the logging.Entry's
// SourceLocation field.
func PrintSourceLocation(e logging.Entry) {
	sl := e.SourceLocation
	sl.File = sl.File[len(sl.File)-len("gslog/example_test.go"):]

	b, _ := protojson.Marshal(sl)
	// another JSON round-trip because protojson randomizes output
	var j map[string]interface{}
	_ = json.Unmarshal(b, &j)
	b, _ = json.Marshal(j)
	fmt.Println(string(b))
}

// When configured via gslog.WithSourceAdded(), gslog.GcpHandler will include
// the computationally expensive SourceLocation field in the logging.Entry.
func ExampleNewGcpHandler_withSourceAdded() {
	h := gslog.NewGcpHandler(gslog.LoggerFunc(PrintSourceLocation), gslog.WithSourceAdded())
	l := slog.New(h)

	l.Log(ctx, slog.LevelInfo, "How now brown cow?")

	// Output: {"file":"gslog/example_test.go","function":"m4o.io/gslog_test.ExampleNewGcpHandler_withSourceAdded","line":"259"}
}

// RemovePassword is a gslog.AttrMapper that elides password attributes.
func RemovePassword(_ []string, a slog.Attr) slog.Attr {
	if a.Key == "password" {
		return slog.Attr{}
	}
	return a
}

// When configured via gslog.WithReplaceAttr(), gslog.GcpHandler will apply
// the supplied gslog.AttrMapper to all non-group attributes before they
// are logged.
func ExampleNewGcpHandler_withReplaceAttr() {
	h := gslog.NewGcpHandler(gslog.LoggerFunc(PrintJsonPayload), gslog.WithReplaceAttr(RemovePassword))
	l := slog.New(h)
	l = l.WithGroup("pub")
	l = l.With(slog.String("username", "user-12234"), slog.String("password", string(pw)))

	l.Info("How now brown cow?")

	// Output: {"message":"How now brown cow?","pub":{"username":"user-12234"}}
}

// When configured via gslog.WithLogLeveler(), gslog.GcpHandler use the
// slog.Leveler for logging level enabled checks.
func ExampleNewGcpHandler_withLogLeveler() {
	h := gslog.NewGcpHandler(gslog.LoggerFunc(PrintJsonPayload), gslog.WithLogLeveler(slog.LevelError))
	l := slog.New(h)

	l.Info("How now brown cow?")
	l.Error("The rain in Spain lies mainly on the plane.")

	// Output: {"message":"The rain in Spain lies mainly on the plane."}
}

// When configured via gslog.WithLogLevelFromEnvVar(), gslog.GcpHandler obtains
// its log level from tne environmental variable specified by the key.
func ExampleNewGcpHandler_withLogLevelFromEnvVar() {
	const envVar = "FOO_LOG_LEVEL"
	_ = os.Setenv(envVar, "ERROR")
	defer func() {
		_ = os.Unsetenv(envVar)
	}()

	h := gslog.NewGcpHandler(gslog.LoggerFunc(PrintJsonPayload), gslog.WithLogLevelFromEnvVar(envVar))
	l := slog.New(h)

	l.Info("How now brown cow?")
	l.Error("The rain in Spain lies mainly on the plane.")

	// Output: {"message":"The rain in Spain lies mainly on the plane."}
}

// A default log level configured via gslog.WithDefaultLogLeveler().
func ExampleNewGcpHandler_withDefaultLogLeveler() {
	const envVar = "FOO_LOG_LEVEL"

	h := gslog.NewGcpHandler(
		gslog.LoggerFunc(PrintJsonPayload),
		gslog.WithLogLevelFromEnvVar(envVar),
		gslog.WithDefaultLogLeveler(slog.LevelError),
	)
	l := slog.New(h)

	l.Info("How now brown cow?")
	l.Error("The rain in Spain lies mainly on the plane.")

	// Output: {"message":"The rain in Spain lies mainly on the plane."}
}
