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
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"m4o.io/gslog"
	"m4o.io/gslog/internal/options"
)

const envVarLogLevelKey = "TEST_ENV_VAR"

func TestWithLogLeveler(t *testing.T) {
	option := gslog.WithLogLeveler(slog.LevelWarn)

	o := options.ApplyOptions(option)

	assert.Equal(t, slog.LevelWarn, o.ExplicitLogLevel)
}

func TestWithLogLevelFromEnvVar(t *testing.T) {
	t.Setenv(envVarLogLevelKey, "ERROR")
	option := gslog.WithLogLevelFromEnvVar(envVarLogLevelKey)

	o := options.ApplyOptions(option)

	assert.Equal(t, slog.LevelError, o.Level)
}

func TestWithDefaultLogLeveler(t *testing.T) {
	option := gslog.WithDefaultLogLeveler(slog.LevelDebug)

	o := options.ApplyOptions(option)

	assert.Equal(t, slog.LevelDebug, o.DefaultLogLevel)
}

func TestWithSourceAdded(t *testing.T) {
	option := gslog.WithSourceAdded()

	o := options.ApplyOptions(option)

	assert.True(t, o.AddSource)
}

func TestWithReplaceAttr(t *testing.T) {
	replaced := slog.String("foo", "bar")
	var mapper gslog.AttrMapper = func(_ []string, _ slog.Attr) slog.Attr {
		return replaced
	}
	option := gslog.WithReplaceAttr(mapper)

	o := options.ApplyOptions(option)

	unused := slog.String("unused", "string")
	actual := o.ReplaceAttr(nil, unused)

	assert.Equal(t, replaced, actual)
}
