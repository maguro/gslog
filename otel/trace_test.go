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

package otel_test

import (
	"context"
	"log/slog"
	"testing"

	"cloud.google.com/go/logging"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace"

	"m4o.io/gslog"
	"m4o.io/gslog/otel"
)

type Got struct {
	LogEntry     logging.Entry
	SyncLogEntry logging.Entry
}

func (g *Got) Log(e logging.Entry) {
	g.LogEntry = e
}

func (g *Got) LogSync(_ context.Context, e logging.Entry) error {
	g.SyncLogEntry = e
	return nil
}

func (g *Got) Flush() error {
	return nil
}

func TestWithOtelTracing(t *testing.T) {
	traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")

	sCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})

	ctx := context.Background()
	ctx = trace.ContextWithRemoteSpanContext(ctx, sCtx)

	got := &Got{}
	h := gslog.NewGcpHandler(got, otel.WithOtelTracing("my-project"))
	l := slog.New(h)

	l.Log(ctx, slog.LevelInfo, "how now brown cow")

	e := got.LogEntry

	assert.Equal(t, "projects/my-project/traces/"+traceID.String(), e.Trace)
	assert.Equal(t, spanID.String(), e.SpanID)
	assert.True(t, e.TraceSampled)
}
