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

package gslog

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	structpb "github.com/golang/protobuf/ptypes/struct"
	spb "google.golang.org/protobuf/types/known/structpb"
)

var (
	timePool = sync.Pool{
		New: func() any {
			b := make([]byte, 0, len(time.RFC3339Nano))
			return &b
		},
	}
)

// AttrMapper is called to rewrite each non-group attribute before it is logged.
// The attribute's value has been resolved (see [Value.Resolve]).
// If replaceAttr returns a zero Attr, the attribute is discarded.
//
// The built-in attributes with keys "time", "level", "source", and "msg"
// are passed to this function, except that time is omitted
// if zero, and source is omitted if addSource is false.
//
// The first argument is a list of currently open groups that contain the
// Attr. It must not be retained or modified. replaceAttr is never called
// for Group attributes, only their contents. For example, the attribute
// list
//
//	Int("a", 1), Group("g", Int("b", 2)), Int("c", 3)
//
// results in consecutive calls to replaceAttr with the following arguments:
//
//	nil, Int("a", 1)
//	[]string{"g"}, Int("b", 2)
//	nil, Int("c", 3)
//
// AttrMapper can be used to change the default keys of the built-in
// attributes, convert types (for example, to replace a `time.Time` with the
// integer seconds since the Unix epoch), sanitize personal information, or
// remove attributes from the output.
type AttrMapper func(groups []string, a slog.Attr) slog.Attr

func decorateWith(p *structpb.Struct, a slog.Attr) {
	a.Value.Resolve()
	val, ok := valToStruct(a.Value)
	if !ok {
		return
	}
	p.Fields[a.Key] = val
}

func valToStruct(v slog.Value) (val *structpb.Value, ok bool) {
	switch v.Kind() {
	case slog.KindString:
		return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: v.String()}}, true
	case slog.KindInt64:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(v.Int64())}}, true
	case slog.KindUint64:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(v.Uint64())}}, true
	case slog.KindFloat64:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: v.Float64()}}, true
	case slog.KindBool:
		return &structpb.Value{Kind: &structpb.Value_BoolValue{BoolValue: v.Bool()}}, true
	case slog.KindDuration:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(v.Duration())}}, true
	case slog.KindTime:
		ptr := timePool.Get().(*[]byte)
		buf := *ptr
		buf = buf[0:0]
		defer func() {
			*ptr = buf
			timePool.Put(ptr)
		}()
		buf = append(buf, byte('"'))
		buf = v.Time().AppendFormat(buf, time.RFC3339Nano)
		buf = append(buf, byte('"'))
		return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: string(buf)}}, true
	case slog.KindGroup:
		if len(v.Group()) == 0 {
			return nil, false
		}
		p2 := &structpb.Struct{Fields: make(map[string]*spb.Value)}
		for _, b := range v.Group() {
			decorateWith(p2, b)
		}
		return &structpb.Value{Kind: &structpb.Value_StructValue{StructValue: p2}}, true
	case slog.KindAny:
		a := v.Any()

		// if value is an error, but not a JSON marshaller, return error
		_, jm := a.(json.Marshaler)
		if err, ok := a.(error); ok && !jm {
			return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: err.Error()}}, true
		}

		// value may be simply mappable to a structpb.Value.
		if nv, err := spb.NewValue(a); err == nil {
			return nv, true
		}

		// try converting to a JSON object
		return asJson(a)
	default:
		return nil, false
	}
}

func fromPath(p *structpb.Struct, path []string) *structpb.Struct {
	for _, k := range path {
		p = p.Fields[k].GetStructValue()
	}
	if p.Fields == nil {
		p.Fields = make(map[string]*structpb.Value)
	}
	return p
}

func asJson(a any) (*structpb.Value, bool) {
	a, err := toJson(a)
	if err != nil {
		return nil, false
	}

	if nv, err := spb.NewValue(a); err == nil {
		return nv, true
	}

	return nil, false
}

func toJson(a any) (any, error) {
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
