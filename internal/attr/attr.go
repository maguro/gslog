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

// Package attr contains code that maps slog.Attr attributes to their
// corresponding structpb.Value values.
package attr

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"math"
	"strconv"
	"time"

	spb "google.golang.org/protobuf/types/known/structpb"

	"m4o.io/gslog/internal/entry"
)

// errorPrefix is the start of the string that NewAny returns for an
// encoding/json error.  log/slog uses the same text.
const errorPrefix = "!ERROR:"

var nilValue = &spb.Value{Kind: &spb.Value_NullValue{NullValue: spb.NullValue_NULL_VALUE}}

// DecorateWith adds the attribute to the Fields of the spb.Struct.
// DecorateWith does nothing for a group with no attribute.  DecorateWith maps
// an attribute of kind slog.KindAny with this precedence:
//
//   - If the value is an error and does not implement json.Marshaler,
//     DecorateWith uses the Error() string.
//   - If the value has a type that encoding/json encodes without a
//     json.Marshaler, DecorateWith maps the value directly with the rules of
//     encoding/json.
//   - If encoding/json can encode the value, DecorateWith maps the JSON to a
//     spb.Value.
//   - DecorateWith uses the string "!ERROR:" followed by the error text of
//     encoding/json.
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
		payload.Fields[entry.ValidUTF8(attr.Key)] = val
	}
}

// ValToStruct creates the spb.Value equivalent of the supplied slog.Value value.
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

		group := NewGroupValue(v.Group())
		if len(group.GetStructValue().GetFields()) == 0 {
			return nil, false
		}

		return group, true
	case slog.KindAny:
		return NewAny(v.Any()), true
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

type listValue struct {
	value spb.Value
	kind  spb.Value_ListValue
}

// NewStringValue creates the spb.Value equivalent of the supplied string.
// NewStringValue replaces a byte that is not valid UTF-8 with U+FFFD, because
// a protobuf string must be valid UTF-8.
func NewStringValue(str string) *spb.Value {
	str = entry.ValidUTF8(str)
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

// NewListValue creates the spb.Value that holds the supplied spb.ListValue.
func NewListValue(l *spb.ListValue) *spb.Value {
	v := &listValue{kind: spb.Value_ListValue{ListValue: l}}
	v.value.Kind = &v.kind

	return &v.value
}

// NewGroupValue creates the spb.Value equivalent of the supplied slog.Attr
// slice.
func NewGroupValue(g []slog.Attr) *spb.Value {
	p := &spb.Struct{Fields: make(map[string]*spb.Value)}
	for _, b := range g {
		DecorateWith(p, b)
	}

	return NewStructValue(p)
}

// NewAny creates the spb.Value equivalent of the supplied any instance.  The
// result is the value that encoding/json gives for the instance.  A nil
// instance is null.  An error that does not implement json.Marshaler is its
// Error() string.  If encoding/json cannot encode the instance, or cannot
// decode the JSON of the instance, NewAny returns the string "!ERROR:"
// followed by the error text.
func NewAny(a any) *spb.Value {
	if a == nil {
		return nilValue
	}

	// If the value is an error but not a json.Marshaler, return the error
	// text.
	_, jm := a.(json.Marshaler)
	if err, ok := a.(error); ok && !jm {
		return NewStringValue(err.Error())
	}

	if nv, ok := newJSONValue(a); ok {
		return nv
	}

	value, err := AsJSON(a)
	if err != nil {
		return NewStringValue(errorPrefix + err.Error())
	}

	return value
}

// newJSONValue returns the spb.Value that encoding/json gives for a, without
// a JSON round trip.  newJSONValue handles the types that encoding/json
// encodes without a json.Marshaler: nil, bool, string, the integer and float
// types, []byte, map[string]any, and []any.  ok is false for another type
// and for a float that is not finite.  The caller then uses encoding/json.
// jsonInteger handles the integer types.
func newJSONValue(a any) (*spb.Value, bool) {
	switch v := a.(type) {
	case nil:
		return nilValue, true
	case bool:
		return NewBoolValue(v), true
	case string:
		return NewStringValue(v), true
	case float64:
		return jsonFloat64(v)
	case float32:
		return jsonFloat32(v)
	case []byte:
		return jsonBytes(v)
	case map[string]any:
		return jsonStruct(v)
	case []any:
		return jsonList(v)
	default:
		return jsonInteger(a)
	}
}

// jsonInteger returns the number value of a if a has an integer type.  ok is
// false for another type.
func jsonInteger(a any) (*spb.Value, bool) {
	switch v := a.(type) {
	case int:
		return NewNumberValue(float64(v)), true
	case int8:
		return NewNumberValue(float64(v)), true
	case int16:
		return NewNumberValue(float64(v)), true
	case int32:
		return NewNumberValue(float64(v)), true
	case int64:
		return NewNumberValue(float64(v)), true
	case uint:
		return NewNumberValue(float64(v)), true
	case uint8:
		return NewNumberValue(float64(v)), true
	case uint16:
		return NewNumberValue(float64(v)), true
	case uint32:
		return NewNumberValue(float64(v)), true
	case uint64:
		return NewNumberValue(float64(v)), true
	default:
		return nil, false
	}
}

// jsonFloat64 returns the number value of f.  ok is false if f is not
// finite, because encoding/json cannot encode NaN or an infinity.
func jsonFloat64(f float64) (*spb.Value, bool) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return nil, false
	}

	return NewNumberValue(f), true
}

