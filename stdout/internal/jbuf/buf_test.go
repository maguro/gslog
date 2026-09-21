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
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendString(t *testing.T) {
	for _, s := range []string{
		"",
		"plain",
		`quote " and backslash \`,
		"newline\n tab\t return\r",
		"control \x01\x1f",
		"unicode é 日本 \U0001F600",
		"html <>&",
		"bad utf8 \xff\xfe end",
	} {
		t.Run(s, func(t *testing.T) {
			got := appendString(nil, s)

			var decoded string

			require.NoError(t, json.Unmarshal(got, &decoded))

			want, err := json.Marshal(s)
			require.NoError(t, err)

			var wantDecoded string

			require.NoError(t, json.Unmarshal(want, &wantDecoded))
			assert.Equal(t, wantDecoded, decoded)
		})
	}
}

func TestAppendNumber(t *testing.T) {
	for _, f := range []float64{0, 1, -1, 42, 3.5, 0.1, 1e20, 1e21, 123456789012345678, 1e-6, 1e-7, 2.5e-9, -1e300, math.MaxFloat64} {
		want, err := json.Marshal(f)
		require.NoError(t, err)
		assert.Equal(t, string(want), string(appendNumber(nil, f)))
	}

	assert.Equal(t, `"NaN"`, string(appendNumber(nil, math.NaN())))
	assert.Equal(t, `"Infinity"`, string(appendNumber(nil, math.Inf(1))))
	assert.Equal(t, `"-Infinity"`, string(appendNumber(nil, math.Inf(-1))))
}

func TestTrimExponent(t *testing.T) {
	for _, tt := range []struct {
		name  string
		buf   string
		start int
		want  string
	}{
		{name: "buffer shorter than an exponent", buf: "1e9", start: 0, want: "1e9"},
		{name: "number shorter than an exponent", buf: "prefix1e9", start: 6, want: "prefix1e9"},
		{name: "leading zero removed", buf: "2.5e-09", start: 0, want: "2.5e-9"},
		{name: "leading zero removed after a prefix", buf: `{"a":1e-09`, start: 5, want: `{"a":1e-9`},
		{name: "two-digit exponent kept", buf: "1e-10", start: 0, want: "1e-10"},
		{name: "positive exponent kept", buf: "1e+21", start: 0, want: "1e+21"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := trimExponent([]byte(tt.buf), tt.start)

			assert.Equal(t, tt.want, string(got))
		})
	}
}
