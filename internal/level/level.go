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

// Package level contains code that maps slog.Level levels to Cloud Logging
// severities.
package level

import (
	"log/slog"
	"strconv"
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
