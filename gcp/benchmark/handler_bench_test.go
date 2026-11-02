// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package benchmark_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"cloud.google.com/go/logging"
	"go.opentelemetry.io/otel/trace"

	"m4o.io/gslog"
	"m4o.io/gslog/gcp"
	"m4o.io/gslog/otel"
)

type noopLogger struct{}

func (noopLogger) Log(logging.Entry) {}

func (noopLogger) LogSync(context.Context, logging.Entry) error {
	return nil
}

func (noopLogger) Flush() error {
	return nil
}

func BenchmarkHandleThreeAttrs(b *testing.B) {
	h := gcp.NewHandler(noopLogger{}, gslog.WithLeveler(slog.LevelInfo))
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		r := slog.NewRecord(time.Time{}, slog.LevelInfo, "message", 0)
		r.AddAttrs(slog.String("s", "hello"), slog.Int("n", 42), slog.Bool("b", true))

		if err := h.Handle(ctx, r); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHandleLabels(b *testing.B) {
	h := gcp.NewHandler(noopLogger{}, gslog.WithLeveler(slog.LevelInfo))

	ctx := context.Background()
	ctx = gslog.WithLabels(ctx,
		gslog.Label("tenant", "acme"), gslog.Label("region", "eu"),
		gslog.Label("env", "prod"), gslog.Label("version", "1.2.3"))

	handle(ctx, b, h, slog.String("s", "hello"), slog.Int("n", 42), slog.Bool("b", true))
}

func BenchmarkHandleTimeAttr(b *testing.B) {
	h := gcp.NewHandler(noopLogger{}, gslog.WithLeveler(slog.LevelInfo))
	ctx := context.Background()
	ts := time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)

	handle(ctx, b, h, slog.Time("t", ts))
}

func BenchmarkHandleTracing(b *testing.B) {
	h := gcp.NewHandler(noopLogger{}, gslog.WithLeveler(slog.LevelInfo), otel.WithTracing("my-project"))
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
	b.ResetTimer()

	for range b.N {
		r := slog.NewRecord(time.Time{}, slog.LevelInfo, "message", 0)
		r.AddAttrs(attrs...)

		if err := h.Handle(ctx, r); err != nil {
			b.Fatal(err)
		}
	}
}
