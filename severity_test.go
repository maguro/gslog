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

package gslog

import (
	"log/slog"
	"testing"

	"cloud.google.com/go/logging"
	"github.com/stretchr/testify/assert"
)

func TestLevelToSeverity(t *testing.T) {
	tests := map[string]struct {
		level    slog.Level
		expected logging.Severity
	}{
		"trace":     {slog.Level(-8), logging.Severity(0)},
		"debug":     {slog.LevelDebug, logging.Debug},
		"info":      {slog.LevelInfo, logging.Info},
		"notice":    {LevelNotice, logging.Notice},
		"warn":      {slog.LevelWarn, logging.Warning},
		"error":     {slog.LevelError, logging.Error},
		"critical":  {LevelCritical, logging.Critical},
		"alert":     {LevelAlert, logging.Alert},
		"emergency": {LevelEmergency, logging.Emergency},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			assert.Equal(t, tc.expected, levelToSeverity(tc.level))
		})
	}
}
