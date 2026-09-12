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

package otel

import (
	"context"

	"cloud.google.com/go/logging"
	"go.opentelemetry.io/otel/baggage"
	spb "google.golang.org/protobuf/types/known/structpb"

	"m4o.io/gslog/internal/attr"
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
// The handler or the logging record can already have an attribute with the
// same key as a baggage attribute.  The baggage attribute has precedence.
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
func WithOtelBaggage() options.OptionProcessor {
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

func addBaggage(ctx context.Context, entry *logging.Entry, groups []string) {
	bag := baggage.FromContext(ctx)

	members := bag.Members()
	if len(members) == 0 {
		return
	}

	c := currentGroup(entry, groups)

	for _, m := range members {
		c.Fields[OtelBaggageKey+m.Key()] = baggageToGroup(m)
	}
}

func currentGroup(entry *logging.Entry, groups []string) *spb.Struct {
	//nolint:forcetypeassert
	payload := entry.Payload.(*spb.Struct)

	for _, group := range groups {
		value, ok := payload.GetFields()[group]
		if !ok {
			value = &spb.Value{
				Kind: &spb.Value_StructValue{
					StructValue: &spb.Struct{
						Fields: make(map[string]*spb.Value),
					},
				},
			}

			payload.Fields[group] = value
		}

		payload = value.GetStructValue()
	}

	return payload
}

func baggageToGroup(member baggage.Member) *spb.Value {
	props := member.Properties()
	if len(props) == 0 {
		return &spb.Value{
			Kind: &spb.Value_StringValue{
				StringValue: member.Value(),
			},
		}
	}

	fields := make(map[string]*spb.Value)
	group := &spb.Value{
		Kind: &spb.Value_StructValue{
			StructValue: &spb.Struct{
				Fields: fields,
			},
		},
	}

	fields["value"] = &spb.Value{
		Kind: &spb.Value_StringValue{
			StringValue: member.Value(),
		},
	}

	properties := make(map[string]*spb.Value, len(props))

	for _, prop := range props {
		var value *spb.Value

		val, has := prop.Value()
		if !has {
			value = attr.NewNilValue()
		} else {
			value = &spb.Value{
				Kind: &spb.Value_StringValue{
					StringValue: val,
				},
			}
		}

		properties[prop.Key()] = value
	}

	fields["properties"] = &spb.Value{
		Kind: &spb.Value_StructValue{
			StructValue: &spb.Struct{
				Fields: properties,
			},
		},
	}

	return group
}
