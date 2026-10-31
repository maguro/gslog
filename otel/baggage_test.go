// Copyright the original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
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
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/baggage"

	"m4o.io/gslog"
	"m4o.io/gslog/otel"
)

func TestWithBaggage(t *testing.T) {
	for _, test := range []struct {
		name    string
		baggage string
		want    []string
	}{
		{
			name:    "a member with no properties maps to a string value",
			baggage: "a=one",
			want:    []string{"otel-baggage/a=one"},
		},
		{
			name:    "a member with properties maps to a group",
			baggage: "b=two;p1;p2=val2",
			want:    []string{"otel-baggage/b=[value=two properties=[p1=<nil> p2=val2]]"},
		},
		{
			name:    "the members are in key order",
			baggage: "e=5,a=1,d=4,b=2,c=3",
			want: []string{
				"otel-baggage/a=1",
				"otel-baggage/b=2",
				"otel-baggage/c=3",
				"otel-baggage/d=4",
				"otel-baggage/e=5",
			},
		},
		{
			name:    "the properties are in key order",
			baggage: "b=two;p2=v2;p1",
			want:    []string{"otel-baggage/b=[value=two properties=[p1=<nil> p2=v2]]"},
		},
		{
			name:    "a repeated property key keeps the last value",
			baggage: "b=two;p2=v2;p1;p2=dup",
			want:    []string{"otel-baggage/b=[value=two properties=[p1=<nil> p2=dup]]"},
		},
		{
			name:    "a context with no baggage gets no attributes",
			baggage: "",
			want:    []string{},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			if test.baggage != "" {
				bag := otel.MustParse(test.baggage)
				ctx = baggage.ContextWithBaggage(ctx, bag)
			}

			e := entryOf(ctx, otel.WithBaggage())

			assert.Equal(t, test.want, rendered(e.Attrs))
		})
	}
}

// TestWithBaggage_propertyWithNoValueIsNil verifies that a property with no
// value maps to a nil value, and not to the text "<nil>".  The two have the
// same string form, so TestWithBaggage cannot tell them apart.
func TestWithBaggage_propertyWithNoValueIsNil(t *testing.T) {
	bag := otel.MustParse("b=two;p1")
	ctx := baggage.ContextWithBaggage(context.Background(), bag)

	e := entryOf(ctx, otel.WithBaggage())

	require.Len(t, e.Attrs, 1)

	member := e.Attrs[0].Value.Group()
	require.Len(t, member, 2)

	props := member[1].Value.Group()
	require.Len(t, props, 1)

	assert.Equal(t, slog.KindAny, props[0].Value.Kind())
	assert.Nil(t, props[0].Value.Any())
}

// TestWithBaggage_repeated verifies that more than one WithBaggage option
// adds the baggage once.
func TestWithBaggage_repeated(t *testing.T) {
	bag := otel.MustParse("a=one")
	ctx := baggage.ContextWithBaggage(context.Background(), bag)

	opts := []gslog.Option{otel.WithBaggage(), otel.WithBaggage()}

	e := entryOf(ctx, opts...)

	assert.Equal(t, []string{"otel-baggage/a=one"}, rendered(e.Attrs))
}

func TestMustParse_invalid(t *testing.T) {
	assert.Panics(t, func() {
		otel.MustParse("=")
	})
}
