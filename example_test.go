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

// The example creates a gslog.GcpHandler with a GCP logging.Logger.  The
// handler maps each slog.Record to a logging.Entry.  The handler then passes
// the entry to the Log() method of its logging.Logger.
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

// Password is a type that implements fmt.Stringer, json.Marshaler, and
// slog.LogValuer.  Each implementation returns an obfuscated value.
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
	// Do another JSON round-trip, because protojson randomizes its output.
	var j map[string]interface{}
	_ = json.Unmarshal(b, &j)
	b, _ = json.Marshal(j)
	fmt.Println(string(b))
}

// The gslog.GcpHandler maps the slog.Record and the nested group attributes
// of the handler into a JSON object.  The message is at the root of the
// object with the key "message".
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

// The gslog.GcpHandler adds the labels in the context to the Labels field of
// the logging.Entry.
func ExampleGcpHandler_Handle_withLabels() {
	h := gslog.NewGcpHandler(gslog.LoggerFunc(PrintLabels))
	l := slog.New(h)

	ctx := context.Background()
	ctx = gslog.WithLabels(ctx, gslog.Label("a", "one"), gslog.Label("b", "two"))

	l.Log(ctx, slog.LevelInfo, "How now brown cow?")

	// Output: a=one, b=two
}

// With the k8s.WithPodinfoLabels() option, gslog.GcpHandler adds labels from
// the configured Kubernetes Downward API podinfo labels file to the Labels
// field of the logging.Entry.
//
// The handler adds the prefix "k8s-pod/" to each label.  This follows the
// Google Cloud Logging conventions for Kubernetes Pod labels.
func ExampleNewGcpHandler_withK8sPodinfo() {
	h := gslog.NewGcpHandler(gslog.LoggerFunc(PrintLabels), k8s.WithPodinfoLabels("k8s/testdata/etc/podinfo"))
	l := slog.New(h)

	ctx := context.Background()
	ctx = gslog.WithLabels(ctx, gslog.Label("a", "one"), gslog.Label("b", "two"))

	l.Log(ctx, gslog.LevelCritical, "Danger, Will Robinson!")

	// Output: a=one, b=two, k8s-pod/app=hello-world, k8s-pod/environment=stg, k8s-pod/tier=backend, k8s-pod/track=stable
}

// With the otel.WithOtelBaggage() option, gslog.GcpHandler adds the
// baggage.Baggage in the context as attributes.
//
// The handler adds the prefix "otel-baggage/" to each baggage key.  The
// prefix makes collisions with other log attributes less likely.  A baggage
// attribute has precedence over an attribute that already has the same key.
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

	sb.WriteString("trace: ")
	sb.WriteString(e.Trace)
	sb.WriteString(" span: ")
	sb.WriteString(e.SpanID)
	sb.WriteString(" flags: ")
	if e.TraceSampled {
		sb.WriteString("01")
	} else {
		sb.WriteString("00")
	}

	fmt.Println(sb.String())
}

// With the otel.WithOtelTracing() option, gslog.GcpHandler adds the
// OpenTelemetry trace.SpanContext information in the context to the tracing
// fields of the logging.Entry.
func ExampleNewGcpHandler_withOpentelemetryTrace() {
	h := gslog.NewGcpHandler(gslog.LoggerFunc(PrintTracing), otel.WithOtelTracing("my-project"))
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

	// Output: trace: projects/my-project/traces/52fc1643a9381fc674742bb0067101e7 span: d3e9e8c51cb190df flags: 01
}

// PrintSourceLocation is a gslog.Logger stub that prints the logging.Entry's
// SourceLocation field.
func PrintSourceLocation(e logging.Entry) {
	sl := e.SourceLocation
	sl.File = sl.File[len(sl.File)-len("gslog/example_test.go"):]

	b, _ := protojson.Marshal(sl)
	// Do another JSON round-trip, because protojson randomizes its output.
	var j map[string]interface{}
	_ = json.Unmarshal(b, &j)
	b, _ = json.Marshal(j)
	fmt.Println(string(b))
}

// With the gslog.WithSourceAdded() option, gslog.GcpHandler adds the
// SourceLocation field to the logging.Entry.  This field is expensive to
// compute.
func ExampleNewGcpHandler_withSourceAdded() {
	h := gslog.NewGcpHandler(gslog.LoggerFunc(PrintSourceLocation), gslog.WithSourceAdded())
	l := slog.New(h)

	l.Log(ctx, slog.LevelInfo, "How now brown cow?")

	// Output: {"file":"gslog/example_test.go","function":"m4o.io/gslog_test.ExampleNewGcpHandler_withSourceAdded","line":"260"}
}

// RemovePassword is a gslog.AttrMapper that elides password attributes.
func RemovePassword(_ []string, a slog.Attr) slog.Attr {
	if a.Key == "password" {
		return slog.Attr{}
	}
	return a
}

// With the gslog.WithReplaceAttr() option, gslog.GcpHandler applies the
// supplied gslog.AttrMapper to each non-group attribute before the handler
// logs the attribute.
func ExampleNewGcpHandler_withReplaceAttr() {
	h := gslog.NewGcpHandler(gslog.LoggerFunc(PrintJsonPayload), gslog.WithReplaceAttr(RemovePassword))
	l := slog.New(h)
	l = l.WithGroup("pub")
	l = l.With(slog.String("username", "user-12234"), slog.String("password", string(pw)))

	l.Info("How now brown cow?")

	// Output: {"message":"How now brown cow?","pub":{"username":"user-12234"}}
}

// With the gslog.WithLogLeveler() option, gslog.GcpHandler uses the
// slog.Leveler to decide whether a log level is enabled.
func ExampleNewGcpHandler_withLogLeveler() {
	h := gslog.NewGcpHandler(gslog.LoggerFunc(PrintJsonPayload), gslog.WithLogLeveler(slog.LevelError))
	l := slog.New(h)

	l.Info("How now brown cow?")
	l.Error("The rain in Spain lies mainly on the plane.")

	// Output: {"message":"The rain in Spain lies mainly on the plane."}
}

// With the gslog.WithLogLevelFromEnvVar() option, gslog.GcpHandler reads its
// log level from the environment variable that the key names.
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

// The gslog.WithDefaultLogLeveler() option sets a default log level.
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
