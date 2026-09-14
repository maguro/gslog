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

package otel

import (
	"context"
	"log/slog"
	"slices"
	"strings"

	"go.opentelemetry.io/otel/baggage"

	"m4o.io/gslog/core"
	"m4o.io/gslog/internal/entry"
	"m4o.io/gslog/internal/options"
)

// noinspection GoNameStartsWithPackageName.
const (
	// OtelBaggageKey is the prefix for keys that come from the OpenTelemetry
	// Baggage.  The prefix makes collisions with other log attributes less
	// likely.
	OtelBaggageKey = "otel-baggage/"
)

// WithOtelBaggage returns a gslog option that causes the handler to include
// OpenTelemetry baggage.  The handler gets the baggage.Baggage from the
// context, if the context has one, and adds the baggage as attributes.
//
// The handler adds the prefix "otel-baggage/" to each baggage key.  The
// prefix makes collisions with other log attributes less likely.  The
// handler maps a member that has no properties to a slog.Attr with a string
// value.  The handler maps a member that has properties to a slog.Group with
// two keys.  The key "value" holds the value of the member.  The key
// "properties" holds the properties of the member as a slog.Group.  The
// handler maps a property that has no value to slog.Any with a nil value.
//
// The handler adds the baggage attributes in key order, after the
// attributes of the log call, and passes each one to the AttrMapper.
//
// The handler or the logging record can already have an attribute with the
// same key as a baggage attribute.  The gcp handler replaces that attribute
// with the baggage attribute.  The stdout handler writes the baggage
// attribute after that attribute.
//
// For example, "a=one,b=two;p1;p2=val2" maps to
//
//	slog.String("otel-baggage/a", "one")
//	slog.Group("otel-baggage/b",
//		slog.String("value", "two"),
//		slog.Group("properties",
//			slog.Any("p1", nil),
//			slog.String("p2", "val2"),
//		),
//	)
func WithOtelBaggage() core.Option {
	return func(options *options.Options) {
		options.EntryAugmentors = append(options.EntryAugmentors, addBaggage)
	}
}

// MustParse wraps baggage.Parse.  Use MustParse when an error cannot occur,
// so that the caller does not need to check for an error.
func MustParse(bStr string) baggage.Baggage {
	bag, err := baggage.Parse(bStr)
	if err != nil {
		panic(err)
	}

	return bag
}

// addBaggage appends one attribute per baggage member to the entry, in key
// order.
func addBaggage(ctx context.Context, e *entry.Entry) {
	bag := baggage.FromContext(ctx)

	members := bag.Members()
	slices.SortFunc(members, compareKeys)

	e.Attrs = slices.Grow(e.Attrs, len(members))

	for _, m := range members {
		e.Attrs = append(e.Attrs, baggageAttr(m))
	}
}

// keyed is a baggage member or a baggage property.
type keyed interface {
	Key() string
}

// compareKeys orders two baggage members or properties by key.
func compareKeys[T keyed](a, b T) int {
	return strings.Compare(a.Key(), b.Key())
}

// baggageAttr maps a baggage member to a slog.Attr.  The properties are in
// key order.  A property replaces an earlier property with the same key.
func baggageAttr(member baggage.Member) slog.Attr {
	key := OtelBaggageKey + member.Key()

	props := member.Properties()
	if len(props) == 0 {
		return slog.String(key, member.Value())
	}

	slices.SortStableFunc(props, compareKeys)

	properties := make([]slog.Attr, 0, len(props))

	for _, prop := range props {
		a := propertyAttr(prop)

		if n := len(properties); n > 0 && properties[n-1].Key == a.Key {
			properties[n-1] = a

			continue
		}

		properties = append(properties, a)
	}

	return slog.Attr{
		Key: key,
		Value: slog.GroupValue(
			slog.String("value", member.Value()),
			slog.Attr{Key: "properties", Value: slog.GroupValue(properties...)},
		),
	}
}

// propertyAttr maps a baggage property to a slog.Attr.  A property with no
// value maps to a nil value.
func propertyAttr(prop baggage.Property) slog.Attr {
	val, has := prop.Value()
	if !has {
		return slog.Any(prop.Key(), nil)
	}

	return slog.String(prop.Key(), val)
}
