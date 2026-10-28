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

package gslog_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"cloud.google.com/go/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	spb "google.golang.org/protobuf/types/known/structpb"

	"m4o.io/gslog/gcp"
	"m4o.io/gslog/stdout"
)

// point is a struct that encoding/json encodes as an object.
type point struct {
	X int
	Y int
}

// chimera implements json.Marshaler and error.  Error panics.  A handler
// that uses the error text instead of the JSON fails the test.
type chimera struct{}

func (chimera) MarshalJSON() ([]byte, error) {
	return []byte(`{"c":1}`), nil
}

func (chimera) Error() string {
	panic("Error called on a json.Marshaler")
}

// TestAnyValueParity checks that the two handlers encode a value of kind
// slog.KindAny the same way.  The test decodes the JSON line of the stdout
// handler and the payload of the gcp handler to Go values, and compares both
// with the expected value.
func TestAnyValueParity(t *testing.T) {
	tests := map[string]struct {
		value any
		want  any
	}{
		"nil":                    {nil, nil},
		"error":                  {errors.New("ouch"), "ouch"},
		"error with MarshalJSON": {chimera{}, map[string]any{"c": 1.0}},
		"struct":                 {point{1, 2}, map[string]any{"X": 1.0, "Y": 2.0}},
		"nil pointer":            {(*point)(nil), nil},
		"map of any":             {map[string]any{"a": 1}, map[string]any{"a": 1.0}},
		"map of int":             {map[string]int{"a": 1}, map[string]any{"a": 1.0}},
		"slice of any":           {[]any{1, "a", true}, []any{1.0, "a", true}},
		"slice of string":        {[]string{"a", "b"}, []any{"a", "b"}},
		"bytes":                  {[]byte{1, 2, 3}, "AQID"},
		"raw JSON":               {json.RawMessage(`{"n":1}`), map[string]any{"n": 1.0}},
		"JSON number":            {json.Number("12"), 12.0},
		"channel":                {make(chan int), "!ERROR:json: unsupported type: chan int"},
		"func":                   {func() {}, "!ERROR:json: unsupported type: func()"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fromStdout := stdoutAny(t, tc.value)
			fromGcp := gcpAny(t, tc.value)

			assert.Equal(t, tc.want, fromStdout, "stdout")
			assert.Equal(t, tc.want, fromGcp, "gcp")
		})
	}
}

// stdoutAny returns the decoded value of the attribute "v" after the stdout
// handler writes slog.Any("v", value).
func stdoutAny(t *testing.T, value any) any {
	t.Helper()

	var buf bytes.Buffer

	h := stdout.NewHandler(&buf)
	l := slog.New(h)
	l.Info("m", slog.Any("v", value))

	var line map[string]any

	err := json.Unmarshal(buf.Bytes(), &line)
	require.NoError(t, err)

	return line["v"]
}

// gcpAny returns the value of the attribute "v" in the payload after the gcp
// handler logs slog.Any("v", value).
func gcpAny(t *testing.T, value any) any {
	t.Helper()

	var got logging.Entry

	logger := gcp.LoggerFunc(func(e logging.Entry) {
		got = e
	})
	h := gcp.NewHandler(logger)
	l := slog.New(h)
	l.Info("m", slog.Any("v", value))

	payload, ok := got.Payload.(*spb.Struct)
	require.True(t, ok)

	return payload.AsMap()["v"]
}
