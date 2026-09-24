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

package attr_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	return []byte(fmt.Sprintf(`{"name":%q}`, u.Name)), nil
}

// Error panics.  NewAny must not call Error on a type that implements
// json.Marshaler.
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

	uJson = map[string]any{
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

	uGroup = append(uGroup,
		slog.String("id", "user-12234"),
		slog.String("first_name", "Jan"),
		slog.String("last_name", "Doe"),
		slog.String("email", "jan@example.com"),
		slog.Any("password", pw),
		slog.Uint64("age", 32),
		slog.Float64("height", 5.91),
		slog.Bool("engineer", true),
		slog.Any("manager", nil),
	)
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
		err   bool
	}{
		"nil":        {nil, attr.NewNilValue(), false},
		"not simple": {u, uStruct, false},
		"error":      {circular, nil, true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			value, err := attr.AsJSON(tc.attr)
			if tc.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.value, value)
			}
		})
	}
}

func TestValToStruct(t *testing.T) {
	now := time.Now().UTC()

	_, cycleErr := json.Marshal(circular)
	require.Error(t, cycleErr)

	cycleText := "!ERROR:" + cycleErr.Error()

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
		"any with no JSON form":  {slog.AnyValue(circular), attr.NewStringValue(cycleText), true},
		"any channel":            {slog.AnyValue(make(chan int)), attr.NewStringValue("!ERROR:json: unsupported type: chan int"), true},
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
