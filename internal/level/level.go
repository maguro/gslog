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

// Package level contains code that maps slog.Level levels to logging.Severity.
package level

import (
	"log/slog"

	"cloud.google.com/go/logging"
)

const (
	severityIntercept = 8
	severitySlope     = 4
	severityIncrement = 100
)

// ToSeverity converts slog.Level logging levels to logging.Severity.
func ToSeverity(level slog.Level) logging.Severity {
	severity := logging.Severity((int(level) + severityIntercept) / severitySlope * severityIncrement)
	if slog.LevelInfo < level {
		return severity + severityIncrement
	}

	return severity
}
