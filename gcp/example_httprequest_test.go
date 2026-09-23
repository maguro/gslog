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

package gcp_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"cloud.google.com/go/logging"

	"m4o.io/gslog/gcp"
)

// statusRecorder is an http.ResponseWriter that records the status and the
// size of the response.
type statusRecorder struct {
	http.ResponseWriter

	status int
	size   int64
}

func (s *statusRecorder) WriteHeader(status int) {
	s.status = status

	s.ResponseWriter.WriteHeader(status)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	n, err := s.ResponseWriter.Write(b)
	s.size += int64(n)

	return n, err
}

// accessLog returns an HTTP handler that logs one record for each request.
// The HTTP handler gives the request, the status, the size, and the latency
// to the gcp.Handler in the context of the log call.
func accessLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK, size: 0}

		next.ServeHTTP(recorder, r)

		request := &logging.HTTPRequest{
			Request:      r,
			Status:       recorder.status,
			ResponseSize: recorder.size,
			Latency:      time.Since(start),
		}
		ctx := gcp.WithHTTPRequest(r.Context(), request)

		logger.InfoContext(ctx, "Request completed")
	})
}

// PrintHTTPRequest is a gcp.Logger stub that prints the HTTPRequest field of
// the logging.Entry.  The latency is different on each run.  PrintHTTPRequest
// does not print the latency.
func PrintHTTPRequest(e logging.Entry) {
	r := e.HTTPRequest

	fmt.Printf("%s %s status=%d size=%d\n", r.Request.Method, r.Request.URL.Path, r.Status, r.ResponseSize)
}

// With gcp.WithHTTPRequest, the gcp.Handler sets the HTTPRequest field of the
// logging.Entry.  The Logs Explorer shows the method, the status, the size,
// and the latency of the request in the summary of that entry.
func ExampleWithHTTPRequest() {
	h := gcp.NewHandler(gcp.LoggerFunc(PrintHTTPRequest))
	logger := slog.New(h)

	app := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	})
	server := accessLog(logger, app)

	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/orders", http.NoBody)

	server.ServeHTTP(httptest.NewRecorder(), request)

	// Output: POST /orders status=201 size=7
}
