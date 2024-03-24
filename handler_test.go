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

package gslog_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/logging"
	logpb "cloud.google.com/go/logging/apiv2/loggingpb"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"m4o.io/gslog"
	"m4o.io/gslog/internal/attr"
	"m4o.io/gslog/internal/options"
)

var testTime = time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)

type replace struct {
	v slog.Value
}

func (r *replace) LogValue() slog.Value { return r.v }

type Got struct {
	LogEntry     logging.Entry
	SyncLogEntry logging.Entry
}

func (g *Got) Log(e logging.Entry) {
	g.LogEntry = e
}

func (g *Got) LogSync(_ context.Context, e logging.Entry) error {
	g.SyncLogEntry = e
	return nil
}

// callerPC returns the program counter at the given stack depth.
func callerPC(depth int) uintptr {
	var pcs [1]uintptr
	runtime.Callers(depth, pcs[:])
	return pcs[0]
}

func TestDefaultHandle(t *testing.T) {
	ctx := context.Background()
	preAttrs := []slog.Attr{slog.Int("pre", 0)}
	attrs := []slog.Attr{slog.Int("a", 1), slog.String("b", "two")}
	for _, test := range []struct {
		name  string
		with  func(slog.Handler) slog.Handler
		attrs []slog.Attr
		want  func() logging.Entry
	}{
		{
			name: "no attrs",
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))

				return logging.Entry{
					Payload:   p,
					Timestamp: time.Time{}.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:  "attrs",
			attrs: attrs,
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.Int("a", 1))
				attr.DecorateWith(p, slog.String("b", "two"))

				return logging.Entry{
					Payload:   p,
					Timestamp: time.Time{}.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:  "preformatted",
			with:  func(h slog.Handler) slog.Handler { return h.WithAttrs(preAttrs) },
			attrs: attrs,
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.Int("pre", 0))
				attr.DecorateWith(p, slog.Int("a", 1))
				attr.DecorateWith(p, slog.String("b", "two"))

				return logging.Entry{
					Payload:   p,
					Timestamp: time.Time{}.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name: "groups",
			attrs: []slog.Attr{
				slog.Int("a", 1),
				slog.Group("g",
					slog.Int("b", 2),
					slog.Group("h", slog.Int("c", 3)),
					slog.Int("d", 4)),
				slog.Int("e", 5),
			},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.Int("a", 1))
				attr.DecorateWith(p, slog.Group("g",
					slog.Int("b", 2),
					slog.Group("h", slog.Int("c", 3)),
					slog.Int("d", 4)))
				attr.DecorateWith(p, slog.Int("e", 5))

				return logging.Entry{
					Payload:   p,
					Timestamp: time.Time{}.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:  "group",
			with:  func(h slog.Handler) slog.Handler { return h.WithAttrs(preAttrs).WithGroup("s") },
			attrs: attrs,
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.Int("pre", 0))
				attr.DecorateWith(p, slog.Group("s",
					slog.Int("a", 1),
					slog.String("b", "two")))

				return logging.Entry{
					Payload:   p,
					Timestamp: time.Time{}.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name: "preformatted groups",
			with: func(h slog.Handler) slog.Handler {
				return h.WithAttrs([]slog.Attr{slog.Int("p1", 1)}).
					WithGroup("s1").
					WithAttrs([]slog.Attr{slog.Int("p2", 2)}).
					WithGroup("s2")
			},
			attrs: attrs,
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.Int("p1", 1))
				attr.DecorateWith(p, slog.Group("s1",
					slog.Int("p2", 2),
					slog.Group("s2",
						slog.Int("a", 1),
						slog.String("b", "two"),
					),
				))

				return logging.Entry{
					Payload:   p,
					Timestamp: time.Time{}.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name: "two with-groups",
			with: func(h slog.Handler) slog.Handler {
				return h.WithAttrs([]slog.Attr{slog.Int("p1", 1)}).
					WithGroup("s1").
					WithGroup("s2")
			},
			attrs: attrs,
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.Int("p1", 1))
				attr.DecorateWith(p, slog.Group("s1",
					slog.Group("s2",
						slog.Int("a", 1),
						slog.String("b", "two"),
					),
				))

				return logging.Entry{
					Payload:   p,
					Timestamp: time.Time{}.UTC(),
					Severity:  logging.Info,
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := &Got{}
			var h slog.Handler = gslog.NewGcpHandler(got, gslog.WithDefaultLogLevel(slog.LevelInfo))
			if test.with != nil {
				h = test.with(h)
			}
			r := slog.NewRecord(time.Time{}, slog.LevelInfo, "message", 0)
			r.AddAttrs(test.attrs...)
			if err := h.Handle(ctx, r); err != nil {
				t.Fatal(err)
			}

			want := test.want()
			assert.Equal(t, want.Timestamp, got.LogEntry.Timestamp)
			assert.Equal(t, want.Severity, got.LogEntry.Severity)
			assert.True(t, proto.Equal(want.Payload.(proto.Message), got.LogEntry.Payload.(proto.Message)))
		})
	}
}

