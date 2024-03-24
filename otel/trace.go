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

/*
Package otel contains options for including OpenTelemetry tracing in logging
records.

Placing the options in a separate package minimizes the dependencies pulled in
by those who do not need OpenTelemetry tracing.
*/
package otel

import (
	"context"

	"cloud.google.com/go/logging"
	"go.opentelemetry.io/otel/trace"

	"m4o.io/gslog/internal/options"
)

// WithOtelTracing returns a gslog.Option that directs that the slog.Handler
// to include OpenTelemetry tracing.
func WithOtelTracing() options.OptionProcessor {
	return func(options *options.Options) {
		options.EntryAugmentors = append(options.EntryAugmentors, addTrace)
	}
}

func addTrace(ctx context.Context, e *logging.Entry) {
	span := trace.SpanContextFromContext(ctx)

	if span.HasTraceID() {
		e.Trace = span.TraceID().String()
	}
	if span.HasSpanID() {
		e.SpanID = span.SpanID().String()
	}
	if span.IsSampled() {
		e.TraceSampled = true
	}
}
