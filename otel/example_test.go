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

package otel_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"

	"go.opentelemetry.io/otel/propagation"

	"m4o.io/gslog/otel"
	"m4o.io/gslog/stdout"
)

// withTraceContext returns an HTTP handler that reads the W3C traceparent
// header of each request.  The HTTP handler puts the span context of the
// header in the context of the request.  A program that uses otelhttp does
// not need this HTTP handler.
func withTraceContext(next http.Handler) http.Handler {
	propagator := propagation.TraceContext{}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		carrier := propagation.HeaderCarrier(r.Header)
		ctx := propagator.Extract(r.Context(), carrier)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// A program with no OpenTelemetry SDK can link its logs to the trace of each
// request.  The HTTP handler from withTraceContext puts the span context of
// the traceparent header in the context of the request.  The log call gives
// that context to the slog handler.
func ExampleWithOtelTracing_httpServer() {
	var buf bytes.Buffer

	h := stdout.NewHandler(&buf, otel.WithOtelTracing("my-project"))
	logger := slog.New(h)

	app := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		logger.InfoContext(r.Context(), "Order placed")
	})
	server := withTraceContext(app)

	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/orders", http.NoBody)
	request.Header.Set("traceparent", "00-52fc1643a9381fc674742bb0067101e7-d3e9e8c51cb190df-01")

	server.ServeHTTP(httptest.NewRecorder(), request)

	// The timestamp is different on each run.  The example prints the
	// tracing fields.
	var line map[string]any

	_ = json.Unmarshal(buf.Bytes(), &line)

	tracing := map[string]any{
		"logging.googleapis.com/trace":         line["logging.googleapis.com/trace"],
		"logging.googleapis.com/spanId":        line["logging.googleapis.com/spanId"],
		"logging.googleapis.com/trace_sampled": line["logging.googleapis.com/trace_sampled"],
	}

	b, _ := json.Marshal(tracing)

	fmt.Println(string(b))

	// Output: {"logging.googleapis.com/spanId":"d3e9e8c51cb190df","logging.googleapis.com/trace":"projects/my-project/traces/52fc1643a9381fc674742bb0067101e7","logging.googleapis.com/trace_sampled":true}
}
