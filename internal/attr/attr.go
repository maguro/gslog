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
	"sync"
	"time"

	spb "google.golang.org/protobuf/types/known/structpb"
)

var (
	timePool = sync.Pool{
		New: func() any {
			const prefixLen = len(time.RFC3339Nano) + 1
			b := make([]byte, 0, prefixLen)
			return &b
		},
	}

	NilValue = &spb.Value{Kind: &spb.Value_NullValue{NullValue: spb.NullValue_NULL_VALUE}}
)

// WrapAttrMapper will wrap an mapper with empty group checks to ensure they
// are properly elided.
func WrapAttrMapper(mapper func(groups []string, a slog.Attr) slog.Attr) func(groups []string, a slog.Attr) slog.Attr {
	if mapper == nil {
		return nil
	}

	var wrapped func(groups []string, a slog.Attr) slog.Attr

	wrapped = func(groups []string, a slog.Attr) slog.Attr {
		if a.Value.Kind() == slog.KindGroup {
			var attrs []any
			for _, ga := range a.Value.Group() {
				ma := wrapped(append(groups, a.Key), ga)

				// elide empty attributes
				if ma.Key == "" && ma.Value.Any() == nil {
					continue
				}

				attrs = append(attrs, ma)
			}

			if len(attrs) == 0 {
				return slog.Attr{}
			}

			return slog.Group(a.Key, attrs...)
		}

		return mapper(groups, a)
	}

	return wrapped
}

// DecorateWith will add the attribute to the spb.Struct's Fields.  If the
// attribute cannot be mapped to a spb.Value, nothing is done. Attributes
// of type slog.AnyAttribute are mapped using the following precedence.
//
//   - If of type builtin.error and does not implement json.Marshaler, the
//     Error() string is used.
//   - If attribute can be simply mappable to a spb.Value, that value is
//     used.
//   - If the attribute can be converted into a JSON object, that JSON object is
//     translated to its corresponding spb.Struct.
//   - Nothing is done.
func DecorateWith(p *spb.Struct, a slog.Attr) {
	rv := a.Value.Resolve()
	if a.Key == "" && rv.Any() == nil {
		return
	}
	val, ok := ValToStruct(rv)
	if !ok {
		return
	}
	if a.Key == "" && a.Value.Kind() == slog.KindGroup {
		for k, v := range val.GetStructValue().Fields {
			p.Fields[k] = v
		}
	} else {
		p.Fields[a.Key] = val
	}
}

func ValToStruct(v slog.Value) (val *spb.Value, ok bool) {
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
		a := v.Any()

		// if value is an error, but not a JSON marshaller, return error
		_, jm := a.(json.Marshaler)
		if err, ok := a.(error); ok && !jm {
			return &spb.Value{Kind: &spb.Value_StringValue{StringValue: err.Error()}}, true
		}

		// value may be simply mappable to a spb.Value.
		if nv, err := spb.NewValue(a); err == nil {
			return nv, true
		}

		// try converting to a JSON object
		return AsJson(a)
	default:
		return nil, false
	}
}

func MustValToStruct(v slog.Value) (val *spb.Value) {
	val, ok := ValToStruct(v)
	if !ok {
		panic("expected everything to be ok")
	}
	return val
}

func NewStringValue(str string) *spb.Value {
	return &spb.Value{Kind: &spb.Value_StringValue{StringValue: str}}
}

func NewNumberValue(val float64) *spb.Value {
	return &spb.Value{Kind: &spb.Value_NumberValue{NumberValue: val}}
}

func NewBoolValue(b bool) *spb.Value {
	return &spb.Value{Kind: &spb.Value_BoolValue{BoolValue: b}}
}

func NewGroupValue(g []slog.Attr) *spb.Value {
	p := &spb.Struct{Fields: make(map[string]*spb.Value)}
	for _, b := range g {
		DecorateWith(p, b)
	}
	return &spb.Value{Kind: &spb.Value_StructValue{StructValue: p}}
}

func NewTimeValue(t time.Time) *spb.Value {
	return &spb.Value{Kind: &spb.Value_StringValue{StringValue: AppendRFC3339Millis(t)}}
}

// AsJson attempts to convert the attribute a to a corresponding spb.Value
// by first converted to a JSON object and then mapping that JSON object to a
// corresponding spb.Value.  The function also returns true for ok if the
// attribute can be first converted to JSON before being mapped, and false
// otherwise.
func AsJson(a any) (value *spb.Value, ok bool) {
	if a == nil {
		return NilValue, true
	}

	a, err := ToJson(a)
	if err != nil {
		return nil, false
	}

	nv, _ := spb.NewValue(a)

	return nv, true
}

func ToJson(a any) (any, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(a); err != nil {
		return nil, err
	}

	var result any
	_ = json.Unmarshal(buf.Bytes(), &result)

	return result, nil
}

func AppendRFC3339Millis(t time.Time) string {
	ptr := timePool.Get().(*[]byte)
	buf := *ptr
	buf = buf[0:0]
	defer func() {
		*ptr = buf
		timePool.Put(ptr)
	}()
	buf = append(buf, byte('"'))

	// Format according to time.RFC3339Nano since it is highly optimized,
	// but truncate it to use millisecond resolution.
	// Unfortunately, that format trims trailing 0s, so add 1/10 millisecond
	// to guarantee that there are exactly 4 digits after the period.
	const prefixLen = len("2006-01-02T15:04:05.000")
	n := len(buf)
	t = t.Truncate(time.Millisecond).Add(time.Millisecond / 10)
	buf = t.AppendFormat(buf, time.RFC3339Nano)
	buf = append(buf[:n+prefixLen], buf[n+prefixLen+1:]...) // drop the 4th digit

	buf = append(buf, byte('"'))

	return string(buf)
}
