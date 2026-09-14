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

package level_test

import (
	"log/slog"
	"math"
	"testing"

	"cloud.google.com/go/logging"
	"github.com/stretchr/testify/assert"

	"m4o.io/gslog/internal/level"
)

// TestToSeverity verifies the mapping from slog levels to severities against
// the constants of the Cloud Logging client.
func TestToSeverity(t *testing.T) {
	tests := map[string]struct {
		level slog.Level
		want  logging.Severity
	}{
		"trace":           {slog.Level(-8), logging.Default},
		"debug":           {slog.LevelDebug, logging.Debug},
		"info":            {slog.LevelInfo, logging.Info},
		"notice":          {level.LevelNotice, logging.Notice},
		"warn":            {slog.LevelWarn, logging.Warning},
		"error":           {slog.LevelError, logging.Error},
		"critical":        {level.LevelCritical, logging.Critical},
		"alert":           {level.LevelAlert, logging.Alert},
		"emergency":       {level.LevelEmergency, logging.Emergency},
		"below the range": {slog.Level(-12), logging.Default},
		"above the range": {slog.Level(24), logging.Emergency},
		"lowest level":    {slog.Level(math.MinInt), logging.Default},
		"highest level":   {slog.Level(math.MaxInt), logging.Emergency},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := level.ToSeverity(tc.level)

			assert.Equal(t, level.Severity(tc.want), got)
		})
	}
}

// TestSeverities verifies that each severity has the value and the name of
// the same severity in the Cloud Logging API.
func TestSeverities(t *testing.T) {
	tests := []struct {
		severity level.Severity
		want     logging.Severity
		name     string
	}{
		{level.Default, logging.Default, "DEFAULT"},
		{level.Debug, logging.Debug, "DEBUG"},
		{level.Info, logging.Info, "INFO"},
		{level.Notice, logging.Notice, "NOTICE"},
		{level.Warning, logging.Warning, "WARNING"},
		{level.Error, logging.Error, "ERROR"},
		{level.Critical, logging.Critical, "CRITICAL"},
		{level.Alert, logging.Alert, "ALERT"},
		{level.Emergency, logging.Emergency, "EMERGENCY"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, int(tc.want), int(tc.severity))
			assert.Equal(t, tc.name, tc.severity.String())
		})
	}
}

// TestSeverity_String_unknown verifies that an unknown severity is its
// number.
func TestSeverity_String_unknown(t *testing.T) {
	assert.Equal(t, "250", level.Severity(250).String())
}
