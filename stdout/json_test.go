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

package stdout

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
