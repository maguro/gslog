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
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloud.google.com/go/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	"m4o.io/gslog/core"
	"m4o.io/gslog/gcp"
)

func TestWithHTTPRequest(t *testing.T) {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/orders", http.NoBody)
	request := &logging.HTTPRequest{Request: r, Status: http.StatusCreated, ResponseSize: 7, Latency: 35 * time.Millisecond}

	var noRequest *logging.HTTPRequest

	noRequestField := &logging.HTTPRequest{Status: http.StatusOK}

	background := context.Background()

	for name, tt := range map[string]struct {
		ctx  context.Context
		log  func(ctx context.Context, l *slog.Logger)
		want *logging.HTTPRequest
	}{
		"request in the context": {
			ctx: gcp.WithHTTPRequest(background, request),
			log: func(ctx context.Context, l *slog.Logger) {
				l.InfoContext(ctx, "request completed")
			},
			want: request,
		},
		"request with groups and attributes": {
			ctx: gcp.WithHTTPRequest(background, request),
			log: func(ctx context.Context, l *slog.Logger) {
				l.With("a", 1).WithGroup("g").InfoContext(ctx, "request completed", "k", "v")
			},
			want: request,
		},
		"request at level Critical": {
			ctx: gcp.WithHTTPRequest(background, request),
			log: func(ctx context.Context, l *slog.Logger) {
				l.Log(ctx, core.LevelCritical, "request completed")
			},
			want: request,
		},
		"no request in the context": {
			ctx: background,
			log: func(ctx context.Context, l *slog.Logger) {
				l.InfoContext(ctx, "hello")
			},
			want: nil,
		},
		"log call with no context": {
			ctx: gcp.WithHTTPRequest(background, request),
			log: func(_ context.Context, l *slog.Logger) {
				l.Info("hello")
			},
			want: nil,
		},
		"nil request": {
			ctx: gcp.WithHTTPRequest(background, noRequest),
			log: func(ctx context.Context, l *slog.Logger) {
				l.InfoContext(ctx, "hello")
			},
			want: nil,
		},
		"request with a nil Request field": {
			ctx: gcp.WithHTTPRequest(background, noRequestField),
			log: func(ctx context.Context, l *slog.Logger) {
				l.InfoContext(ctx, "hello")
			},
			want: noRequestField,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var entries []logging.Entry

			logger := gcp.LoggerFunc(func(e logging.Entry) {
				entries = append(entries, e)
			})

			tt.log(tt.ctx, slog.New(gcp.NewHandler(logger)))

			require.Len(t, entries, 1)
			assert.Same(t, tt.want, entries[0].HTTPRequest)

			payload, ok := entries[0].Payload.(*structpb.Struct)
			require.True(t, ok)
			assert.NotContains(t, payload.AsMap(), "httpRequest")
		})
	}
}
