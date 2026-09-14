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

package stdout_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"time"

	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"

	"m4o.io/gslog/core"
	"m4o.io/gslog/k8s"
	"m4o.io/gslog/otel"
	"m4o.io/gslog/stdout"
)

// newRecord returns an Info record with a fixed time and the message "How
// now brown cow?".  A fixed time keeps the timestamp of the example output
// the same on each run.
func newRecord(attrs ...slog.Attr) slog.Record {
	at := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

	r := slog.NewRecord(at, slog.LevelInfo, "How now brown cow?", 0)
	r.AddAttrs(attrs...)

	return r
}

// NewHandler writes each entry to the writer as one line of JSON in the
// format that the Google Cloud logging agent reads.
func ExampleNewHandler() {
	h := stdout.NewHandler(os.Stdout)

	r := newRecord(slog.String("animal", "cow"))

	_ = h.Handle(context.Background(), r)

	// Output: {"severity":"INFO","message":"How now brown cow?","timestamp":"2024-01-02T03:04:05Z","animal":"cow"}
}

type User struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// The stdout.Handler writes the nested group attributes of the handler as
// nested JSON objects.  The message is at the root of the object with the
// key "message".
func ExampleHandler_Handle_payloadMapping() {
	u := User{ID: "user-12234", FirstName: "Jan", LastName: "Doe"}

	var h slog.Handler = stdout.NewHandler(os.Stdout)
	h = h.WithGroup("pub")
	h = h.WithAttrs([]slog.Attr{slog.Any("user", u)})

	_ = h.Handle(context.Background(), newRecord())

	// Output: {"severity":"INFO","message":"How now brown cow?","timestamp":"2024-01-02T03:04:05Z","pub":{"user":{"id":"user-12234","first_name":"Jan","last_name":"Doe"}}}
}

// The stdout.Handler writes the labels in the context in the field
// "logging.googleapis.com/labels".
func ExampleHandler_Handle_withLabels() {
	h := stdout.NewHandler(os.Stdout)

	ctx := context.Background()
	ctx = core.WithLabels(ctx, core.Label("a", "one"), core.Label("b", "two"))

	_ = h.Handle(ctx, newRecord())

	// Output: {"severity":"INFO","message":"How now brown cow?","timestamp":"2024-01-02T03:04:05Z","logging.googleapis.com/labels":{"a":"one","b":"two"}}
}

// With the k8s.WithPodinfoLabels() option, stdout.Handler adds labels from
// the configured Kubernetes Downward API podinfo labels file to the field
// "logging.googleapis.com/labels".
//
// The handler adds the prefix "k8s-pod/" to each label.  This follows the
// Google Cloud Logging conventions for Kubernetes Pod labels.
func ExampleNewHandler_withK8sPodinfo() {
	h := stdout.NewHandler(os.Stdout, k8s.WithPodinfoLabels("../k8s/testdata/etc/podinfo"))

	ctx := context.Background()
	ctx = core.WithLabels(ctx, core.Label("a", "one"), core.Label("b", "two"))

	_ = h.Handle(ctx, newRecord())

	// Output: {"severity":"INFO","message":"How now brown cow?","timestamp":"2024-01-02T03:04:05Z","logging.googleapis.com/labels":{"a":"one","b":"two","k8s-pod/app":"hello-world","k8s-pod/environment":"stg","k8s-pod/tier":"backend","k8s-pod/track":"stable"}}
}

// With the otel.WithOtelBaggage() option, stdout.Handler adds the
// baggage.Baggage in the context as attributes.
//
// The handler adds the prefix "otel-baggage/" to each baggage key.  The
// prefix makes collisions with other log attributes less likely.  The
// handler writes the baggage attributes after the attributes of the record.
func ExampleNewHandler_withOpentelemetryBaggage() {
	h := stdout.NewHandler(os.Stdout, otel.WithOtelBaggage())

	ctx := context.Background()
	ctx = baggage.ContextWithBaggage(ctx, otel.MustParse("a=one,b=two;p1;p2=val2"))

	_ = h.Handle(ctx, newRecord())

	// Output: {"severity":"INFO","message":"How now brown cow?","timestamp":"2024-01-02T03:04:05Z","otel-baggage/a":"one","otel-baggage/b":{"value":"two","properties":{"p1":null,"p2":"val2"}}}
}