func TestConcurrentWrites(t *testing.T) {
	const count = 1000

	var mu sync.Mutex
	var s1Count int
	var s2Count int
	var h slog.Handler = gslog.NewGcpHandler(
		gslog.LoggerFunc(func(e logging.Entry) {
			mu.Lock()
			defer mu.Unlock()

			p := e.Payload.(*structpb.Struct)
			if _, ok := p.Fields["sub1"]; ok {
				s1Count++
			}
			if _, ok := p.Fields["sub2"]; ok {
				s2Count++
			}
		}),
		gslog.WithDefaultLogLevel(slog.LevelInfo))

	sub1 := h.WithAttrs([]slog.Attr{slog.Bool("sub1", true)})
	sub2 := h.WithAttrs([]slog.Attr{slog.Bool("sub2", true)})

	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		sub1Record := slog.NewRecord(time.Time{}, slog.LevelInfo, "hello from sub1", 0)
		sub1Record.AddAttrs(slog.Int("i", i))
		sub2Record := slog.NewRecord(time.Time{}, slog.LevelInfo, "hello from sub2", 0)
		sub2Record.AddAttrs(slog.Int("i", i))

		wg.Add(1)

		go func() {
			defer wg.Done()
			if err := sub1.Handle(ctx, sub1Record); err != nil {
				t.Error(err)
			}
			if err := sub2.Handle(ctx, sub2Record); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, count, s1Count)
	assert.Equal(t, count, s2Count)
}

