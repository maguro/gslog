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

package benchmark_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"

	"m4o.io/gslog"
	"m4o.io/gslog/otel"
	"m4o.io/gslog/stdout"
)

func BenchmarkHandleThreeAttrs(b *testing.B) {
	h := stdout.NewHandler(io.Discard, gslog.WithLeveler(slog.LevelInfo))
	ctx := context.Background()

	b.ReportAllocs()

	for b.Loop() {
		r := slog.NewRecord(time.Time{}, slog.LevelInfo, "message", 0)
		r.AddAttrs(slog.String("s", "hello"), slog.Int("n", 42), slog.Bool("b", true))

		if err := h.Handle(ctx, r); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHandleGroups(b *testing.B) {
	var h slog.Handler = stdout.NewHandler(io.Discard, gslog.WithLeveler(slog.LevelInfo))
	h = h.WithGroup("req")
	h = h.WithAttrs([]slog.Attr{slog.String("id", "abc")})
	h = h.WithGroup("user")

	ctx := context.Background()

	b.ReportAllocs()

	for b.Loop() {
		r := slog.NewRecord(time.Time{}, slog.LevelInfo, "message", 0)
		r.AddAttrs(slog.String("s", "hello"), slog.Group("g", slog.Int("n", 42)))

		if err := h.Handle(ctx, r); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHandleBaggage(b *testing.B) {
	h := stdout.NewHandler(io.Discard, gslog.WithLeveler(slog.LevelInfo), otel.WithBaggage())

	bag := otel.MustParse("a=one,b=two;p1;p2=val2")
	ctx := baggage.ContextWithBaggage(context.Background(), bag)

	b.ReportAllocs()

	for b.Loop() {
		r := slog.NewRecord(time.Time{}, slog.LevelInfo, "message", 0)

		if err := h.Handle(ctx, r); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHandleLabels(b *testing.B) {
	h := stdout.NewHandler(io.Discard, gslog.WithLeveler(slog.LevelInfo))

	ctx := context.Background()
	ctx = gslog.WithLabels(ctx,
		gslog.Label("tenant", "acme"), gslog.Label("region", "eu"),
		gslog.Label("env", "prod"), gslog.Label("version", "1.2.3"))

	handle(ctx, b, h, slog.String("s", "hello"), slog.Int("n", 42), slog.Bool("b", true))
}

func BenchmarkHandleTimeAttr(b *testing.B) {
	h := stdout.NewHandler(io.Discard, gslog.WithLeveler(slog.LevelInfo))
	ctx := context.Background()
	ts := time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)

	handle(ctx, b, h, slog.Time("t", ts))
}

func BenchmarkHandleTracing(b *testing.B) {
	h := stdout.NewHandler(io.Discard, gslog.WithLeveler(slog.LevelInfo), otel.WithTracing("my-project"))
	ctx := tracedContext(b)

	handle(ctx, b, h, slog.String("s", "hello"))
}

// tracedContext returns a context with a sampled span.
func tracedContext(b *testing.B) context.Context {
	b.Helper()

	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		b.Fatal(err)
	}

	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	if err != nil {
		b.Fatal(err)
	}

	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})

	return trace.ContextWithSpanContext(context.Background(), spanContext)
}

// handle runs the benchmark loop.  Each iteration handles one record with
// attrs.
func handle(ctx context.Context, b *testing.B, h slog.Handler, attrs ...slog.Attr) {
	b.Helper()
	b.ReportAllocs()

	for b.Loop() {
		r := slog.NewRecord(time.Time{}, slog.LevelInfo, "message", 0)
		r.AddAttrs(attrs...)

		if err := h.Handle(ctx, r); err != nil {
			b.Fatal(err)
		}
	}
}
