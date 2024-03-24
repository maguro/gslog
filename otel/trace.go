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

package otel

import (
	"context"

	"cloud.google.com/go/logging"
	"go.opentelemetry.io/otel/trace"

	"m4o.io/gslog/internal/options"
)

// WithOtelTracing returns an option that directs that the slog.Handler to
// include OpenTelemetry tracing.  Tracing information is obtained from the
// trace.SpanContext stored in the context, if provided.
func WithOtelTracing() options.OptionProcessor {
	return func(options *options.Options) {
		options.EntryAugmentors = append(options.EntryAugmentors, addTrace)
	}
}

func addTrace(ctx context.Context, e *logging.Entry, _ []string) {
	sc := trace.SpanContextFromContext(ctx)

	if sc.HasTraceID() {
		e.Trace = sc.TraceID().String()
	}
	if sc.HasSpanID() {
		e.SpanID = sc.SpanID().String()
	}
	if sc.IsSampled() {
		e.TraceSampled = true
	}
}
