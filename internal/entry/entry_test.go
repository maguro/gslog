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
	ctx := entry.WithLabels(context.Background(), labels)

	augmentors := []entry.Augmentor{
		func(_ context.Context, e *entry.Entry) {
			e.Trace = "first"
			e.Labels = map[string]string{"augmentor": "a"}
		},
		func(_ context.Context, e *entry.Entry) {
			e.Trace = "second"
		},
	}

	e := entry.Fill(ctx, &record, true, augmentors)

	require.NotNil(t, e.Source)
	assert.Contains(t, e.Source.Function, "TestFill")
	assert.Equal(t, "second", e.Trace)
	assert.Equal(t, map[string]string{"augmentor": "a", "ctx": "c"}, e.Labels)
	assert.Equal(t, map[string]string{"ctx": "c"}, labels)
}

func TestFill_noSource(t *testing.T) {
	record := slog.NewRecord(time.Time{}, slog.LevelInfo, "message", 0)

	e := entry.Fill(context.Background(), &record, false, nil)

	assert.Nil(t, e.Source)
	assert.Nil(t, e.Labels)
}

func TestAddLabels(t *testing.T) {
	labels := map[string]string{"ctx": "c"}
	ctx := entry.WithLabels(context.Background(), labels)

	var e entry.Entry

	entry.AddLabels(ctx, &e)

	assert.Equal(t, labels, e.Labels)
	assert.Equal(t, labels, entry.LabelsFrom(ctx))
}

func TestLabelsFrom_noLabels(t *testing.T) {
	assert.Nil(t, entry.LabelsFrom(context.Background()))
}