// jsonFloat32 returns the number value that encoding/json gives for f: the
// shortest decimal that reads back as f at 32 bits, read as a float64.
func jsonFloat32(f float32) (*spb.Value, bool) {
	f64 := float64(f)
	if math.IsNaN(f64) || math.IsInf(f64, 0) {
		return nil, false
	}

	text := strconv.FormatFloat(f64, 'g', -1, 32)
	n, _ := strconv.ParseFloat(text, 64)

	return NewNumberValue(n), true
}

// jsonBytes returns the value that encoding/json gives for b: null for a nil
// slice, and the base64 text for another slice.
func jsonBytes(b []byte) (*spb.Value, bool) {
	if b == nil {
		return nilValue, true
	}

	text := base64.StdEncoding.EncodeToString(b)

	return NewStringValue(text), true
}

// jsonStruct returns the value that encoding/json gives for m: null for a
// nil map, and a struct with the value of each member.  ok is false if a
// member is not one that newJSONValue handles.
func jsonStruct(m map[string]any) (*spb.Value, bool) {
	if m == nil {
		return nilValue, true
	}

	s := &spb.Struct{Fields: make(map[string]*spb.Value, len(m))}

	for k, v := range m {
		fv, ok := newJSONValue(v)
		if !ok {
			return nil, false
		}

		s.Fields[entry.ValidUTF8(k)] = fv
	}

	return NewStructValue(s), true
}

// jsonList returns the value that encoding/json gives for l: null for a nil
// slice, and a list with the value of each element.  ok is false if an
// element is not one that newJSONValue handles.
func jsonList(l []any) (*spb.Value, bool) {
	if l == nil {
		return nilValue, true
	}

	values := make([]*spb.Value, len(l))

	for i, v := range l {
		fv, ok := newJSONValue(v)
		if !ok {
			return nil, false
		}

		values[i] = fv
	}

	list := &spb.ListValue{Values: values}

	return NewListValue(list), true
}

// NewTimeValue creates the spb.Value equivalent of the supplied time.Time
// instance.  The text keeps the nanosecond resolution of t, and the
// time.RFC3339Nano layout removes the trailing zeros of the fraction.
func NewTimeValue(t time.Time) *spb.Value {
	formatted := t.Format(time.RFC3339Nano)

	return NewStringValue(formatted)
}

// AsJSON tries to convert the attribute a to a JSON object.  AsJSON then
// maps that JSON object to a spb.Value.  AsJSON returns the error of ToJSON
// if the conversion to JSON does not succeed.
func AsJSON(a any) (*spb.Value, error) {
	a, err := ToJSON(a)
	if err != nil {
		return nil, err
	}

	value, _ := spb.NewValue(a)

	return value, nil
}

// ToJSON converts an instance of any to a JSON object, map[string]any.
// ToJSON returns an error if it cannot encode the instance as JSON, or cannot
// decode that JSON.
func ToJSON(a any) (any, error) {
	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)

	if err := enc.Encode(a); err != nil {
		return nil, err
	}

	var result any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		return nil, err
	}

	return result, nil
}
