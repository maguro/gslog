// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package entry_test

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"m4o.io/gslog/internal/entry"
)

func TestReporter_For(t *testing.T) {
	want := entry.ErrorReport{Service: "svc", Version: "v1", StackTrace: "stack"}

	var calls int

	reporter := &entry.Reporter{
		MinLevel: slog.LevelError,
		Report: func() entry.ErrorReport {
			calls++

			return want
		},
	}

	for name, tt := range map[string]struct {
		reporter *entry.Reporter
		level    slog.Level
		want     entry.ErrorReport
		ok       bool
	}{
		"nil reporter":    {nil, slog.LevelError, entry.ErrorReport{}, false},
		"below MinLevel":  {reporter, slog.LevelWarn, entry.ErrorReport{}, false},
		"at MinLevel":     {reporter, slog.LevelError, want, true},
		"above MinLevel":  {reporter, slog.LevelError + 4, want, true},
		"far below level": {reporter, slog.LevelDebug, entry.ErrorReport{}, false},
	} {
		t.Run(name, func(t *testing.T) {
			got, ok := tt.reporter.For(tt.level)

			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}

	assert.Equal(t, 2, calls)
}