// With the otel.WithOtelTracing() option, stdout.Handler adds the
// OpenTelemetry trace.SpanContext information in the context to the tracing
// fields of the line.
func ExampleNewHandler_withOpentelemetryTrace() {
	h := stdout.NewHandler(os.Stdout, otel.WithOtelTracing("my-project"))

	traceID, _ := trace.TraceIDFromHex("52fc1643a9381fc674742bb0067101e7")
	spanID, _ := trace.SpanIDFromHex("d3e9e8c51cb190df")
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})

	ctx := trace.ContextWithSpanContext(context.Background(), spanContext)

	_ = h.Handle(ctx, newRecord())

	// Output: {"severity":"INFO","message":"How now brown cow?","timestamp":"2024-01-02T03:04:05Z","logging.googleapis.com/trace":"projects/my-project/traces/52fc1643a9381fc674742bb0067101e7","logging.googleapis.com/spanId":"d3e9e8c51cb190df","logging.googleapis.com/trace_sampled":true}
}

// With the core.WithSourceAdded() option, stdout.Handler adds the field
// "logging.googleapis.com/sourceLocation" to the line.  This field is
// expensive to compute.
func ExampleNewHandler_withSourceAdded() {
	var buf bytes.Buffer

	h := stdout.NewHandler(&buf, core.WithSourceAdded())

	var pcs [1]uintptr

	runtime.Callers(1, pcs[:])

	r := slog.NewRecord(time.Time{}, slog.LevelInfo, "How now brown cow?", pcs[0])

	_ = h.Handle(context.Background(), r)

	// The file is an absolute path.  The example prints the end of the path.
	var line map[string]any

	_ = json.Unmarshal(buf.Bytes(), &line)

	loc, _ := line["logging.googleapis.com/sourceLocation"].(map[string]any)
	file, _ := loc["file"].(string)
	loc["file"] = file[len(file)-len("stdout/example_test.go"):]

	b, _ := json.Marshal(loc)

	fmt.Println(string(b))

	// Output: {"file":"stdout/example_test.go","function":"m4o.io/gslog/stdout_test.ExampleNewHandler_withSourceAdded","line":"159"}
}

// RemovePassword is a core.AttrMapper that elides password attributes.
func RemovePassword(_ []string, a slog.Attr) slog.Attr {
	if a.Key == "password" {
		return slog.Attr{}
	}

	return a
}

// With the core.WithReplaceAttr() option, stdout.Handler applies the
// supplied core.AttrMapper to each non-group attribute before the handler
// writes the attribute.
func ExampleNewHandler_withReplaceAttr() {
	var h slog.Handler = stdout.NewHandler(os.Stdout, core.WithReplaceAttr(RemovePassword))
	h = h.WithGroup("pub")
	h = h.WithAttrs([]slog.Attr{slog.String("username", "user-12234"), slog.String("password", "hunter2")})

	_ = h.Handle(context.Background(), newRecord())

	// Output: {"severity":"INFO","message":"How now brown cow?","timestamp":"2024-01-02T03:04:05Z","pub":{"username":"user-12234"}}
}

// With the core.WithLogLeveler() option, stdout.Handler uses the
// slog.Leveler to decide whether a log level is enabled.
func ExampleNewHandler_withLogLeveler() {
	h := stdout.NewHandler(os.Stdout, core.WithLogLeveler(slog.LevelError))

	ctx := context.Background()

	fmt.Println(h.Enabled(ctx, slog.LevelInfo), h.Enabled(ctx, slog.LevelError))

	// Output: false true
}

// With the core.WithLogLevelFromEnvVar() option, stdout.Handler reads its
// log level from the environment variable that the key names.
func ExampleNewHandler_withLogLevelFromEnvVar() {
	const envVar = "FOO_LOG_LEVEL"
	_ = os.Setenv(envVar, "ERROR")
	defer func() {
		_ = os.Unsetenv(envVar)
	}()

	h := stdout.NewHandler(os.Stdout, core.WithLogLevelFromEnvVar(envVar))

	ctx := context.Background()

	fmt.Println(h.Enabled(ctx, slog.LevelInfo), h.Enabled(ctx, slog.LevelError))

	// Output: false true
}

// The core.WithDefaultLogLeveler() option sets a default log level.  The
// environment variable is not set, so the default applies.
func ExampleNewHandler_withDefaultLogLeveler() {
	const envVar = "FOO_LOG_LEVEL"

	h := stdout.NewHandler(os.Stdout,
		core.WithLogLevelFromEnvVar(envVar),
		core.WithDefaultLogLeveler(slog.LevelError),
	)

	ctx := context.Background()

	fmt.Println(h.Enabled(ctx, slog.LevelInfo), h.Enabled(ctx, slog.LevelError))

	// Output: false true
}
