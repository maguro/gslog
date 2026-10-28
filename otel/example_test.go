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
	"os"
	"time"

	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"

	"m4o.io/gslog/otel"
	"m4o.io/gslog/stdout"
)

// withTraceContext returns an HTTP handler that reads the W3C traceparent
// header of each request.  The HTTP handler puts the span context of the
// header in the context of the request.  A program that uses otelhttp with
// a configured propagator does not need this HTTP handler.
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
func ExampleWithTracing_httpServer() {
	var buf bytes.Buffer

	h := stdout.NewHandler(&buf, otel.WithTracing("my-project"))
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

// withBaggage returns an HTTP handler that reads the W3C baggage header of
// each request.  The HTTP handler puts the baggage of the header in the
// context of the request.  A program that uses otelhttp with a configured
// propagator does not need this HTTP handler.
func withBaggage(next http.Handler) http.Handler {
	propagator := propagation.Baggage{}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		carrier := propagation.HeaderCarrier(r.Header)
		ctx := propagator.Extract(r.Context(), carrier)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// An upstream service sends the tenant and the user of a request in the
// baggage header.  The HTTP handler from withBaggage puts the baggage in the
// context of the request, and each log entry of the request gets the members
// as attributes.
func ExampleWithBaggage_httpServer() {
	var buf bytes.Buffer

	h := stdout.NewHandler(&buf, otel.WithBaggage())
	logger := slog.New(h)

	app := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		logger.InfoContext(r.Context(), "Order placed")
	})
	server := withBaggage(app)

	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/orders", http.NoBody)
	request.Header.Set("baggage", "tenant=acme,user=jan;role=admin")

	server.ServeHTTP(httptest.NewRecorder(), request)

	// The timestamp is different on each run.  The example prints the
	// baggage attributes.
	var line map[string]any

	_ = json.Unmarshal(buf.Bytes(), &line)

	attrs := map[string]any{
		"otel-baggage/tenant": line["otel-baggage/tenant"],
		"otel-baggage/user":   line["otel-baggage/user"],
	}

	b, _ := json.Marshal(attrs)

	fmt.Println(string(b))

	// Output: {"otel-baggage/tenant":"acme","otel-baggage/user":{"properties":{"role":"admin"},"value":"jan"}}
}

// A job with no incoming request sets the baggage in the context itself.
// MustParse builds the baggage from a literal that always parses.  A fixed
// time keeps the timestamp of the output the same on each run.
func ExampleMustParse() {
	h := stdout.NewHandler(os.Stdout, otel.WithBaggage())

	bag := otel.MustParse("tenant=acme,job=nightly-report")
	ctx := baggage.ContextWithBaggage(context.Background(), bag)

	at := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	r := slog.NewRecord(at, slog.LevelInfo, "Report started", 0)

	_ = h.Handle(ctx, r)

	// Output: {"severity":"INFO","message":"Report started","timestamp":"2024-01-02T03:04:05Z","otel-baggage/job":"nightly-report","otel-baggage/tenant":"acme"}
}
