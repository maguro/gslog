// Copyright the original author or authors.
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

package gcp_test

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

	"m4o.io/gslog/core"
	"m4o.io/gslog/errorreporting"
	"m4o.io/gslog/gcp"
	"m4o.io/gslog/k8s"
	"m4o.io/gslog/otel"
)

// The example creates a gcp.Handler with a GCP logging.Logger.  The
// handler maps each slog.Record to a logging.Entry.  The handler then passes
// the entry to the Log() method of its logging.Logger.
func ExampleNewHandler() {
	ctx := context.Background()
	client, err := logging.NewClient(ctx, "my-project")
	if err != nil {
		// TODO: Handle error.
	}

	lg := client.Logger("my-log")

	lg.Flush()

	h := gcp.NewHandler(lg)
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

// PrintJsonPayload is a gcp.Logger stub that prints the logging.Entry
// Payload field as a JSON string.
func PrintJsonPayload(e logging.Entry) {
	b, _ := protojson.Marshal(e.Payload.(*spb.Struct))
	// Do another JSON round-trip, because protojson randomizes its output.
	var j map[string]interface{}
	_ = json.Unmarshal(b, &j)
	b, _ = json.Marshal(j)
	fmt.Println(string(b))
}

// The gcp.Handler maps the slog.Record and the nested group attributes
// of the handler into a JSON object.  The message is at the root of the
// object with the key "message".
func ExampleHandler_Handle_payloadMapping() {
	h := gcp.NewHandler(gcp.LoggerFunc(PrintJsonPayload))
	l := slog.New(h)
	l = l.WithGroup("pub")
	l = l.With(slog.Any("user", u))

	l.Info("How now brown cow?")

	// Output: {"message":"How now brown cow?","pub":{"user":{"age":32,"email":"jan@example.com","engineer":true,"first_name":"Jan","height":5.91,"id":"user-12234","last_name":"Doe","manager":null,"password":"\u003csecret\u003e"}}}
}

// PrintLabels is a gcp.Logger stub that prints the logging.Entry's
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

// The gcp.Handler adds the labels in the context to the Labels field of
// the logging.Entry.
func ExampleHandler_Handle_withLabels() {
	h := gcp.NewHandler(gcp.LoggerFunc(PrintLabels))
	l := slog.New(h)

	ctx := context.Background()
	ctx = core.WithLabels(ctx, core.Label("a", "one"), core.Label("b", "two"))

	l.Log(ctx, slog.LevelInfo, "How now brown cow?")

	// Output: a=one, b=two
}

// With the k8s.WithPodinfoLabels() option, gcp.Handler adds labels from
// the configured Kubernetes Downward API podinfo labels file to the Labels
// field of the logging.Entry.
//
// The handler adds the prefix "k8s-pod/" to each label.  This follows the
// Google Cloud Logging conventions for Kubernetes Pod labels.
func ExampleNewHandler_withK8sPodinfo() {
	h := gcp.NewHandler(gcp.LoggerFunc(PrintLabels), k8s.WithPodinfoLabels("../k8s/testdata/etc/podinfo"))
	l := slog.New(h)

	ctx := context.Background()
	ctx = core.WithLabels(ctx, core.Label("a", "one"), core.Label("b", "two"))

	l.Log(ctx, core.LevelCritical, "Danger, Will Robinson!")

	// Output: a=one, b=two, k8s-pod/app=hello-world, k8s-pod/environment=stg, k8s-pod/tier=backend, k8s-pod/track=stable
}

// With the otel.WithOtelBaggage() option, gcp.Handler adds the
// baggage.Baggage in the context as attributes.
//
// The handler adds the prefix "otel-baggage/" to each baggage key.  The
// prefix makes collisions with other log attributes less likely.  A baggage
// attribute has precedence over an attribute that already has the same key.
func ExampleNewHandler_withOpenTelemetryBaggage() {
	h := gcp.NewHandler(gcp.LoggerFunc(PrintJsonPayload), otel.WithOtelBaggage())
	l := slog.New(h)

	ctx := context.Background()
	ctx = baggage.ContextWithBaggage(ctx, otel.MustParse("a=one,b=two;p1;p2=val2"))

	l.Log(ctx, slog.LevelInfo, "How now brown cow?")

	// Output: {"message":"How now brown cow?","otel-baggage/a":"one","otel-baggage/b":{"properties":{"p1":null,"p2":"val2"},"value":"two"}}
}

// PrintTracing is a gcp.Logger stub that prints the logging.Entry's
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

// With the otel.WithOtelTracing() option, gcp.Handler adds the
// OpenTelemetry trace.SpanContext information in the context to the tracing
// fields of the logging.Entry.
func ExampleNewHandler_withOpenTelemetryTrace() {
	h := gcp.NewHandler(gcp.LoggerFunc(PrintTracing), otel.WithOtelTracing("my-project"))
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

// PrintSourceLocation is a gcp.Logger stub that prints the logging.Entry's
// SourceLocation field.
func PrintSourceLocation(e logging.Entry) {
	sl := e.SourceLocation
	sl.File = sl.File[len(sl.File)-len("gcp/example_test.go"):]

	b, _ := protojson.Marshal(sl)
	// Do another JSON round-trip, because protojson randomizes its output.
	var j map[string]interface{}
	_ = json.Unmarshal(b, &j)
	b, _ = json.Marshal(j)
	fmt.Println(string(b))
}

// With the core.WithSourceAdded() option, gcp.Handler adds the
// SourceLocation field to the logging.Entry.  This field is expensive to
// compute.
func ExampleNewHandler_withSourceAdded() {
	h := gcp.NewHandler(gcp.LoggerFunc(PrintSourceLocation), core.WithSourceAdded())
	l := slog.New(h)

	l.Log(context.Background(), slog.LevelInfo, "How now brown cow?")

	// Output: {"file":"gcp/example_test.go","function":"m4o.io/gslog/gcp_test.ExampleNewHandler_withSourceAdded","line":"262"}
}

// RemovePassword is a core.AttrMapper that elides password attributes.
func RemovePassword(_ []string, a slog.Attr) slog.Attr {
	if a.Key == "password" {
		return slog.Attr{}
	}
	return a
}

// With the core.WithReplaceAttr() option, gcp.Handler applies the
// supplied core.AttrMapper to each non-group attribute before the handler
// logs the attribute.
func ExampleNewHandler_withReplaceAttr() {
	h := gcp.NewHandler(gcp.LoggerFunc(PrintJsonPayload), core.WithReplaceAttr(RemovePassword))
	l := slog.New(h)
	l = l.WithGroup("pub")
	l = l.With(slog.String("username", "user-12234"), slog.String("password", string(pw)))

	l.Info("How now brown cow?")

	// Output: {"message":"How now brown cow?","pub":{"username":"user-12234"}}
}

// With the core.WithLogLeveler() option, gcp.Handler uses the
// slog.Leveler to decide whether a log level is enabled.
func ExampleNewHandler_withLogLeveler() {
	h := gcp.NewHandler(gcp.LoggerFunc(PrintJsonPayload), core.WithLogLeveler(slog.LevelError))
	l := slog.New(h)

	l.Info("How now brown cow?")
	l.Error("The rain in Spain lies mainly on the plane.")

	// Output: {"message":"The rain in Spain lies mainly on the plane."}
}

// With the core.WithLogLevelFromEnvVar() option, gcp.Handler reads its
// log level from the environment variable that the key names.
func ExampleNewHandler_withLogLevelFromEnvVar() {
	const envVar = "FOO_LOG_LEVEL"
	_ = os.Setenv(envVar, "ERROR")
	defer func() {
		_ = os.Unsetenv(envVar)
	}()

	h := gcp.NewHandler(gcp.LoggerFunc(PrintJsonPayload), core.WithLogLevelFromEnvVar(envVar))
	l := slog.New(h)

	l.Info("How now brown cow?")
	l.Error("The rain in Spain lies mainly on the plane.")

	// Output: {"message":"The rain in Spain lies mainly on the plane."}
}

// The core.WithDefaultLogLeveler() option sets a default log level.
func ExampleNewHandler_withDefaultLogLeveler() {
	const envVar = "FOO_LOG_LEVEL"

	h := gcp.NewHandler(
		gcp.LoggerFunc(PrintJsonPayload),
		core.WithLogLevelFromEnvVar(envVar),
		core.WithDefaultLogLeveler(slog.LevelError),
	)
	l := slog.New(h)

	l.Info("How now brown cow?")
	l.Error("The rain in Spain lies mainly on the plane.")

	// Output: {"message":"The rain in Spain lies mainly on the plane."}
}

// PrintErrorReport is a gcp.Logger stub that prints the fields "@type" and
// "serviceContext" of the logging.Entry Payload.  The stack trace is
// different on each run.
func PrintErrorReport(e logging.Entry) {
	payload := e.Payload.(*spb.Struct).AsMap()

	report := map[string]any{
		"@type":          payload["@type"],
		"serviceContext": payload["serviceContext"],
	}

	b, _ := json.Marshal(report)

	fmt.Println(string(b))
}

// With the errorreporting.WithService() option, gcp.Handler writes the
// fields that Google Cloud Error Reporting reads.  The handler writes the
// fields in the payload of each record at level Error or higher.  The field
// "stack_trace" holds the stack of the log call.
func ExampleNewHandler_withErrorReporting() {
	h := gcp.NewHandler(gcp.LoggerFunc(PrintErrorReport), errorreporting.WithService("checkout", "v1.2.0"))
	l := slog.New(h)

	l.Error("Payment failed")

	// Output: {"@type":"type.googleapis.com/google.devtools.clouderrorreporting.v1beta1.ReportedErrorEvent","serviceContext":{"service":"checkout","version":"v1.2.0"}}
}
