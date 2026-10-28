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

package jbuf

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingMarshaler is a json.Marshaler that returns an error.
type failingMarshaler struct{}

func (failingMarshaler) MarshalJSON() ([]byte, error) {
	return nil, errors.New("no JSON form")
}

func TestRemoveMember(t *testing.T) {
	tests := map[string]struct {
		members string
		key     string
		want    string
	}{
		"first":              {`"a":1,"b":2`, "a", `"b":2`},
		"middle":             {`"a":1,"b":2,"c":3`, "b", `"a":1,"c":3`},
		"last":               {`"a":1,"b":2`, "b", `"a":1`},
		"only":               {`"a":1`, "a", ``},
		"absent":             {`"a":1,"b":2`, "c", `"a":1,"b":2`},
		"prefix of a key":    {`"ab":1,"a":2`, "a", `"ab":1`},
		"object value":       {`"a":{"x":1,"y":{"z":[1,2]}},"b":2`, "a", `"b":2`},
		"array value":        {`"a":[1,{"x":"}"}],"b":2`, "a", `"b":2`},
		"string with escape": {`"a":"x\",y","b":2`, "a", `"b":2`},
		"string with comma":  {`"a":"x,y","b":2`, "a", `"b":2`},
		"repeated key":       {`"a":1,"b":2,"a":3`, "a", `"b":2`},
		"empty":              {``, "a", ``},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			members := []byte(tc.members)

			got := RemoveMember(members, tc.key)

			assert.Equal(t, tc.want, string(got))
			assert.Equal(t, tc.members, string(members), "RemoveMember must not change its argument")
		})
	}
}

func TestWriter_AppendAnyAttr(t *testing.T) {
	type point struct {
		X, Y int
	}

	_, marshalErr := json.Marshal(failingMarshaler{})
	require.Error(t, marshalErr)

	marshalText := appendString(nil, "!ERROR:"+marshalErr.Error())

	for name, tt := range map[string]struct {
		value any
		want  string
	}{
		"nil":               {nil, `"k":null`},
		"error":             {errors.New("ouch"), `"k":"ouch"`},
		"struct":            {point{1, 2}, `"k":{"X":1,"Y":2}`},
		"channel":           {make(chan int), `"k":"!ERROR:json: unsupported type: chan int"`},
		"failing marshaler": {failingMarshaler{}, `"k":` + string(marshalText)},
	} {
		t.Run(name, func(t *testing.T) {
			w := NewWriter(nil)

			w.AppendAnyAttr("k", tt.value)

			got := string(w.Buf())
			assert.Equal(t, tt.want, got)
		})
	}
}
