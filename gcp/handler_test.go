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
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/logging"
	logpb "cloud.google.com/go/logging/apiv2/loggingpb"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/baggage"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"m4o.io/gslog/core"
	"m4o.io/gslog/gcp"
	"m4o.io/gslog/internal/attr"
	"m4o.io/gslog/internal/options"
	"m4o.io/gslog/internal/testhelp"
	"m4o.io/gslog/otel"
)

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

func (g *Got) Flush() error {
	return nil
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
			var h slog.Handler = gcp.NewHandler(got, core.WithDefaultLogLeveler(slog.LevelInfo))
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
	var h slog.Handler = gcp.NewHandler(
		gcp.LoggerFunc(func(e logging.Entry) {
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
		core.WithDefaultLogLeveler(slog.LevelInfo))

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

// TestConcurrentGroupMapper verifies that concurrent log calls with a group
// attribute each see their own group path in the mapper.
func TestConcurrentGroupMapper(t *testing.T) {
	const (
		goroutines = 8
		iterations = 500
	)

	var wrong atomic.Int64

	mapper := func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == "k" && groups[len(groups)-1] != a.Value.String() {
			wrong.Add(1)
		}

		return a
	}

	h := gcp.NewHandler(gcp.LoggerFunc(func(logging.Entry) {}), core.WithReplaceAttr(mapper))
	l := slog.New(h).WithGroup("a").WithGroup("b").WithGroup("c")

	var wg sync.WaitGroup

	for i := range goroutines {
		wg.Go(func() {
			name := fmt.Sprintf("g%d", i)
			for range iterations {
				l.Info("m", slog.Group(name, slog.String("k", name)))
			}
		})
	}

	wg.Wait()

	assert.Zero(t, wrong.Load())
}

// TestJSONAndTextHandlers verifies the common parts of TextHandler and
// JSONHandler.
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "remove built-in",
			replace: removeKeys(core.MessageKey),
			attrs:   attrs,
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("a", "one"))
				attr.DecorateWith(p, slog.Int("b", 2))

				return logging.Entry{
					Payload:   p,
					Timestamp: testhelp.Time.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "preformatted remove built-in",
			replace: removeKeys(core.MessageKey),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			// Test resolution when there is no Mapper function.
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
					Severity:  logging.Info,
				}
			},
		},
		{
			name:    "GroupValue as slog.Attr value",
			replace: removeKeys(),
			attrs:   []slog.Attr{{Key: "v", Value: slog.AnyValue(slog.IntValue(3))}},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))
				attr.DecorateWith(p, slog.Int("v", 3))

				return logging.Entry{
					Payload:   p,
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
				File:     "gcp/handler_test.go",
				Line:     1,
				Function: "m4o.io/gslog/gcp_test.TestJSONAndTextHandlers",
			},
			want: func() logging.Entry {
				p := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
				attr.DecorateWith(p, slog.String("message", "message"))

				return logging.Entry{
					Payload:   p,
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
					Timestamp: testhelp.Time.UTC(),
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
				return removeKeys(core.MessageKey, "a")(groups, attr)
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
					Timestamp: testhelp.Time.UTC(),
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
				return removeKeys(core.MessageKey, "a", "b")(groups, attr)
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
					Timestamp: testhelp.Time.UTC(),
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
				return removeKeys(core.MessageKey, "a", "c")(groups, attr)
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
					Timestamp: testhelp.Time.UTC(),
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
				return removeKeys(core.MessageKey)(groups, a)
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
					Timestamp: testhelp.Time.UTC(),
					Severity:  logging.Info,
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := slog.NewRecord(testhelp.Time, slog.LevelInfo, "message", testhelp.CallerPC(2))
			line := source(r).Line
			r.AddAttrs(test.attrs...)

			opts := []options.OptionProcessor{
				core.WithReplaceAttr(test.replace),
				core.WithDefaultLogLeveler(slog.LevelInfo),
			}

			if test.addSource != nil {
				opts = append(opts, core.WithSourceAdded())
			}

			got := &Got{}
			var h slog.Handler = gcp.NewHandler(got, opts...)

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

// TestWithReplaceAttr_seesBaggage verifies that the mapper sees each
// baggage attribute.
// For a value that encoding/json cannot encode, the handler writes the text
// "!ERROR:" and the error.  The value is content for the groups around it.
func TestHandler_unencodableValueHasErrorText(t *testing.T) {
	const text = "!ERROR:json: unsupported type: chan int"

	tests := map[string]struct {
		log  func(l *slog.Logger)
		want map[string]any
	}{
		"top level": {
			log: func(l *slog.Logger) {
				l.Info("hello", slog.Any("ch", make(chan int)), "kept", 1)
			},
			want: map[string]any{"ch": text, "kept": 1.0},
		},
		"in nested groups": {
			log: func(l *slog.Logger) {
				l.WithGroup("a").WithGroup("b").Info("hello", slog.Any("ch", make(chan int)))
			},
			want: map[string]any{"a": map[string]any{"b": map[string]any{"ch": text}}},
		},
		"member of a group attr": {
			log: func(l *slog.Logger) {
				l.Info("hello", slog.Group("g", slog.Any("ch", make(chan int))))
			},
			want: map[string]any{"g": map[string]any{"ch": text}},
		},
		"from WithAttrs": {
			log: func(l *slog.Logger) {
				l.With(slog.Any("ch", make(chan int))).Info("hello")
			},
			want: map[string]any{"ch": text},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var got map[string]any

			logger := gcp.LoggerFunc(func(e logging.Entry) {
				payload, _ := e.Payload.(*structpb.Struct)
				got = payload.AsMap()
			})

			tc.log(slog.New(gcp.NewHandler(logger)))

			delete(got, "message")

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestWithReplaceAttr_seesBaggage(t *testing.T) {
	redact := func(_ []string, a slog.Attr) slog.Attr {
		if strings.Contains(a.Key, "email") {
			return slog.String(a.Key, "<redacted>")
		}

		return a
	}

	got := &Got{}
	h := gcp.NewHandler(got, core.WithReplaceAttr(redact), otel.WithOtelBaggage())

	bag := otel.MustParse("email=jan@example.com")
	ctx := baggage.ContextWithBaggage(context.Background(), bag)
	record := slog.NewRecord(testhelp.Time, slog.LevelInfo, "hello", 0)

	assert.NoError(t, h.Handle(ctx, record))

	payload, ok := got.LogEntry.Payload.(*structpb.Struct)
	assert.True(t, ok)
	assert.Equal(t, "<redacted>", payload.GetFields()["otel-baggage/email"].GetStringValue())
}

// TestWithSourceAdded_noPC verifies that a record with no program counter
// gets no source location.
func TestWithSourceAdded_noPC(t *testing.T) {
	got := &Got{}
	h := gcp.NewHandler(got, core.WithSourceAdded())

	record := slog.NewRecord(testhelp.Time, slog.LevelInfo, "hello", 0)

	assert.NoError(t, h.Handle(context.Background(), record))
	assert.Nil(t, got.LogEntry.SourceLocation)
}

func TestWithLeveler(t *testing.T) {
	got := &Got{}
	h := gcp.NewHandler(got, core.WithLogLeveler(slog.LevelInfo))

	l := slog.New(h.WithLeveler(slog.LevelError))

	l.Debug("How now brown cow")

	assert.Nil(t, got.LogEntry.Payload)

	l.Error("Ouch!")

	assert.NotNil(t, got.LogEntry.Payload)
}

// TestWithGroupSiblingsDoNotAlias derives two sibling handlers from a
// parent with three nested groups.  The groups slice of that parent has
// spare capacity.  Each sibling must log under its own group.
func TestWithGroupSiblingsDoNotAlias(t *testing.T) {
	got := &Got{}
	parent := gcp.NewHandler(got).WithGroup("a").WithGroup("b").WithGroup("c")

	first := parent.WithGroup("d")
	second := parent.WithGroup("e")

	slog.New(first).Info("first", "k", "v")
	firstPayload := got.LogEntry.Payload.(*structpb.Struct)

	slog.New(second).Info("second", "k", "v")
	secondPayload := got.LogEntry.Payload.(*structpb.Struct)

	assert.NotNil(t, firstPayload.GetFields()["a"].GetStructValue().
		GetFields()["b"].GetStructValue().
		GetFields()["c"].GetStructValue().
		GetFields()["d"], "first sibling should log under a.b.c.d")

	assert.NotNil(t, secondPayload.GetFields()["a"].GetStructValue().
		GetFields()["b"].GetStructValue().
		GetFields()["c"].GetStructValue().
		GetFields()["e"], "second sibling should log under a.b.c.e")
}

func TestLevelCritical(t *testing.T) {
	got := &Got{}
	h := gcp.NewHandler(got, core.WithLogLeveler(slog.LevelInfo))
	l := slog.New(h)

	l.Info("How now brown cow")

	assert.NotNil(t, got.LogEntry.Payload)
	assert.Nil(t, got.SyncLogEntry.Payload)

	got.LogEntry = logging.Entry{}

	l.Log(context.Background(), core.LevelCritical, "Ouch!")
	assert.Nil(t, got.LogEntry.Payload)
	assert.NotNil(t, got.SyncLogEntry.Payload)
}

// removeKeys returns a function for HandlerOptions.Mapper.  The function
// removes all Attrs that have the given keys.
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

func TestJson(t *testing.T) {
	s := `{"a":"one","b":"two"}`
	rj := json.RawMessage(s)
	stripped := strip(rj)

	raw, ok := stripped.(json.RawMessage)
	assert.True(t, ok)

	jb := []byte(raw)
	var m map[string]any
	err := json.Unmarshal(jb, &m)
	assert.NoError(t, err)
}

func strip(rj json.RawMessage) any {
	return rj
}
