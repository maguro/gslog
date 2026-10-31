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

package entry_test

import (
	"context"
	"log/slog"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"m4o.io/gslog/internal/entry"
)

func TestFill(t *testing.T) {
	pc, _, _, ok := runtime.Caller(0)
	require.True(t, ok)

	record := slog.NewRecord(time.Time{}, slog.LevelInfo, "message", pc)

	labels := map[string]string{"ctx": "c"}
	stored := entry.NewLabels(labels)
	ctx := entry.WithLabels(context.Background(), stored)

	augmentors := []entry.Augmentor{
		func(_ context.Context, e *entry.Entry) {
			e.TracePrefix = "first"
			e.Labels = entry.NewLabels(map[string]string{"augmentor": "a"})
		},
		func(_ context.Context, e *entry.Entry) {
			e.TracePrefix = "second"
		},
	}

	var e entry.Entry

	entry.Fill(ctx, &record, true, augmentors, &e)

	require.NotNil(t, e.Source)
	assert.Contains(t, e.Source.Function, "TestFill")
	assert.Equal(t, "second", e.TracePrefix)
	assert.Equal(t, map[string]string{"augmentor": "a", "ctx": "c"}, e.Labels.Map)
	assert.Equal(t, []string{"augmentor", "ctx"}, e.Labels.Keys)
	assert.Equal(t, map[string]string{"ctx": "c"}, labels)
}

func TestFill_noSource(t *testing.T) {
	record := slog.NewRecord(time.Time{}, slog.LevelInfo, "message", 0)

	var e entry.Entry

	entry.Fill(context.Background(), &record, false, nil, &e)

	assert.Nil(t, e.Source)
	assert.Zero(t, e.Labels.Len())
}

func TestRelease(t *testing.T) {
	e := entry.Alloc()
	e.TracePrefix = "trace"
	e.Attrs = append(e.Attrs, slog.String("a", "1"))

	entry.Release(e)

	e = entry.Alloc()
	defer entry.Release(e)

	assert.Empty(t, e.TracePrefix)
	assert.Empty(t, e.Attrs)
}

func TestEntry_Trace(t *testing.T) {
	e := entry.Entry{
		TracePrefix: "projects/my-project/traces/",
		TraceID:     [16]byte{0x4b, 0xf9, 0x2f, 0x35, 0x77, 0xb3, 0x4d, 0xa6, 0xa3, 0xce, 0x92, 0x9d, 0x0e, 0x0e, 0x47, 0x36},
		SpanID:      [8]byte{0x00, 0xf0, 0x67, 0xaa, 0x0b, 0xa9, 0x02, 0xb7},
	}

	assert.Equal(t, "projects/my-project/traces/4bf92f3577b34da6a3ce929d0e0e4736", e.Trace())
	assert.Equal(t, "00f067aa0ba902b7", e.SpanIDString())

	var none entry.Entry

	assert.Empty(t, none.Trace())
	assert.Empty(t, none.SpanIDString())
}

func TestAddLabels(t *testing.T) {
	labels := map[string]string{"ctx": "c"}
	stored := entry.NewLabels(labels)
	ctx := entry.WithLabels(context.Background(), stored)

	var e entry.Entry

	entry.AddLabels(ctx, &e)

	assert.Equal(t, labels, e.Labels.Map)
	assert.Equal(t, labels, entry.LabelsFrom(ctx).Map)
}

func TestLabelsFrom_noLabels(t *testing.T) {
	assert.Zero(t, entry.LabelsFrom(context.Background()))
}

func TestNewLabels(t *testing.T) {
	labels := entry.NewLabels(map[string]string{"b": "2", "a": "1", "c": "3"})

	assert.Equal(t, 3, labels.Len())
	assert.Equal(t, []string{"a", "b", "c"}, labels.Keys)

	assert.Zero(t, entry.NewLabels(nil))
	assert.Zero(t, entry.NewLabels(map[string]string{}))
}

func TestLabels_Merge(t *testing.T) {
	first := entry.NewLabels(map[string]string{"a": "1", "b": "first"})
	second := entry.NewLabels(map[string]string{"b": "second", "c": "3"})

	tests := map[string]struct {
		l, other entry.Labels
		wantMap  map[string]string
		wantKeys []string
	}{
		"both":        {first, second, map[string]string{"a": "1", "b": "second", "c": "3"}, []string{"a", "b", "c"}},
		"other empty": {first, entry.Labels{}, first.Map, first.Keys},
		"l empty":     {entry.Labels{}, second, second.Map, second.Keys},
		"both empty":  {entry.Labels{}, entry.Labels{}, nil, nil},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tc.l.Merge(tc.other)

			assert.Equal(t, tc.wantMap, got.Map)
			assert.Equal(t, tc.wantKeys, got.Keys)
		})
	}

	assert.Equal(t, map[string]string{"a": "1", "b": "first"}, first.Map, "Merge must not change its receiver")
}

func TestValidUTF8(t *testing.T) {
	tests := map[string]struct {
		s    string
		want string
	}{
		"ASCII":              {"plain", "plain"},
		"multibyte":          {"日本 é", "日本 é"},
		"empty":              {"", ""},
		"one invalid byte":   {"a\xffb", "a�b"},
		"two invalid bytes":  {"a\xff\xfeb", "a��b"},
		"truncated sequence": {"\xe6\x97", "��"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, entry.ValidUTF8(tc.s))
		})
	}
}
