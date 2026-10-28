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
