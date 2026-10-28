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
	"bytes"
	"log/slog"
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"m4o.io/gslog"
	"m4o.io/gslog/internal/options"
)

const (
	naString          = ""
	envVarLogLevelKey = "TEST_ENV_VAR"
	levelUnknown      = slog.Level(math.MaxInt)
)

func TestLogLevel(t *testing.T) {
	tests := map[string]struct {
		explicitLogLevel slog.Level
		defaultLogLevel  slog.Level
		envVar           bool
		envVarKey        string
		envVarValue      string
		expected         slog.Level
	}{
		"do nothing":                 {levelUnknown, levelUnknown, false, naString, naString, slog.LevelInfo},
		"default":                    {levelUnknown, slog.LevelInfo, false, naString, naString, slog.LevelInfo},
		"default missing env var":    {levelUnknown, slog.LevelInfo, true, naString, naString, slog.LevelInfo},
		"explicit":                   {slog.LevelInfo, levelUnknown, false, naString, naString, slog.LevelInfo},
		"explicit overrides env var": {slog.LevelInfo, levelUnknown, true, envVarLogLevelKey, "INFO", slog.LevelInfo},
		"explicit overrides default": {slog.LevelInfo, slog.LevelDebug, false, naString, naString, slog.LevelInfo},
		"explicit overrides all":     {slog.LevelInfo, slog.LevelDebug, true, envVarLogLevelKey, "ERROR", slog.LevelInfo},
		"env var garbage":            {levelUnknown, levelUnknown, true, envVarLogLevelKey, "OUCH", slog.LevelInfo},
		"garbage keeps default":      {levelUnknown, slog.LevelDebug, true, envVarLogLevelKey, "OUCH", slog.LevelDebug},
		"env var lower case":         {levelUnknown, levelUnknown, true, envVarLogLevelKey, "debug", slog.LevelDebug},
		"env var offset":             {levelUnknown, levelUnknown, true, envVarLogLevelKey, "WARN+2", slog.Level(6)},
		"env var NOTICE":             {levelUnknown, levelUnknown, true, envVarLogLevelKey, "NOTICE", gslog.LevelNotice},
		"env var DEBUG":              {levelUnknown, levelUnknown, true, envVarLogLevelKey, "DEBUG", slog.LevelDebug},
		"env var INFO":               {levelUnknown, levelUnknown, true, envVarLogLevelKey, "INFO", slog.LevelInfo},
		"env var WARN":               {levelUnknown, levelUnknown, true, envVarLogLevelKey, "WARN", slog.LevelWarn},
		"env var ERROR":              {levelUnknown, levelUnknown, true, envVarLogLevelKey, "ERROR", slog.LevelError},
		"env var missing":            {levelUnknown, levelUnknown, true, naString, naString, slog.LevelInfo},
		"env var overrides default":  {levelUnknown, slog.LevelDebug, true, envVarLogLevelKey, "INFO", slog.LevelInfo},
		"env var high custom level":  {levelUnknown, levelUnknown, true, envVarLogLevelKey, "32", slog.Level(32)},
		"env var low custom level":   {levelUnknown, levelUnknown, true, envVarLogLevelKey, "-8", slog.Level(-8)},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var opts []gslog.Option
			if tc.explicitLogLevel != levelUnknown {
				opts = append(opts, gslog.WithLeveler(tc.explicitLogLevel))
			}
			if tc.defaultLogLevel != levelUnknown {
				opts = append(opts, gslog.WithDefaultLeveler(tc.defaultLogLevel))
			}
			if tc.envVar {
				if tc.envVarKey != "" {
					assert.NoError(t, os.Setenv(tc.envVarKey, tc.envVarValue))
					defer func() {
						assert.NoError(t, os.Unsetenv(envVarLogLevelKey))
					}()
				}
				opts = append(opts, gslog.WithLogLevelFromEnvVar(envVarLogLevelKey))
			}

			o := options.ApplyOptions(opts...)
			assert.Equal(t, tc.expected, o.Level)
		})
	}
}

func TestWithLogLevelFromEnvVar(t *testing.T) {
	defer func() {
		if x := recover(); x == nil {
			t.Error("expected panic")
		}
	}()
	gslog.WithLogLevelFromEnvVar("")
}

func TestWithLogLevelFromEnvVar_invalidValueWarns(t *testing.T) {
	var buf bytes.Buffer

	previous := slog.Default()
	defer slog.SetDefault(previous)

	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Setenv(envVarLogLevelKey, "DEUBG")

	o := options.ApplyOptions(
		gslog.WithDefaultLeveler(slog.LevelWarn),
		gslog.WithLogLevelFromEnvVar(envVarLogLevelKey))

	assert.Equal(t, slog.LevelWarn, o.Level)
	assert.Contains(t, buf.String(), `level=WARN msg="Invalid log level" key=TEST_ENV_VAR value=DEUBG`)
}

func TestWithSourceAdded(t *testing.T) {
	o := options.ApplyOptions(gslog.WithSourceAdded(), gslog.WithDefaultLeveler(slog.LevelInfo))
	assert.True(t, o.AddSource)
}

func TestWithReplaceAttr(t *testing.T) {
	s := slog.String("foo", "bar")
	var ra gslog.AttrMapper = func(groups []string, a slog.Attr) slog.Attr {
		return s
	}

	o := options.ApplyOptions(gslog.WithReplaceAttr(ra), gslog.WithDefaultLeveler(slog.LevelInfo))
	assert.Equal(t, s, o.ReplaceAttr(nil, slog.String("unused", "string")))
}
