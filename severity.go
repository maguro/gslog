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

	"cloud.google.com/go/logging"
)

const (
	// LevelNotice means normal but significant events, such as start up,
	// shut down, or configuration.
	LevelNotice = slog.Level(2)
	// LevelCritical means events that cause more severe problems or brief
	// outages.
	LevelCritical = slog.Level(12)
	// LevelAlert means a person must take an action immediately.
	LevelAlert = slog.Level(16)
	// LevelEmergency means one or more systems are unusable.
	LevelEmergency = slog.Level(20)
)

// levelToSeverity converts slog.Level logging levels to logging.Severity.
func levelToSeverity(level slog.Level) logging.Severity {
	severity := logging.Severity((int(level) + 8) / 4 * 100)
	if slog.LevelInfo < level {
		return severity + 100
	}
	return severity
}
