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

/*
Package attr contains code that maps slog.Attr attributes to their
corresponding structpb.Value values.
*/
package attr

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/pkg/errors"
	spb "google.golang.org/protobuf/types/known/structpb"
)

//nolint:gochecknoglobals
var nilValue = &spb.Value{Kind: &spb.Value_NullValue{NullValue: spb.NullValue_NULL_VALUE}}

// The handler calls a Mapper to rewrite each non-group attribute before the
// handler logs the attribute.
type Mapper func(groups []string, attr slog.Attr) slog.Attr

// WrapAttrMapper wraps a mapper with checks for empty groups.  The wrapper
// elides an empty group.
func WrapAttrMapper(mapper Mapper) Mapper {
	if mapper == nil {
		return nil
	}

	var wrapped Mapper

	wrapped = func(groups []string, attr slog.Attr) slog.Attr {
		if attr.Value.Kind() == slog.KindGroup {
			var attrs []any

			for _, ga := range attr.Value.Group() {
				mapped := wrapped(append(groups, attr.Key), ga)

				// elide empty attributes
				if mapped.Key == "" && mapped.Value.Any() == nil {
					continue
				}

				attrs = append(attrs, mapped)
			}

			if len(attrs) == 0 {
				//nolint:exhaustruct
				return slog.Attr{}
			}

			return slog.Group(attr.Key, attrs...)
		}

		return mapper(groups, attr)
	}

	return wrapped
}

// DecorateWith adds the attribute to the Fields of the spb.Struct.  If
// DecorateWith cannot map the attribute to a spb.Value, DecorateWith does
// nothing.  DecorateWith maps an attribute of kind slog.KindAny with this
// precedence:
//
//   - If the value is a builtin.error and does not implement json.Marshaler,
//     DecorateWith uses the Error() string.
//   - If the value maps directly to a spb.Value, DecorateWith uses that
//     value.
//   - If DecorateWith can convert the value to a JSON object, DecorateWith
//     translates that JSON object to a spb.Struct.
//   - DecorateWith does nothing.
func DecorateWith(payload *spb.Struct, attr slog.Attr) {
	rv := attr.Value.Resolve()
	if attr.Key == "" && rv.Any() == nil {
		return
	}

	val, ok := ValToStruct(rv)
	if !ok {
		return
	}

	if attr.Key == "" && attr.Value.Kind() == slog.KindGroup {
		for k, v := range val.GetStructValue().GetFields() {
			payload.Fields[k] = v
		}
	} else {
		payload.Fields[attr.Key] = val
	}
}

// ValToStruct creates the spb.Value equivalent of the supplied slog.Value value.
//
//nolint:cyclop
func ValToStruct(v slog.Value) (*spb.Value, bool) {
	switch v.Kind() {
	case slog.KindString:
		return NewStringValue(v.String()), true
	case slog.KindInt64:
		return NewNumberValue(float64(v.Int64())), true
	case slog.KindUint64:
		return NewNumberValue(float64(v.Uint64())), true
	case slog.KindFloat64:
		return NewNumberValue(v.Float64()), true
	case slog.KindBool:
		return NewBoolValue(v.Bool()), true
	case slog.KindDuration:
		return NewNumberValue(float64(v.Duration())), true
	case slog.KindTime:
		return NewTimeValue(v.Time()), true
	case slog.KindGroup:
		if len(v.Group()) == 0 {
			return nil, false
		}

		return NewGroupValue(v.Group()), true
	case slog.KindAny:
		return NewAny(v.Any())
	default:
		return nil, false
	}
}

// NewNilValue is the spb.Value equivalent of nil.
func NewNilValue() *spb.Value {
	return nilValue
}

// These types hold a spb.Value and its Kind in one object.  The Kind field
// of the value points into the same object.
type stringValue struct {
	value spb.Value
	kind  spb.Value_StringValue
}

type numberValue struct {
	value spb.Value
	kind  spb.Value_NumberValue
}

type boolValue struct {
	value spb.Value
	kind  spb.Value_BoolValue
}

