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

package stdout_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"

	"m4o.io/gslog/core"
	"m4o.io/gslog/internal/testhelp"
	"m4o.io/gslog/otel"
	"m4o.io/gslog/stdout"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("closed")
}

func decodeLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()

	line := buf.String()
	require.True(t, strings.HasSuffix(line, "\n"), "line must end with a newline")
	require.Equal(t, 1, strings.Count(line, "\n"), "one line per entry")

	var decoded map[string]any

	require.NoError(t, json.Unmarshal([]byte(line), &decoded))

	return decoded
}

func TestHandler_agentFields(t *testing.T) {
	var buf bytes.Buffer

	h := stdout.NewHandler(&buf, core.WithSourceAdded(), otel.WithOtelTracing("my-project"))

	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x52, 0xfc, 0x16, 0x43, 0xa9, 0x38, 0x1f, 0xc6, 0x74, 0x74, 0x2b, 0xb0, 0x06, 0x71, 0x01, 0xe7},
		SpanID:     trace.SpanID{0xd3, 0xe9, 0xe8, 0xc5, 0x1c, 0xb1, 0x90, 0xdf},
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), spanContext)
	ctx = core.WithLabels(ctx, core.Label("b", "two"), core.Label("a", "one"))

	record := slog.NewRecord(testhelp.Time, slog.LevelWarn, "How now brown cow?", testhelp.CallerPC(2))
	record.AddAttrs(slog.Int("count", 3))

	require.NoError(t, h.Handle(ctx, record))

	got := decodeLine(t, &buf)

	assert.Equal(t, "WARNING", got["severity"])
	assert.Equal(t, "How now brown cow?", got["message"])
	assert.Equal(t, "2000-01-02T03:04:05Z", got["timestamp"])
	assert.Equal(t, map[string]any{"a": "one", "b": "two"}, got["logging.googleapis.com/labels"])
	assert.Equal(t, "projects/my-project/traces/52fc1643a9381fc674742bb0067101e7", got["logging.googleapis.com/trace"])
	assert.Equal(t, "d3e9e8c51cb190df", got["logging.googleapis.com/spanId"])
	assert.Equal(t, true, got["logging.googleapis.com/trace_sampled"])
	assert.Equal(t, 3.0, got["count"])

	loc, ok := got["logging.googleapis.com/sourceLocation"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, loc["file"], "stdout/handler_test.go")
	assert.Contains(t, loc["function"], "TestHandler_agentFields")
	assert.IsType(t, "", loc["line"])
}

func TestHandler_groups(t *testing.T) {
	var buf bytes.Buffer

	l := slog.New(stdout.NewHandler(&buf)).
		With("svc", "api").
		WithGroup("req").With("id", "abc").
		WithGroup("user")

	l.Info("hello", "name", "jan", slog.Group("addr", "city", "Oslo"))

	got := decodeLine(t, &buf)

	assert.Equal(t, "INFO", got["severity"])
	assert.Equal(t, "hello", got["message"])
	assert.Equal(t, "api", got["svc"])
	assert.Equal(t, map[string]any{
		"id":   "abc",
		"user": map[string]any{"name": "jan", "addr": map[string]any{"city": "Oslo"}},
	}, got["req"])
}

func TestHandler_reservedKeysAreDropped(t *testing.T) {
	var buf bytes.Buffer

	slog.New(stdout.NewHandler(&buf)).Info("hello", "severity", "bogus", "timestamp", "bogus", "kept", 1)

	got := decodeLine(t, &buf)

	assert.Equal(t, "INFO", got["severity"])
	assert.NotEqual(t, "bogus", got["timestamp"])
	assert.Equal(t, 1.0, got["kept"])
}

