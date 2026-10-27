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

// Package level contains code that maps slog.Level levels to Cloud Logging
// severities.
package level

import (
	"log/slog"
	"strconv"
	"strings"
)

// Severity is a Cloud Logging severity.  The values are the values of the
// LogSeverity enum of the Cloud Logging API.
type Severity int

// The Cloud Logging severities.
const (
	Default   Severity = 0
	Debug     Severity = 100
	Info      Severity = 200
	Notice    Severity = 300
	Warning   Severity = 400
	Error     Severity = 500
	Critical  Severity = 600
	Alert     Severity = 700
	Emergency Severity = 800
)

// The levels that Cloud Logging has and slog does not.
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

const (
	severityIntercept = 8
	severitySlope     = 4
	severityIncrement = 100
)

// ToSeverity clamps a level to the range lowestLevel to highestLevel.
const (
	lowestLevel  = slog.Level(-severityIntercept)
	highestLevel = LevelEmergency
)

// cloudLevels maps the Cloud Logging names that slog does not have to their
// levels.
var cloudLevels = newCloudLevels(
	slog.LevelWarn, LevelNotice, LevelCritical, LevelAlert, LevelEmergency)

// ToSeverity converts a slog.Level to a Severity.  The result is in the
// range Default to Emergency.
func ToSeverity(level slog.Level) Severity {
	level = min(max(level, lowestLevel), highestLevel)

	severity := Severity((int(level) + severityIntercept) / severitySlope * severityIncrement)
	if slog.LevelInfo < level {
		return severity + severityIncrement
	}

	return severity
}

// String returns the name of the severity as the Cloud Logging API spells
// it.  An unknown severity is its number.
func (s Severity) String() string {
	switch s {
	case Default:
		return "DEFAULT"
	case Debug:
		return "DEBUG"
	case Info:
		return "INFO"
	case Notice:
		return "NOTICE"
	case Warning:
		return "WARNING"
	case Error:
		return "ERROR"
	case Critical:
		return "CRITICAL"
	case Alert:
		return "ALERT"
	case Emergency:
		return "EMERGENCY"
	default:
		return strconv.Itoa(int(s))
	}
}

// Parse returns the level that s names.  The value s is an integer
// slog.Level, such as -4 for Debug, or a name with an optional offset, such
// as "INFO+2".  The names are the slog names and the Cloud Logging names
// "WARNING", "NOTICE", "CRITICAL", "ALERT", and "EMERGENCY".  Parse ignores
// the case of a name, and space at the start and end of s.  Parse returns
// false if s does not name a level.
func Parse(s string) (slog.Level, bool) {
	s = strings.TrimSpace(s)

	n, err := strconv.Atoi(s)
	if err == nil {
		return slog.Level(n), true
	}

	name, offset := s, 0
	if i := strings.IndexAny(s, "+-"); i > 0 {
		name = s[:i]

		offset, err = strconv.Atoi(s[i:])
		if err != nil {
			return 0, false
		}
	}

	base, ok := baseLevel(name)
	if !ok {
		return 0, false
	}

	return base + slog.Level(offset), true
}

// baseLevel returns the level that name names, with no offset.
func baseLevel(name string) (slog.Level, bool) {
	upper := strings.ToUpper(name)

	lvl, ok := cloudLevels[upper]
	if ok {
		return lvl, true
	}

	err := lvl.UnmarshalText([]byte(upper))
	if err != nil {
		return 0, false
	}

	return lvl, true
}

// newCloudLevels returns a map from the severity name of each level to the
// level.
func newCloudLevels(levels ...slog.Level) map[string]slog.Level {
	names := make(map[string]slog.Level, len(levels))

	for _, lvl := range levels {
		severity := ToSeverity(lvl)
		names[severity.String()] = lvl
	}

	return names
}