type structValue struct {
	value spb.Value
	kind  spb.Value_StructValue
}

// NewStringValue creates the spb.Value equivalent of the supplied string.
func NewStringValue(str string) *spb.Value {
	v := &stringValue{kind: spb.Value_StringValue{StringValue: str}}
	v.value.Kind = &v.kind

	return &v.value
}

// NewNumberValue creates the spb.Value equivalent of the supplied float64.
func NewNumberValue(val float64) *spb.Value {
	v := &numberValue{kind: spb.Value_NumberValue{NumberValue: val}}
	v.value.Kind = &v.kind

	return &v.value
}

// NewBoolValue creates the spb.Value equivalent of the supplied bool.
func NewBoolValue(b bool) *spb.Value {
	v := &boolValue{kind: spb.Value_BoolValue{BoolValue: b}}
	v.value.Kind = &v.kind

	return &v.value
}

// NewStructValue creates the spb.Value that holds the supplied spb.Struct.
func NewStructValue(s *spb.Struct) *spb.Value {
	v := &structValue{kind: spb.Value_StructValue{StructValue: s}}
	v.value.Kind = &v.kind

	return &v.value
}

// NewGroupValue creates the spb.Value equivalent of the supplied slog.Attr array.
func NewGroupValue(g []slog.Attr) *spb.Value {
	p := &spb.Struct{Fields: make(map[string]*spb.Value)}
	for _, b := range g {
		DecorateWith(p, b)
	}

	return NewStructValue(p)
}

// NewAny creates the spb.Value equivalent of the supplied any instance.
func NewAny(a any) (*spb.Value, bool) {
	// If the value is an error but not a json.Marshaler, return the error
	// text.
	_, jm := a.(json.Marshaler)
	if err, ok := a.(error); ok && !jm {
		return NewStringValue(err.Error()), true
	}

	// The value can map directly to a spb.Value.
	if nv, err := spb.NewValue(a); err == nil {
		return nv, true
	}

	// Try to convert the value to a JSON object.
	return AsJSON(a)
}

// NewTimeValue creates the spb.Value equivalent of the supplied time.Time instance.
func NewTimeValue(t time.Time) *spb.Value {
	return NewStringValue(TimeToRFC3339InMs(t))
}

// AsJSON tries to convert the attribute a to a JSON object.  AsJSON then
// maps that JSON object to a spb.Value.  AsJSON returns true for ok if the
// conversion to JSON succeeds, and false if it does not.
func AsJSON(a any) (*spb.Value, bool) {
	if a == nil {
		return nilValue, true
	}

	a, err := ToJSON(a)
	if err != nil {
		return nil, false
	}

	value, _ := spb.NewValue(a)

	return value, true
}

// ToJSON converts an instance of any to a JSON object, map[string]interface{}.
// ToJSON returns an error if it cannot encode the instance as JSON.
func ToJSON(a any) (any, error) {
	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)

	if err := enc.Encode(a); err != nil {
		return nil, errors.Wrap(err, "unable to encode attr")
	}

	var result any
	_ = json.Unmarshal(buf.Bytes(), &result)

	return result, nil
}

// TimeToRFC3339InMs formats an instance of time.Time in the RFC3339 layout
// with millisecond resolution.  The function is optimized for speed.
func TimeToRFC3339InMs(t time.Time) string {
	// Format with time.RFC3339Nano because that format is highly optimized.
	// Truncate the result to millisecond resolution.
	const prefixLen = len("2006-01-02T15:04:05.000")

	// That format trims trailing zeros.  Because of this, add 1/10
	// millisecond to make sure that there are exactly 4 digits after the
	// period.
	const rounding = time.Millisecond / 10

	var arr [len(time.RFC3339Nano)]byte

	t = t.Truncate(time.Millisecond).Add(rounding)

	buf := t.AppendFormat(arr[:0], time.RFC3339Nano)
	buf = append(buf[:prefixLen], buf[prefixLen+1:]...) // drop the 4th digit

	return string(buf)
}
