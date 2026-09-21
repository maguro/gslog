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

package errorreporting_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"cloud.google.com/go/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	spb "google.golang.org/protobuf/types/known/structpb"

	"m4o.io/gslog/errorreporting"
	"m4o.io/gslog/gcp"
	"m4o.io/gslog/stdout"
)

const typeValue = "type.googleapis.com/google.devtools.clouderrorreporting.v1beta1.ReportedErrorEvent"

// wrapHandler is a handler that calls an inner handler through a sync.Once.
// The stack trace of a call to the inner handler has frames of the sync
// package.
type wrapHandler struct{ inner slog.Handler }

func (w wrapHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return w.inner.Enabled(ctx, l)
}

func (w wrapHandler) Handle(ctx context.Context, record slog.Record) error {
	var (
		once sync.Once
		err  error
	)

	once.Do(func() {
		err = w.inner.Handle(ctx, record)
	})

	return err
}

func (w wrapHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	inner := w.inner.WithAttrs(attrs)

	return wrapHandler{inner: inner}
}

func (w wrapHandler) WithGroup(name string) slog.Handler {
	inner := w.inner.WithGroup(name)

	return wrapHandler{inner: inner}
}

func TestWithService_ErrorRecord(t *testing.T) {
	var buf bytes.Buffer

	l := newLogger(&buf, "v1")
	l.Error("boom", "k", "v")

	m := decode(t, &buf)

	assert.Equal(t, typeValue, m["@type"])
	assert.Equal(t, map[string]any{"service": "svc", "version": "v1"}, m["serviceContext"])
	assert.Equal(t, "v", m["k"])

	got, _ := m["stack_trace"].(string)
	lines := strings.SplitN(got, "\n", 3)
	require.Len(t, lines, 3)
	assert.True(t, strings.HasPrefix(lines[0], "goroutine "), got)
	assert.Contains(t, lines[1], "errorreporting_test.TestWithService_ErrorRecord")
	assert.NotContains(t, got, "log/slog.")
	assert.NotContains(t, got, "m4o.io/gslog/stdout")
}

func TestWithService_WrappedHandler(t *testing.T) {
	var buf bytes.Buffer

	h := stdout.NewHandler(&buf, errorreporting.WithService("svc", "v1"))

	l := slog.New(wrapHandler{inner: wrapHandler{inner: h}})
	l.Error("boom")

	m := decode(t, &buf)

	got, _ := m["stack_trace"].(string)
	lines := strings.SplitN(got, "\n", 3)
	require.Len(t, lines, 3)
	assert.Contains(t, lines[1], "errorreporting_test.TestWithService_WrappedHandler")
	assert.NotContains(t, got, "wrapHandler")
	assert.NotContains(t, got, "sync.")
	assert.NotContains(t, got, "log/slog.")
}

func TestWithService_InfoRecord(t *testing.T) {
	var buf bytes.Buffer

	l := newLogger(&buf, "v1")
	l.Info("fine")

	m := decode(t, &buf)

	assert.NotContains(t, m, "@type")
	assert.NotContains(t, m, "serviceContext")
	assert.NotContains(t, m, "stack_trace")
}

func TestWithService_NoVersion(t *testing.T) {
	var buf bytes.Buffer

	l := newLogger(&buf, "")
	l.Error("boom")

	m := decode(t, &buf)

	assert.Equal(t, map[string]any{"service": "svc"}, m["serviceContext"])
}

func TestWithService_ReservedKeys(t *testing.T) {
	var buf bytes.Buffer

	l := newLogger(&buf, "v1").With("@type", "with", "stack_trace", "with")
	l.Error("boom", "@type", "user", "stack_trace", "user", slog.Group("serviceContext", "service", "user"))

	line := buf.String()
	assert.Equal(t, 1, strings.Count(line, `"@type"`))
	assert.Equal(t, 1, strings.Count(line, `"stack_trace"`))
	assert.Equal(t, 1, strings.Count(line, `"serviceContext"`))

	m := decode(t, &buf)

	assert.Equal(t, typeValue, m["@type"])
	assert.Equal(t, map[string]any{"service": "svc", "version": "v1"}, m["serviceContext"])
}

func TestWithService_WithGroup(t *testing.T) {
	var buf bytes.Buffer

	l := newLogger(&buf, "v1")
	l.WithGroup("g").Error("boom", "k", "v")

	m := decode(t, &buf)

	assert.Equal(t, typeValue, m["@type"])
	assert.Equal(t, map[string]any{"k": "v"}, m["g"])
}

func TestWithService_GcpHandler(t *testing.T) {
	var entries []logging.Entry

	logger := gcp.LoggerFunc(func(e logging.Entry) {
		entries = append(entries, e)
	})
	h := gcp.NewHandler(logger, errorreporting.WithService("svc", "v1"))

	l := slog.New(h)
	l.WithGroup("g").Error("boom", "k", "v")
	l.Error("collide", "@type", "user")

	require.Len(t, entries, 2)

	payload, ok := entries[0].Payload.(*spb.Struct)
	require.True(t, ok)

	m := payload.AsMap()

	assert.Equal(t, typeValue, m["@type"])
	assert.Equal(t, map[string]any{"service": "svc", "version": "v1"}, m["serviceContext"])
	assert.Equal(t, map[string]any{"k": "v"}, m["g"])

	got, _ := m["stack_trace"].(string)
	assert.True(t, strings.HasPrefix(got, "goroutine "), got)
	assert.Contains(t, got, "errorreporting_test.TestWithService_GcpHandler")
	assert.NotContains(t, got, "m4o.io/gslog/gcp")

	collide, ok := entries[1].Payload.(*spb.Struct)
	require.True(t, ok)
	assert.Equal(t, typeValue, collide.AsMap()["@type"])
}

func TestWithService_EmptyService(t *testing.T) {
	assert.Panics(t, func() {
		errorreporting.WithService("", "v1")
	})
}

// newLogger returns a logger that writes to w.  The handler of the logger
// has the WithService option for the service "svc".
func newLogger(w *bytes.Buffer, version string) *slog.Logger {
	h := stdout.NewHandler(w, errorreporting.WithService("svc", version))

	return slog.New(h)
}

// decode returns the JSON object in buf.
func decode(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()

	var m map[string]any

	err := json.Unmarshal(buf.Bytes(), &m)
	require.NoError(t, err)

	return m
}
