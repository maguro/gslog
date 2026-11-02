// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package gslog_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"m4o.io/gslog"
	"m4o.io/gslog/internal/level"
)

// TestLevels verifies that the exported levels have the values of the
// internal level package, which cannot import this package.
func TestLevels(t *testing.T) {
	assert.Equal(t, level.LevelNotice, gslog.LevelNotice)
	assert.Equal(t, level.LevelCritical, gslog.LevelCritical)
	assert.Equal(t, level.LevelAlert, gslog.LevelAlert)
	assert.Equal(t, level.LevelEmergency, gslog.LevelEmergency)
}
