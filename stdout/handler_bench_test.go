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

package stdout_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"go.opentelemetry.io/otel/baggage"

	"m4o.io/gslog/core"
	"m4o.io/gslog/otel"
	"m4o.io/gslog/stdout"
)

func BenchmarkHandleThreeAttrs(b *testing.B) {
	h := stdout.NewHandler(io.Discard, core.WithLogLeveler(slog.LevelInfo))
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
	var h slog.Handler = stdout.NewHandler(io.Discard, core.WithLogLeveler(slog.LevelInfo))
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
	h := stdout.NewHandler(io.Discard, core.WithLogLeveler(slog.LevelInfo), otel.WithOtelBaggage())

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
