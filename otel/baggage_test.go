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

package otel_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/baggage"
	"google.golang.org/protobuf/proto"
	spb "google.golang.org/protobuf/types/known/structpb"

	"m4o.io/gslog"
	"m4o.io/gslog/internal/attr"
	"m4o.io/gslog/otel"
)

func TestWithOtelBaggage(t *testing.T) {
	b := otel.MustParse("a=one,b=two;prop1;prop2=1")
	for _, test := range []struct {
		name    string
		groups  []string
		attrs   []slog.Attr
		baggage baggage.Baggage
		want    func() *spb.Struct
	}{
		{
			name:    "a=one,b=two;prop1;prop2=1",
			baggage: b,
			want: func() *spb.Struct {
				p := &spb.Struct{Fields: make(map[string]*spb.Value)}
				attr.DecorateWith(p, slog.String("message", "how now brown cow"))
				attr.DecorateWith(p, slog.String("otel-baggage/a", "one"))
				attr.DecorateWith(p, slog.Group("otel-baggage/b",
					slog.String("value", "two"),
					slog.Group("properties",
						slog.Any("prop1", nil),
						slog.String("prop2", "1"),
					),
				))

				return p
			},
		},
		{
			name:    "a=one,b=two;prop1;prop2=1 attr precedence",
			attrs:   []slog.Attr{slog.String("otel-baggage/a", "foo"), slog.String("otel-baggage/b", "bar")},
			baggage: b,
			want: func() *spb.Struct {
				p := &spb.Struct{Fields: make(map[string]*spb.Value)}
				attr.DecorateWith(p, slog.String("message", "how now brown cow"))
				attr.DecorateWith(p, slog.String("otel-baggage/a", "one"))
				attr.DecorateWith(p, slog.Group("otel-baggage/b",
					slog.String("value", "two"),
					slog.Group("properties",
						slog.Any("prop1", nil),
						slog.String("prop2", "1"),
					),
				))

				return p
			},
		},
		{
			name:    "a=one,b=two;prop1;prop2=1 within groups",
			groups:  []string{"g1", "g2"},
			baggage: b,
			want: func() *spb.Struct {
				p := &spb.Struct{Fields: make(map[string]*spb.Value)}
				attr.DecorateWith(p, slog.String("message", "how now brown cow"))
				attr.DecorateWith(p, slog.Group("g1",
					slog.Group("g2",
						slog.String("otel-baggage/a", "one"),
						slog.Group("otel-baggage/b",
							slog.String("value", "two"),
							slog.Group("properties",
								slog.Any("prop1", nil),
								slog.String("prop2", "1"),
							),
						),
					),
				))

				return p
			},
		},
		{
			name:    "no baggage",
			baggage: baggage.Baggage{},
			want: func() *spb.Struct {
				p := &spb.Struct{Fields: make(map[string]*spb.Value)}
				attr.DecorateWith(p, slog.String("message", "how now brown cow"))

				return p
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := &Got{}
			var h slog.Handler = gslog.NewGcpHandler(got, otel.WithOtelBaggage())

			for _, group := range test.groups {
				h = h.WithGroup(group)
			}
			if test.attrs != nil {
				h = h.WithAttrs(test.attrs)
			}

			ctx := context.Background()
			if test.baggage.Len() != 0 {
				ctx = baggage.ContextWithBaggage(ctx, test.baggage)
			}

			l := slog.New(h)
			l.Log(ctx, slog.LevelInfo, "how now brown cow")

			e := got.LogEntry

			expected := test.want()

			b, err := e.Payload.(*spb.Struct).MarshalJSON()
			assert.NoError(t, err)
			s := string(b)
			_ = s

			assert.True(t, proto.Equal(expected, e.Payload.(proto.Message)))
		})
	}
}
