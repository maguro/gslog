// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package gslog

import "log/slog"

// The levels that Cloud Logging has and that slog does not have.  The
// internal level package holds the same values.
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