// TestHandler_agentKeyGroupIsNotWritten verifies that a top-level group with
// a key that the agent reads is not written.
func TestHandler_agentKeyGroupIsNotWritten(t *testing.T) {
	var buf bytes.Buffer

	slog.New(stdout.NewHandler(&buf)).With("top", 1).WithGroup("severity").Info("hello", "k", 1)

	assert.Equal(t, 1, strings.Count(buf.String(), `"severity":`))

	got := decodeLine(t, &buf)

	assert.Equal(t, "INFO", got["severity"])
	assert.Equal(t, 1.0, got["top"])
}

func TestHandler_messageKeyIsNotDuplicated(t *testing.T) {
	var buf bytes.Buffer

	slog.New(stdout.NewHandler(&buf)).With("message", "fromWith").Info("hello", "message", "fromRecord")

	assert.Equal(t, 1, strings.Count(buf.String(), `"message":`))

	got := decodeLine(t, &buf)

	assert.Equal(t, "hello", got["message"])
}

func TestHandler_renamedMessageDoesNotShadowAgentKey(t *testing.T) {
	var buf bytes.Buffer

	rename := func(_ []string, a slog.Attr) slog.Attr {
		if a.Key == core.MessageKey {
			return slog.String("severity", a.Value.String())
		}

		return a
	}

	slog.New(stdout.NewHandler(&buf, core.WithReplaceAttr(rename))).Info("hello")

	assert.Equal(t, 1, strings.Count(buf.String(), `"severity":`))

	got := decodeLine(t, &buf)

	assert.Equal(t, "INFO", got["severity"])
	assert.NotContains(t, got, "message")
}

func TestHandler_noPCHasNoSourceLocation(t *testing.T) {
	var buf bytes.Buffer

	h := stdout.NewHandler(&buf, core.WithSourceAdded())
	record := slog.NewRecord(testhelp.Time, slog.LevelInfo, "hello", 0)

	require.NoError(t, h.Handle(context.Background(), record))

	got := decodeLine(t, &buf)

	assert.NotContains(t, got, "logging.googleapis.com/sourceLocation")
}

func TestHandler_zeroTimeHasNoTimestamp(t *testing.T) {
	var buf bytes.Buffer

	h := stdout.NewHandler(&buf)
	record := slog.NewRecord(time.Time{}, slog.LevelInfo, "hello", 0)

	require.NoError(t, h.Handle(context.Background(), record))

	got := decodeLine(t, &buf)

	assert.NotContains(t, got, "timestamp")
}

func TestHandler_writeError(t *testing.T) {
	h := stdout.NewHandler(failingWriter{})

	record := slog.NewRecord(testhelp.Time, slog.LevelInfo, "hello", 0)

	assert.Error(t, h.Handle(context.Background(), record))
}

func TestHandler_siblingsDoNotShareAttrs(t *testing.T) {
	var buf bytes.Buffer

	parent := stdout.NewHandler(&buf)
	first := slog.New(parent.WithAttrs([]slog.Attr{slog.String("a", "1")}))
	second := slog.New(parent.WithAttrs([]slog.Attr{slog.String("b", "2")}))

	slog.New(parent).Info("parent")
	gotParent := decodeLine(t, &buf)
	buf.Reset()

	first.Info("first")
	gotFirst := decodeLine(t, &buf)
	buf.Reset()

	second.Info("second")
	gotSecond := decodeLine(t, &buf)

	assert.NotContains(t, gotParent, "a")
	assert.NotContains(t, gotParent, "b")
	assert.Equal(t, "1", gotFirst["a"])
	assert.NotContains(t, gotFirst, "b")
	assert.Equal(t, "2", gotSecond["b"])
	assert.NotContains(t, gotSecond, "a")
}

func TestHandler_emptyGroupsAreNotWritten(t *testing.T) {
	var buf bytes.Buffer

	slog.New(stdout.NewHandler(&buf)).With("svc", "api").WithGroup("req").WithGroup("user").Info("hello")

	got := decodeLine(t, &buf)

	assert.Equal(t, "api", got["svc"])
	assert.NotContains(t, got, "req")
}

