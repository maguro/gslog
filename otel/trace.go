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

package otel

import (
	"context"
	"encoding/hex"
	"strings"

	"go.opentelemetry.io/otel/trace"

	"m4o.io/gslog/core"
	"m4o.io/gslog/internal/entry"
	"m4o.io/gslog/internal/options"
)

// WithOtelTracing returns an option that causes the handler to include
// OpenTelemetry tracing.  The handler gets the tracing information from the
// trace.SpanContext in the context, if the context has one.  projectID is the
// ID of the project that holds the traces.
//
// The handler gets the span context only from the context of the log call.
// The record of a log call with no context, such as Logger.Info, has no
// tracing.  Use a call with a context, such as Logger.InfoContext.
func WithOtelTracing(projectID string) core.Option {
	tracePrefix := "projects/" + projectID + "/traces/"

	return func(options *options.Options) {
		options.EntryAugmentors = append(options.EntryAugmentors,
			func(ctx context.Context, e *entry.Entry) {
				spanContext := trace.SpanContextFromContext(ctx)

				if spanContext.HasTraceID() {
					e.Trace = traceName(tracePrefix, spanContext.TraceID())
				}

				if spanContext.HasSpanID() {
					e.SpanID = spanContext.SpanID().String()
				}

				if spanContext.IsSampled() {
					e.TraceSampled = true
				}
			})
	}
}

// traceName returns the prefix followed by the hex form of the trace ID.
func traceName(prefix string, traceID trace.TraceID) string {
	var hexID [2 * len(traceID)]byte

	hex.Encode(hexID[:], traceID[:])

	var b strings.Builder

	b.Grow(len(prefix) + len(hexID))
	b.WriteString(prefix)
	b.Write(hexID[:])

	return b.String()
}
