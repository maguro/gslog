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

// noinspection GoNameStartsWithPackageName
const (
	// OtelBaggageKey is the prefix for keys obtained from the OpenTelemetry
	// Baggage to mitigate collision with other log attributes.
	OtelBaggageKey = "otel-baggage/"
)

// WithOtelBaggage returns an gslog option that directs that the slog.Handler
// to include OpenTelemetry baggage.  The baggage.Baggage is obtained from the
// context, if available, and added as attributes.
//
// The baggage keys are prefixed with "otel-baggage/" to mitigate collision
// with other log attributes.  Baggage that have no properties are mapped to
// an slog.Attr for a string value.  Baggage that have properties mapped to a
// slog.Group with two keys, "value" which is the value of the baggage, and
// "properties" which is the properties of the baggage as a slog.Group.
// Baggage properties that have no value are mapped to slog.Any with a nil
// value.
//
// Baggage mapped attributes take precedence over any preexisting attributes
// that a handler or logging record may already have.
//
// For example, "a=one,b=two;p1;p2=val2" would map to
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

// MustParse wraps baggage.Parse to alleviate needless error checking
// when it's known, a priori, that an error can never happen.
func MustParse(bStr string) baggage.Baggage {
	b, err := baggage.Parse(bStr)
	if err != nil {
		panic(err)
	}
	return b
}

func addBaggage(ctx context.Context, e *logging.Entry, groups []string) {
	b := baggage.FromContext(ctx)

	if len(b.Members()) == 0 {
		return
	}

	c := currentGroup(e, groups)

	for _, m := range b.Members() {
		c.Fields[OtelBaggageKey+m.Key()] = baggageToGroup(m)
	}
}

func currentGroup(e *logging.Entry, groups []string) *spb.Struct {
	payload := e.Payload.(*spb.Struct)

	for _, group := range groups {
		v, ok := payload.Fields[group]
		if !ok {
			v = &spb.Value{
				Kind: &spb.Value_StructValue{
					StructValue: &spb.Struct{
						Fields: make(map[string]*spb.Value),
					},
				},
			}
			payload.Fields[group] = v
		}
		payload = v.GetStructValue()
	}

	return payload
}

func baggageToGroup(m baggage.Member) *spb.Value {
	if len(m.Properties()) == 0 {
		return &spb.Value{
			Kind: &spb.Value_StringValue{
				StringValue: m.Value(),
			},
		}
	}

	fields := make(map[string]*spb.Value)
	v := &spb.Value{
		Kind: &spb.Value_StructValue{
			StructValue: &spb.Struct{
				Fields: fields,
			},
		},
	}

	fields["value"] = &spb.Value{
		Kind: &spb.Value_StringValue{
			StringValue: m.Value(),
		},
	}

	properties := make(map[string]*spb.Value)

	for _, p := range m.Properties() {
		var value *spb.Value
		val, has := p.Value()
		if !has {
			value = attr.NilValue
		} else {
			value = &spb.Value{
				Kind: &spb.Value_StringValue{
					StringValue: val,
				},
			}
		}
		properties[p.Key()] = value
	}

	fields["properties"] = &spb.Value{
		Kind: &spb.Value_StructValue{
			StructValue: &spb.Struct{
				Fields: properties,
			},
		},
	}

	return v
}