// Verify the common parts of TextHandler and JSONHandler.
func TestJSONAndTextHandlers(t *testing.T) {
	// remove all Attrs
	removeAll := func(_ []string, a slog.Attr) slog.Attr { return slog.Attr{} }

	attrs := []slog.Attr{slog.String("a", "one"), slog.Int("b", 2), slog.Any("", nil)}
	preAttrs := []slog.Attr{slog.Int("pre", 3), slog.String("x", "y")}

	for _, test := range []struct {
		name      string
		replace   func([]string, slog.Attr) slog.Attr
		addSource *logpb.LogEntrySourceLocation
		with      func(slog.Handler) slog.Handler
		preAttrs  []slog.Attr
		attrs     []slog.Attr
		want      func() logging.Entry
	}{
		{
			name:  "basic",
			attrs: attrs,
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.String("a", "one"))
				attr.DecorateWith(p, slog.Int("b", 2))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:  "empty key",
			attrs: append(slices.Clip(attrs), slog.Any("", "v")),
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.String("a", "one"))
				attr.DecorateWith(p, slog.Int("b", 2))
				attr.DecorateWith(p, slog.Any("", "v"))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "cap keys",
			replace: upperCaseKey,
			attrs:   attrs,
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("MESSAGE", "message"))
				attr.DecorateWith(p, slog.String("A", "one"))
				attr.DecorateWith(p, slog.Int("B", 2))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "remove all",
			replace: removeAll,
			attrs:   attrs,
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:     "preformatted",
			with:     func(h slog.Handler) slog.Handler { return h.WithAttrs(preAttrs) },
			preAttrs: preAttrs,
			attrs:    attrs,
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.Int("pre", 3))
				attr.DecorateWith(p, slog.String("x", "y"))
				attr.DecorateWith(p, slog.String("a", "one"))
				attr.DecorateWith(p, slog.Int("b", 2))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:     "preformatted cap keys",
			replace:  upperCaseKey,
			with:     func(h slog.Handler) slog.Handler { return h.WithAttrs(preAttrs) },
			preAttrs: preAttrs,
			attrs:    attrs,
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("MESSAGE", "message"))
				attr.DecorateWith(p, slog.Int("PRE", 3))
				attr.DecorateWith(p, slog.String("X", "y"))
				attr.DecorateWith(p, slog.String("A", "one"))
				attr.DecorateWith(p, slog.Int("B", 2))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:     "preformatted remove all",
			replace:  removeAll,
			with:     func(h slog.Handler) slog.Handler { return h.WithAttrs(preAttrs) },
			preAttrs: preAttrs,
			attrs:    attrs,
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "remove built-in",
			replace: removeKeys(gslog.MessageKey),
			attrs:   attrs,
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("a", "one"))
				attr.DecorateWith(p, slog.Int("b", 2))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "preformatted remove built-in",
			replace: removeKeys(gslog.MessageKey),
			with:    func(h slog.Handler) slog.Handler { return h.WithAttrs(preAttrs) },
			attrs:   attrs,
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.Int("pre", 3))
				attr.DecorateWith(p, slog.String("x", "y"))
				attr.DecorateWith(p, slog.String("a", "one"))
				attr.DecorateWith(p, slog.Int("b", 2))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "groups",
			replace: removeKeys(), // to simplify the result
			attrs: []slog.Attr{
				slog.Int("a", 1),
				slog.Group("g",
					slog.Int("b", 2),
					slog.Group("h", slog.Int("c", 3)),
					slog.Int("d", 4)),
				slog.Int("e", 5),
			},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.Int("a", 1))
				attr.DecorateWith(p, slog.Group("g",
					slog.Int("b", 2),
					slog.Group("h", slog.Int("c", 3)),
					slog.Int("d", 4)))
				attr.DecorateWith(p, slog.Int("e", 5))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "empty group",
			replace: removeKeys(),
			attrs:   []slog.Attr{slog.Group("g"), slog.Group("h", slog.Int("a", 1))},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.Group("h",
					slog.Int("a", 1)))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "nested empty group",
			replace: removeKeys(),
			attrs: []slog.Attr{
				slog.Group("g",
					slog.Group("h",
						slog.Group("i"), slog.Group("j"))),
			},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "nested non-empty group",
			replace: removeKeys(),
			attrs: []slog.Attr{
				slog.Group("g",
					slog.Group("h",
						slog.Group("i"), slog.Group("j", slog.Int("a", 1)))),
			},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.Group("g",
					slog.Group("h",
						slog.Group("i"), slog.Group("j", slog.Int("a", 1)))))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "escapes",
			replace: removeKeys(),
			attrs: []slog.Attr{
				slog.String("a b", "x\t\n\000y"),
				slog.Group(" b.c=\"\\x2E\t",
					slog.String("d=e", "f.g\""),
					slog.Int("m.d", 1)), // dot is not escaped
			},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.String("a b", "x\t\n\000y"))
				attr.DecorateWith(p, slog.Group(" b.c=\"\\x2E\t",
					slog.String("d=e", "f.g\""),
					slog.Int("m.d", 1))) // dot is not escaped

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "LogValuer",
			replace: removeKeys(),
			attrs: []slog.Attr{
				slog.Int("a", 1),
				slog.Any("name", logValueName{"Ren", "Hoek"}),
				slog.Int("b", 2),
			},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.Int("a", 1))
				attr.DecorateWith(p, slog.Any("name", logValueName{"Ren", "Hoek"}))
				attr.DecorateWith(p, slog.Int("b", 2))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			// Test resolution when there is no ReplaceAttr function.
			name: "resolve",
			attrs: []slog.Attr{
				slog.Any("", &replace{slog.Value{}}), // should be elided
				slog.Any("name", logValueName{"Ren", "Hoek"}),
			},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.Any("name", logValueName{"Ren", "Hoek"}))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "with-group",
			replace: removeKeys(),
			with:    func(h slog.Handler) slog.Handler { return h.WithAttrs(preAttrs).WithGroup("s") },
			attrs:   attrs,
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.Int("pre", 3))
				attr.DecorateWith(p, slog.String("x", "y"))
				attr.DecorateWith(p, slog.Group("s",
					slog.String("a", "one"),
					slog.Int("b", 2)))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "preformatted with-groups",
			replace: removeKeys(),
			with: func(h slog.Handler) slog.Handler {
				return h.WithAttrs([]slog.Attr{slog.Int("p1", 1)}).
					WithGroup("s1").
					WithAttrs([]slog.Attr{slog.Int("p2", 2)}).
					WithGroup("s2").
					WithAttrs([]slog.Attr{slog.Int("p3", 3)})
			},
			attrs: attrs,
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.Int("p1", 1))
				attr.DecorateWith(p, slog.Group("s1",
					slog.Int("p2", 2),
					slog.Group("s2",
						slog.Int("p3", 3),
						slog.String("a", "one"),
						slog.Int("b", 2),
					),
				))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "two with-groups",
			replace: removeKeys(),
			with: func(h slog.Handler) slog.Handler {
				return h.WithAttrs([]slog.Attr{slog.Int("p1", 1)}).
					WithGroup("s1").
					WithGroup("s2")
			},
			attrs: attrs,
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.Int("p1", 1))
				attr.DecorateWith(p, slog.Group("s1",
					slog.Group("s2",
						slog.String("a", "one"),
						slog.Int("b", 2),
					),
				))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "empty with-groups",
			replace: removeKeys(),
			with: func(h slog.Handler) slog.Handler {
				return h.WithGroup("x").WithGroup("y")
			},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "with-group empty",
			replace: removeKeys(),
			with: func(h slog.Handler) slog.Handler {
				return h.WithGroup("").WithGroup("y").WithAttrs([]slog.Attr{slog.String("a", "one")})
			},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.Group("y", slog.String("a", "one")))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "empty with-groups, no non-empty attrs",
			replace: removeKeys(),
			with: func(h slog.Handler) slog.Handler {
				return h.WithGroup("x").WithAttrs([]slog.Attr{slog.Group("g")}).WithGroup("y")
			},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "one empty with-group",
			replace: removeKeys(),
			with: func(h slog.Handler) slog.Handler {
				return h.WithGroup("x").WithAttrs([]slog.Attr{slog.Int("a", 1)}).WithGroup("y")
			},
			attrs: []slog.Attr{slog.Group("g", slog.Group("h"))},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.Group("x",
					slog.Int("a", 1),
				))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "GroupValue as slog.Attr value",
			replace: removeKeys(),
			attrs:   []slog.Attr{{"v", slog.AnyValue(slog.IntValue(3))}},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.Int("v", 3))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "byte slice",
			replace: removeKeys(),
			attrs:   []slog.Attr{slog.Any("bs", []byte{1, 2, 3, 4})},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.String("bs", "AQIDBA=="))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "json.RawMessage",
			replace: removeKeys(),
			attrs:   []slog.Attr{slog.Any("bs", json.RawMessage("1234"))},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.Int("bs", 1234))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "inline group",
			replace: removeKeys(),
			attrs: []slog.Attr{
				slog.Int("a", 1),
				slog.Group("", slog.Int("b", 2), slog.Int("c", 3)),
				slog.Int("d", 4),
			},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.Int("a", 1))
				attr.DecorateWith(p, slog.Int("b", 2))
				attr.DecorateWith(p, slog.Int("c", 3))
				attr.DecorateWith(p, slog.Int("d", 4))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name: "Source",
			replace: func(gs []string, a slog.Attr) slog.Attr {
				if a.Key == slog.SourceKey {
					s := a.Value.Any().(*slog.Source)
					s.File = filepath.Base(s.File)
					return slog.Any(a.Key, s)
				}
				return removeKeys()(gs, a)
			},
			addSource: &logpb.LogEntrySourceLocation{
				File:     "gslog/handler_test.go",
				Line:     1,
				Function: "m4o.io/gslog/test.TestJSONAndTextHandlers",
			},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "replace empty",
			replace: func([]string, slog.Attr) slog.Attr { return slog.Attr{} },
			attrs:   []slog.Attr{slog.Group("g", slog.Int("a", 1))},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name: "replace empty 1",
			with: func(h slog.Handler) slog.Handler {
				return h.WithGroup("g").WithAttrs([]slog.Attr{slog.Int("a", 1)})
			},
			replace: func([]string, slog.Attr) slog.Attr { return slog.Attr{} },
			attrs:   []slog.Attr{slog.Group("h", slog.Int("b", 2))},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name: "replace empty 2",
			with: func(h slog.Handler) slog.Handler {
				return h.WithGroup("g").WithAttrs([]slog.Attr{slog.Int("a", 1)}).WithGroup("h").WithAttrs([]slog.Attr{slog.Int("b", 2)})
			},
			replace: func([]string, slog.Attr) slog.Attr { return slog.Attr{} },
			attrs:   []slog.Attr{slog.Group("i", slog.Int("c", 3))},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "replace empty 3",
			with:    func(h slog.Handler) slog.Handler { return h.WithGroup("g") },
			replace: func([]string, slog.Attr) slog.Attr { return slog.Attr{} },
			attrs:   []slog.Attr{slog.Int("a", 1)},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name: "replace empty inline",
			with: func(h slog.Handler) slog.Handler {
				return h.WithGroup("g").WithAttrs([]slog.Attr{slog.Int("a", 1)}).WithGroup("h").WithAttrs([]slog.Attr{slog.Int("b", 2)})
			},
			replace: func([]string, slog.Attr) slog.Attr { return slog.Attr{} },
			attrs:   []slog.Attr{slog.Group("", slog.Int("c", 3))},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name: "replace partial empty attrs 1",
			with: func(h slog.Handler) slog.Handler {
				return h.WithGroup("g").WithAttrs([]slog.Attr{slog.Int("a", 1)}).WithGroup("h").WithAttrs([]slog.Attr{slog.Int("b", 2)})
			},
			replace: func(groups []string, attr slog.Attr) slog.Attr {
				return removeKeys(gslog.MessageKey, "a")(groups, attr)
			},
			attrs: []slog.Attr{slog.Group("i", slog.Int("c", 3))},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.Group("g",
					slog.Group("h",
						slog.Int("b", 2),
						slog.Group("i",
							slog.Int("c", 3)),
					),
				))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name: "replace partial empty attrs 2",
			with: func(h slog.Handler) slog.Handler {
				return h.WithGroup("g").WithAttrs([]slog.Attr{slog.Int("a", 1)}).WithAttrs([]slog.Attr{slog.Int("n", 4)}).WithGroup("h").WithAttrs([]slog.Attr{slog.Int("b", 2)})
			},
			replace: func(groups []string, attr slog.Attr) slog.Attr {
				return removeKeys(gslog.MessageKey, "a", "b")(groups, attr)
			},
			attrs: []slog.Attr{slog.Group("i", slog.Int("c", 3))},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.Group("g",
					slog.Int("n", 4),
					slog.Group("h",
						slog.Group("i",
							slog.Int("c", 3)),
					),
				))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name: "replace partial empty attrs 3",
			with: func(h slog.Handler) slog.Handler {
				return h.WithGroup("g").
					WithAttrs([]slog.Attr{slog.Int("x", 0)}).
					WithAttrs([]slog.Attr{slog.Int("a", 1)}).
					WithAttrs([]slog.Attr{slog.Int("n", 4)}).
					WithGroup("h").WithAttrs([]slog.Attr{slog.Int("b", 2)})
			},
			replace: func(groups []string, attr slog.Attr) slog.Attr {
				return removeKeys(gslog.MessageKey, "a", "c")(groups, attr)
			},
			attrs: []slog.Attr{slog.Group("i", slog.Int("c", 3))},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.Group("g",
					slog.Int("x", 0),
					slog.Int("n", 4),
					slog.Group("h",
						slog.Int("b", 2),
					),
				))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name: "replace resolved group",
			replace: func(groups []string, a slog.Attr) slog.Attr {
				if a.Value.Kind() == slog.KindGroup {
					return slog.Attr{Key: "bad", Value: slog.IntValue(1)}
				}
				return removeKeys(gslog.MessageKey)(groups, a)
			},
			attrs: []slog.Attr{slog.Any("name", logValueName{"Perry", "Platypus"})},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.Group("name",
					slog.String("first", "Perry"),
					slog.String("last", "Platypus"),
				))

				return logging.Entry{
					Payload:   p,
					Timestamp: testTime.UTC(),
					Severity:  logging.Info,
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := slog.NewRecord(testTime, slog.LevelInfo, "message", callerPC(2))
			line := source(r).Line
			r.AddAttrs(test.attrs...)

			var opts = []options.OptionProcessor{
				gslog.WithReplaceAttr(test.replace),
				gslog.WithDefaultLogLevel(slog.LevelInfo),
			}

			if test.addSource != nil {
				opts = append(opts, gslog.WithSourceAdded())
			}

			got := &Got{}
			var h slog.Handler = gslog.NewGcpHandler(got, opts...)

			if test.with != nil {
				h = test.with(h)
			}

			if err := h.Handle(context.Background(), r); err != nil {
				t.Fatal(err)
			}

			if test.want == nil {
				return
			}
			want := test.want()
			assert.Equal(t, want.Timestamp, got.LogEntry.Timestamp)
			assert.Equal(t, want.Severity, got.LogEntry.Severity)
			assert.True(t, proto.Equal(want.Payload.(proto.Message), got.LogEntry.Payload.(proto.Message)))

			if test.addSource != nil {
				actual := got.LogEntry.SourceLocation
				expected := test.addSource
				assert.Equal(t, int64(line), actual.Line)
				assert.Equal(t, expected.File, actual.File[len(actual.File)-len(expected.File):])
				assert.Equal(t, expected.Function, actual.Function[0:len(expected.Function)])
			}
		})
	}
}