// When the mapper or the encoder elides every record attribute, the handler
// does not write a group that has no other content.
func TestHandler_elidedAttrsDoNotOpenGroups(t *testing.T) {
	drop := func(_ []string, a slog.Attr) slog.Attr {
		if a.Key == "password" {
			return slog.Attr{}
		}

		return a
	}

	tests := map[string]struct {
		log  func(l *slog.Logger)
		want map[string]any
	}{
		"replaced attr": {
			log: func(l *slog.Logger) {
				l.With("svc", "api").WithGroup("req").Info("hello", "password", "y")
			},
			want: map[string]any{"svc": "api"},
		},
		"unencodable attr in nested groups": {
			log: func(l *slog.Logger) {
				l.WithGroup("a").WithGroup("b").Info("hello", slog.Any("ch", make(chan int)))
			},
			want: map[string]any{},
		},
		"prefix in the middle group": {
			log: func(l *slog.Logger) {
				l.WithGroup("a").With("x", "1").WithGroup("b").Info("hello", "password", "y")
			},
			want: map[string]any{"a": map[string]any{"x": "1"}},
		},
		"group attr with an unencodable member": {
			log: func(l *slog.Logger) {
				l.Info("hello", slog.Group("g", slog.Any("ch", make(chan int))), "kept", 1)
			},
			want: map[string]any{"kept": 1.0},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer

			tc.log(slog.New(stdout.NewHandler(&buf, core.WithReplaceAttr(drop))))

			got := decodeLine(t, &buf)
			delete(got, "severity")
			delete(got, "message")
			delete(got, "timestamp")

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestHandler_baggageInGroup(t *testing.T) {
	var buf bytes.Buffer

	bag := otel.MustParse("a=one,b=two;p1;p2=val2")
	ctx := baggage.ContextWithBaggage(context.Background(), bag)

	slog.New(stdout.NewHandler(&buf, otel.WithOtelBaggage())).
		WithGroup("req").With("id", "abc").
		InfoContext(ctx, "hello", "n", 1)

	got := decodeLine(t, &buf)

	assert.Equal(t, map[string]any{
		"id":             "abc",
		"n":              1.0,
		"otel-baggage/a": "one",
		"otel-baggage/b": map[string]any{"value": "two", "properties": map[string]any{"p1": nil, "p2": "val2"}},
	}, got["req"])
}

// TestHandler_baggagePropertyKeysAreUnique verifies that a repeated property
// key is written once, with the last value, in key order.
func TestHandler_baggagePropertyKeysAreUnique(t *testing.T) {
	var buf bytes.Buffer

	bag := otel.MustParse("b=two;p2=v2;p1;p2=dup")
	ctx := baggage.ContextWithBaggage(context.Background(), bag)

	slog.New(stdout.NewHandler(&buf, otel.WithOtelBaggage())).InfoContext(ctx, "hello")

	assert.Equal(t, 1, strings.Count(buf.String(), `"p2":`))
	assert.Contains(t, buf.String(), `"properties":{"p1":null,"p2":"dup"}`)
}

// TestHandler_baggageGoesThroughReplaceAttr verifies that the mapper sees
// each baggage attribute.
func TestHandler_baggageGoesThroughReplaceAttr(t *testing.T) {
	var buf bytes.Buffer

	redact := func(_ []string, a slog.Attr) slog.Attr {
		if strings.Contains(a.Key, "email") {
			return slog.String(a.Key, "<redacted>")
		}

		return a
	}

	bag := otel.MustParse("email=jan@example.com")
	ctx := baggage.ContextWithBaggage(context.Background(), bag)

	slog.New(stdout.NewHandler(&buf, core.WithReplaceAttr(redact), otel.WithOtelBaggage())).InfoContext(ctx, "hello")

	got := decodeLine(t, &buf)

	assert.Equal(t, "<redacted>", got["otel-baggage/email"])
}

// TestHandler_baggageOrderIsStable verifies that the baggage attributes are
// in key order on every line.
func TestHandler_baggageOrderIsStable(t *testing.T) {
	const lines = 20

	var buf bytes.Buffer

	bag := otel.MustParse("e=5,a=1,d=4,b=2,c=3")
	ctx := baggage.ContextWithBaggage(context.Background(), bag)

	h := stdout.NewHandler(&buf, otel.WithOtelBaggage())
	record := slog.NewRecord(testhelp.Time, slog.LevelInfo, "hello", 0)

	for range lines {
		require.NoError(t, h.Handle(ctx, record))
	}

	got := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, got, lines)

	for _, line := range got {
		assert.Equal(t, got[0], line)
	}

	assert.Contains(t, got[0], `"otel-baggage/a":"1","otel-baggage/b":"2","otel-baggage/c":"3","otel-baggage/d":"4","otel-baggage/e":"5"`)
}

func TestHandler_replaceAttr(t *testing.T) {
	var buf bytes.Buffer

	remove := func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == "password" {
			return slog.Attr{}
		}

		if a.Key == core.MessageKey && len(groups) == 0 {
			return slog.String("msg", a.Value.String())
		}

		return a
	}

	slog.New(stdout.NewHandler(&buf, core.WithReplaceAttr(remove))).
		With("password", "x", "user", "jan").
		Info("hello", "password", "y", "n", 1)

	got := decodeLine(t, &buf)

	assert.Equal(t, "hello", got["msg"])
	assert.NotContains(t, got, "message")
	assert.NotContains(t, got, "password")
	assert.Equal(t, "jan", got["user"])
	assert.Equal(t, 1.0, got["n"])
}

func TestHandler_attrKinds(t *testing.T) {
	var buf bytes.Buffer

	type point struct {
		X, Y int
	}

	slog.New(stdout.NewHandler(&buf)).Info("kinds",
		slog.Int("i", -3),
		slog.Uint64("u", 7),
		slog.Float64("f", 2.5),
		slog.Bool("b", true),
		slog.Duration("d", 1500*time.Millisecond),
		slog.Time("t", testhelp.Time),
		slog.Any("err", errors.New("ouch")),
		slog.Any("p", point{1, 2}),
		slog.Any("nil", nil),
		slog.Any("bad", make(chan int)),
		slog.Group("g", slog.String("k", "v")),
		slog.Group("empty"),
	)

	got := decodeLine(t, &buf)

	assert.Equal(t, -3.0, got["i"])
	assert.Equal(t, 7.0, got["u"])
	assert.Equal(t, 2.5, got["f"])
	assert.Equal(t, true, got["b"])
	assert.Equal(t, 1.5e9, got["d"])
	assert.Equal(t, "2000-01-02T03:04:05.000Z", got["t"])
	assert.Equal(t, "ouch", got["err"])
	assert.Equal(t, map[string]any{"X": 1.0, "Y": 2.0}, got["p"])
	assert.Contains(t, got, "nil")
	assert.Nil(t, got["nil"])
	assert.NotContains(t, got, "bad")
	assert.Equal(t, map[string]any{"k": "v"}, got["g"])
	assert.NotContains(t, got, "empty")
}

func TestHandler_concurrentLinesDoNotInterleave(t *testing.T) {
	var buf bytes.Buffer

	l := slog.New(stdout.NewHandler(&buf))

	const (
		goroutines = 8
		perRoutine = 200
	)

	done := make(chan struct{})

	for g := range goroutines {
		go func() {
			for i := range perRoutine {
				l.Info("line", "g", g, "i", i, "pad", strings.Repeat("x", 200))
			}

			done <- struct{}{}
		}()
	}

	for range goroutines {
		<-done
	}

	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	require.Len(t, lines, goroutines*perRoutine)

	for _, line := range lines {
		var decoded map[string]any

		require.NoError(t, json.Unmarshal([]byte(line), &decoded), line)
	}
}
