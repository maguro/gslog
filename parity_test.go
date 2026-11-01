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

package gslog_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"testing"
	"time"

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

// TestValueParity checks that the two handlers encode an attribute value of
// each kind the same way.  The test decodes the JSON line of the stdout
// handler and the payload of the gcp handler to Go values, and compares both
// with the expected value.
func TestValueParity(t *testing.T) {
	const nanError = "!ERROR:json: unsupported value: NaN"

	ts := time.Date(2000, 1, 2, 3, 4, 5, 123456789, time.FixedZone("CET", 3600))

	tests := map[string]struct {
		attr slog.Attr
		want any
	}{
		"string":      {slog.String("v", "hello"), "hello"},
		"bad UTF-8":   {slog.String("v", "a\xff\xfeb"), "a��b"},
		"int":         {slog.Int("v", -3), -3.0},
		"max int64":   {slog.Int64("v", math.MaxInt64), float64(math.MaxInt64)},
		"max uint64":  {slog.Uint64("v", math.MaxUint64), float64(math.MaxUint64)},
		"float":       {slog.Float64("v", 2.5), 2.5},
		"large float": {slog.Float64("v", 1e21), 1e21},
		"small float": {slog.Float64("v", 1e-7), 1e-7},
		"NaN":         {slog.Float64("v", math.NaN()), "NaN"},
		"+Inf":        {slog.Float64("v", math.Inf(1)), "Infinity"},
		"-Inf":        {slog.Float64("v", math.Inf(-1)), "-Infinity"},
		"bool":        {slog.Bool("v", true), true},
		"duration":    {slog.Duration("v", 1500*time.Millisecond), 1.5e9},
		"time":        {slog.Time("v", ts), "2000-01-02T03:04:05.123456789+01:00"},
		"zero time":   {slog.Time("v", time.Time{}), "0001-01-01T00:00:00Z"},
		"group":       {slog.Group("v", slog.Int("n", 1)), map[string]any{"n": 1.0}},
		"empty group": {slog.Group("v", slog.Attr{}), nil},

		"nil":                    {slog.Any("v", nil), nil},
		"error":                  {slog.Any("v", errors.New("ouch")), "ouch"},
		"error with MarshalJSON": {slog.Any("v", chimera{}), map[string]any{"c": 1.0}},
		"struct":                 {slog.Any("v", point{1, 2}), map[string]any{"X": 1.0, "Y": 2.0}},
		"nil pointer":            {slog.Any("v", (*point)(nil)), nil},
		"map of any":             {slog.Any("v", map[string]any{"a": 1}), map[string]any{"a": 1.0}},
		"map of int":             {slog.Any("v", map[string]int{"a": 1}), map[string]any{"a": 1.0}},
		"nil map of any":         {slog.Any("v", map[string]any(nil)), nil},
		"slice of any":           {slog.Any("v", []any{1, "a", true}), []any{1.0, "a", true}},
		"slice of string":        {slog.Any("v", []string{"a", "b"}), []any{"a", "b"}},
		"nil slice of any":       {slog.Any("v", []any(nil)), nil},
		"nil slice in a map":     {slog.Any("v", map[string]any{"a": []any(nil)}), map[string]any{"a": nil}},
		"nil map in a slice":     {slog.Any("v", []any{map[string]any(nil)}), []any{nil}},
		"float32 in a map":       {slog.Any("v", map[string]any{"f": float32(0.1)}), map[string]any{"f": 0.1}},
		"float64 in a map":       {slog.Any("v", map[string]any{"f": 2.5}), map[string]any{"f": 2.5}},
		"nil in a map":           {slog.Any("v", map[string]any{"a": nil}), map[string]any{"a": nil}},
		"NaN in a map":           {slog.Any("v", map[string]any{"f": math.NaN()}), nanError},
		"float32 NaN in a map":   {slog.Any("v", map[string]any{"f": float32(math.NaN())}), nanError},
		"bad UTF-8 key in a map": {slog.Any("v", map[string]any{"k\xff": 1}), map[string]any{"k�": 1.0}},
		"bad UTF-8 in a map":     {slog.Any("v", map[string]any{"s": "a\xffb"}), map[string]any{"s": "a�b"}},
		"struct in a slice":      {slog.Any("v", []any{point{1, 2}}), []any{map[string]any{"X": 1.0, "Y": 2.0}}},
		"integers in a map": {
			slog.Any("v", map[string]any{
				"i8": int8(-1), "i16": int16(-2), "i32": int32(-3), "i64": int64(-4),
				"u": uint(5), "u8": uint8(6), "u16": uint16(7), "u32": uint32(8), "u64": uint64(9),
			}),
			map[string]any{
				"i8": -1.0, "i16": -2.0, "i32": -3.0, "i64": -4.0,
				"u": 5.0, "u8": 6.0, "u16": 7.0, "u32": 8.0, "u64": 9.0,
			},
		},
		"bytes":       {slog.Any("v", []byte{1, 2, 3}), "AQID"},
		"nil bytes":   {slog.Any("v", []byte(nil)), nil},
		"raw JSON":    {slog.Any("v", json.RawMessage(`{"n":1}`)), map[string]any{"n": 1.0}},
		"JSON number": {slog.Any("v", json.Number("12")), 12.0},
		"channel":     {slog.Any("v", make(chan int)), "!ERROR:json: unsupported type: chan int"},
		"func":        {slog.Any("v", func() {}), "!ERROR:json: unsupported type: func()"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fromStdout := stdoutValue(t, tc.attr)
			fromGcp := gcpValue(t, tc.attr)

			assert.Equal(t, tc.want, fromStdout, "stdout")
			assert.Equal(t, tc.want, fromGcp, "gcp")
		})
	}
}

// stdoutValue returns the decoded value of the attribute after the stdout
// handler writes it.
func stdoutValue(t *testing.T, attr slog.Attr) any {
	t.Helper()

	var buf bytes.Buffer

	h := stdout.NewHandler(&buf)
	l := slog.New(h)
	l.Info("m", attr)

	var line map[string]any

	err := json.Unmarshal(buf.Bytes(), &line)
	require.NoError(t, err)

	return line[attr.Key]
}

// gcpValue returns the value of the attribute in the payload after the gcp
// handler logs it.
func gcpValue(t *testing.T, attr slog.Attr) any {
	t.Helper()

	var got logging.Entry

	logger := gcp.LoggerFunc(func(e logging.Entry) {
		got = e
	})
	h := gcp.NewHandler(logger)
	l := slog.New(h)
	l.Info("m", attr)

	payload, ok := got.Payload.(*spb.Struct)
	require.True(t, ok)

	return payload.AsMap()[attr.Key]
}
