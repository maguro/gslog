// Copyright the original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otel_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"

	"m4o.io/gslog/otel"
)

// remoteSpanContext returns a context that has a sampled remote span.
func remoteSpanContext(t *testing.T) context.Context {
	t.Helper()

	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	require.NoError(t, err)

	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	require.NoError(t, err)

	sCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})

	return trace.ContextWithRemoteSpanContext(context.Background(), sCtx)
}

func TestWithTracing(t *testing.T) {
	ctx := remoteSpanContext(t)

	e := entryOf(ctx, otel.WithTracing("my-project"))

	assert.Equal(t, "projects/my-project/traces/4bf92f3577b34da6a3ce929d0e0e4736", e.Trace())
	assert.Equal(t, "00f067aa0ba902b7", e.SpanIDString())
	assert.True(t, e.TraceSampled)
}

// TestWithTracing_repeated verifies that the last WithTracing option wins.
func TestWithTracing_repeated(t *testing.T) {
	ctx := remoteSpanContext(t)

	e := entryOf(ctx, otel.WithTracing("first"), otel.WithTracing("second"))

	assert.Equal(t, "projects/second/traces/4bf92f3577b34da6a3ce929d0e0e4736", e.Trace())
}

func TestWithTracing_EmptyProjectID(t *testing.T) {
	assert.Panics(t, func() {
		otel.WithTracing("")
	})
}