func TestWithLeveler(t *testing.T) {
	got := &Got{}
	var h = gslog.NewGcpHandler(got, gslog.WithLogLevel(slog.LevelInfo))

	l := slog.New(h.WithLeveler(slog.LevelError))

	l.Debug("How now brown cow")

	assert.Nil(t, got.LogEntry.Payload)

	l.Error("Ouch!")

	assert.NotNil(t, got.LogEntry.Payload)
}

func TestLevelCritical(t *testing.T) {
	got := &Got{}
	var h = gslog.NewGcpHandler(got, gslog.WithLogLevel(slog.LevelInfo))
	l := slog.New(h)

	l.Info("How now brown cow")

	assert.NotNil(t, got.LogEntry.Payload)
	assert.Nil(t, got.SyncLogEntry.Payload)

	got.LogEntry = logging.Entry{}

	l.Log(context.Background(), gslog.LevelCritical, "Ouch!")
	assert.Nil(t, got.LogEntry.Payload)
	assert.NotNil(t, got.SyncLogEntry.Payload)
}

// removeKeys returns a function suitable for HandlerOptions.ReplaceAttr
// that removes all Attrs with the given keys.
func removeKeys(keys ...string) func([]string, slog.Attr) slog.Attr {
	return func(_ []string, a slog.Attr) slog.Attr {
		for _, k := range keys {
			if a.Key == k {
				return slog.Attr{}
			}
		}
		return a
	}
}

func upperCaseKey(_ []string, a slog.Attr) slog.Attr {
	a.Key = strings.ToUpper(a.Key)
	return a
}

type logValueName struct {
	first, last string
}

func (n logValueName) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("first", n.first),
		slog.String("last", n.last))
}

func source(r slog.Record) *slog.Source {
	fs := runtime.CallersFrames([]uintptr{r.PC})
	f, _ := fs.Next()
	return &slog.Source{
		Function: f.Function,
		File:     f.File,
		Line:     f.Line,
	}
}
