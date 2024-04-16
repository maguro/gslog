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

package attr_test

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"

	"m4o.io/gslog/internal/attr"
)

type Circular struct {
	Self *Circular `json:"self"`
}

type Manager struct{}

type Password string

func (p Password) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote("<secret>")), nil
}

func (p Password) LogValue() slog.Value {
	return pwObfuscated
}

type User struct {
	ID        string   `json:"id"`
	FirstName string   `json:"first_name"`
	LastName  string   `json:"last_name"`
	Email     string   `json:"email"`
	Password  Password `json:"password"`
	Age       uint8    `json:"age"`
	Height    float32  `json:"height"`
	Engineer  bool     `json:"engineer"`
	Manager   *Manager `json:"manager"`
}

type Chimera struct {
	Name string `json:"name"`
}

func (u *Chimera) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{"name":"%s"}`, u.Name)), nil
}

// Error should never be called since
func (u *Chimera) Error() string {
	panic("ouch")
}

var (
	pw           = Password("pass-12334")
	pwObfuscated = slog.StringValue("<secret>")
	u            = &User{
		ID:        "user-12234",
		FirstName: "Jan",
		LastName:  "Doe",
		Email:     "jan@example.com",
		Password:  pw,
		Age:       32,
		Height:    5.91,
		Engineer:  true,
	}

	uJson = map[string]interface{}{
		"id":         "user-12234",
		"first_name": "Jan",
		"last_name":  "Doe",
		"email":      "jan@example.com",
		"password":   "<secret>",
		"age":        float64(32),
		"height":     5.91,
		"engineer":   true,
		"manager":    nil,
	}

	uStruct *structpb.Value

	uGroup []slog.Attr

	circular *Circular

	chimera = &Chimera{Name: "Pookie Bear"}
	cStruct = &structpb.Value{
		Kind: &structpb.Value_StructValue{
			StructValue: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"name": {
						Kind: &structpb.Value_StringValue{StringValue: "Pookie Bear"},
					},
				},
			},
		},
	}
)

func init() {
	circular = &Circular{}
	circular.Self = circular

	fields := make(map[string]*structpb.Value)
	fields["id"] = &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: "user-12234"}}
	fields["first_name"] = &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: "Jan"}}
	fields["last_name"] = &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: "Doe"}}
	fields["email"] = &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: "jan@example.com"}}
	fields["password"] = &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: "<secret>"}}
	fields["age"] = &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(32)}}
	fields["height"] = &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: 5.91}}
	fields["engineer"] = &structpb.Value{Kind: &structpb.Value_BoolValue{BoolValue: true}}
	fields["manager"] = attr.NewNilValue()
	uStruct = &structpb.Value{
		Kind: &structpb.Value_StructValue{
			StructValue: &structpb.Struct{
				Fields: fields,
			},
		},
	}

	uGroup = append(uGroup, slog.String("id", "user-12234"))
	uGroup = append(uGroup, slog.String("first_name", "Jan"))
	uGroup = append(uGroup, slog.String("last_name", "Doe"))
	uGroup = append(uGroup, slog.String("email", "jan@example.com"))
	uGroup = append(uGroup, slog.Any("password", pw))
	uGroup = append(uGroup, slog.Uint64("age", 32))
	uGroup = append(uGroup, slog.Float64("height", 5.91))
	uGroup = append(uGroup, slog.Bool("engineer", true))
	uGroup = append(uGroup, slog.Any("manager", nil))
}

func TestToJson(t *testing.T) {
	tests := map[string]struct {
		attr any
		json any
		err  bool
	}{
		"ok":     {u, uJson, false},
		"simple": {"cow", "cow", false},
		"error":  {circular, nil, true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			json, err := attr.ToJSON(tc.attr)
			if tc.err {
				assert.Error(t, err)
			} else {
				assert.Equal(t, tc.json, json)
			}
		})
	}
}

func TestAsJson(t *testing.T) {
	tests := map[string]struct {
		attr  any
		value *structpb.Value
		ok    bool
	}{
		"nil":        {nil, attr.NewNilValue(), true},
		"not simple": {u, uStruct, true},
		"error":      {circular, nil, false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			value, ok := attr.AsJSON(tc.attr)
			assert.Equal(t, tc.ok, ok)
			if tc.ok {
				assert.Equal(t, tc.value, value)
			}
		})
	}
}

func TestValToStruct(t *testing.T) {
	now := time.Now().UTC()
	tests := map[string]struct {
		attr  slog.Value
		value *structpb.Value
		ok    bool
	}{
		"nil":                    {slog.AnyValue(nil), attr.NewNilValue(), true},
		"string":                 {slog.StringValue("how now brown cow"), attr.NewStringValue("how now brown cow"), true},
		"int64":                  {slog.Int64Value(math.MaxInt64), attr.NewNumberValue(float64(math.MaxInt64)), true},
		"uint64":                 {slog.Uint64Value(math.MaxUint64), attr.NewNumberValue(float64(math.MaxUint64)), true},
		"float64":                {slog.Float64Value(math.MaxFloat64), attr.NewNumberValue(math.MaxFloat64), true},
		"bool true":              {slog.BoolValue(true), attr.NewBoolValue(true), true},
		"bool false":             {slog.BoolValue(false), attr.NewBoolValue(false), true},
		"duration":               {slog.DurationValue(time.Minute * 5), attr.NewNumberValue(float64(time.Minute * 5)), true},
		"time":                   {slog.TimeValue(now), attr.NewTimeValue(now), true},
		"group":                  {slog.GroupValue(uGroup...), uStruct, true},
		"group empty":            {slog.GroupValue(), nil, false},
		"any LogValuer":          {slog.AnyValue(pw), nil, false}, // this should have been transformed earlier via Resolve()
		"any resolved LogValuer": {slog.AnyValue(pw).Resolve(), attr.NewStringValue("<secret>"), true},
		"any JSON":               {slog.AnyValue(u), uStruct, true},
		"any json.Marshaler":     {slog.AnyValue(chimera), cStruct, true},
		"any error":              {slog.AnyValue(errors.New("ouch")), attr.NewStringValue("ouch"), true},
		"error":                  {slog.AnyValue(circular), nil, false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			value, ok := attr.ValToStruct(tc.attr)
			assert.Equal(t, tc.ok, ok)
			if tc.ok {
				assert.Equal(t, tc.value, value)
			}
		})
	}
}

type mapper func(a slog.Attr) slog.Attr

func removeMapper(_ slog.Attr) slog.Attr {
	return slog.Attr{}
}

func genReplace(r slog.Attr, groups ...string) attr.Mapper {
	return func(g []string, a slog.Attr) slog.Attr {
		if reflect.DeepEqual(groups, g) {
			return r
		}
		return a
	}
}

func genMapper(mapper mapper, groups []string, keys ...string) attr.Mapper {
	return func(g []string, a slog.Attr) slog.Attr {
		for _, key := range keys {
			if reflect.DeepEqual(groups, g) && a.Key == key {
				return mapper(a)
			}
		}
		return a
	}
}

func groups(groups ...string) []string {
	return groups
}

func TestWrapAttrMapper(t *testing.T) {
	tests := map[string]struct {
		groups   []string
		attr     slog.Attr
		mapper   attr.Mapper
		expected slog.Attr
	}{
		"simple replacement":  {nil, slog.Int("a", 1), genReplace(slog.Int("b", 2)), slog.Int("b", 2)},
		"inside group":        {groups("g", "h"), slog.Int("a", 1), genReplace(slog.Int("b", 2), "g", "h"), slog.Int("b", 2)},
		"with group":          {groups("g"), slog.Group("h", slog.Int("a", 1)), genReplace(slog.Int("b", 2), "g", "h"), slog.Group("h", slog.Int("b", 2))},
		"group replace":       {groups("g"), slog.Group("h", slog.Int("a", 1), slog.Int("b", 2)), genMapper(removeMapper, groups("g", "h"), "a"), slog.Group("h", slog.Int("b", 2))},
		"group replace empty": {groups("g"), slog.Group("h", slog.Int("a", 1)), genReplace(slog.Attr{}, "g", "h"), slog.Attr{}},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			m := attr.WrapAttrMapper(tc.mapper)
			actual := m(tc.groups, tc.attr)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestWrapAttrMapper_nil(t *testing.T) {
	assert.Nil(t, attr.WrapAttrMapper(nil))
}

const rfc3339Millis = "2006-01-02T15:04:05.000Z07:00"

func TestWriteTimeRFC3339(t *testing.T) {
	for _, tm := range []time.Time{
		time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC),
		time.Date(2000, 1, 2, 3, 4, 5, 400, time.Local),
		time.Date(2000, 11, 12, 3, 4, 500, 5e7, time.UTC),
	} {
		got := attr.TimeToRFC3339InMs(tm)
		want := `"` + tm.Format(rfc3339Millis) + `"`
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	}
}

func BenchmarkWriteTime(b *testing.B) {
	tm := time.Date(2022, 3, 4, 5, 6, 7, 823456789, time.Local)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		attr.TimeToRFC3339InMs(tm)
	}
}
