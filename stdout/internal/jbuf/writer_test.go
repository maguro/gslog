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
