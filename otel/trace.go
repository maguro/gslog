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

	"go.opentelemetry.io/otel/trace"

	"m4o.io/gslog"
	"m4o.io/gslog/internal/entry"
	"m4o.io/gslog/internal/options"
)

// WithTracing returns an option that causes the handler to include
// OpenTelemetry tracing.  The handler gets the tracing information from the
// trace.SpanContext in the context, if the context has one.  projectID is the
// ID of the project that holds the traces.  WithTracing panics if projectID
// is empty.  If the options have more than one WithTracing option, the
// handler uses the last WithTracing option.
//
// The handler gets the span context only from the context of the log call.
// The record of a log call with no context, such as Logger.Info, has no
// tracing.  Use a call with a context, such as Logger.InfoContext.
func WithTracing(projectID string) gslog.Option {
	if projectID == "" {
		panic("projectID is empty")
	}

	tracePrefix := "projects/" + projectID + "/traces/"

	return func(options *options.Options) {
		options.AddAugmentor("otel.WithTracing",
			func(ctx context.Context, e *entry.Entry) {
				spanContext := trace.SpanContextFromContext(ctx)

				if spanContext.HasTraceID() {
					e.TracePrefix = tracePrefix
					e.TraceID = spanContext.TraceID()
				}

				if spanContext.HasSpanID() {
					e.SpanID = spanContext.SpanID()
				}

				if spanContext.IsSampled() {
					e.TraceSampled = true
				}
			})
	}
}
