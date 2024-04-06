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
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"m4o.io/gslog/internal/options"
)

const (
	naString = ""
)

func TestLogLevel(t *testing.T) {
	tests := map[string]struct {
		explicitLogLevel slog.Level
		defaultLogLevel  slog.Level
		envVar           bool
		envVarKey        string
		envVarValue      string
		err              error
		expected         slog.Level
	}{
		"do nothing":                 {LevelUnknown, LevelUnknown, false, naString, naString, ErrNoLogLevelSet, LevelUnknown},
		"default":                    {LevelUnknown, slog.LevelInfo, false, naString, naString, nil, slog.LevelInfo},
		"default missing env var":    {LevelUnknown, slog.LevelInfo, true, naString, naString, nil, slog.LevelInfo},
		"explicit":                   {slog.LevelInfo, LevelUnknown, false, naString, naString, nil, slog.LevelInfo},
		"explicit overrides env var": {slog.LevelInfo, LevelUnknown, true, envVarLogLevelKey, "INFO", nil, slog.LevelInfo},
		"explicit overrides default": {slog.LevelInfo, slog.LevelDebug, false, naString, naString, nil, slog.LevelInfo},
		"explicit overrides all":     {slog.LevelInfo, slog.LevelDebug, true, envVarLogLevelKey, "ERROR", nil, slog.LevelInfo},
		"env var garbage":            {LevelUnknown, LevelUnknown, true, envVarLogLevelKey, "OUCH", nil, slog.LevelInfo},
		"env var DEBUG":              {LevelUnknown, LevelUnknown, true, envVarLogLevelKey, "DEBUG", nil, slog.LevelDebug},
		"env var INFO":               {LevelUnknown, LevelUnknown, true, envVarLogLevelKey, "INFO", nil, slog.LevelInfo},
		"env var WARN":               {LevelUnknown, LevelUnknown, true, envVarLogLevelKey, "WARN", nil, slog.LevelWarn},
		"env var ERROR":              {LevelUnknown, LevelUnknown, true, envVarLogLevelKey, "ERROR", nil, slog.LevelError},
		"env var missing":            {LevelUnknown, LevelUnknown, true, naString, naString, ErrNoLogLevelSet, LevelUnknown},
		"env var overrides default":  {LevelUnknown, slog.LevelDebug, true, envVarLogLevelKey, "INFO", nil, slog.LevelInfo},
		"env var high custom level":  {LevelUnknown, LevelUnknown, true, envVarLogLevelKey, "32", nil, slog.Level(32)},
		"env var low custom level":   {LevelUnknown, LevelUnknown, true, envVarLogLevelKey, "-8", nil, slog.Level(-8)},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var opts []options.OptionProcessor
			if tc.explicitLogLevel != LevelUnknown {
				opts = append(opts, WithLogLevel(tc.explicitLogLevel))
			}
			if tc.defaultLogLevel != LevelUnknown {
				opts = append(opts, WithDefaultLogLevel(tc.defaultLogLevel))
			}
			if tc.envVar {
				if tc.envVarKey != "" {
					assert.NoError(t, os.Setenv(tc.envVarKey, tc.envVarValue))
					defer func() {
						assert.NoError(t, os.Unsetenv(envVarLogLevelKey))
					}()
				}
				opts = append(opts, WithLogLevelFromEnvVar())
			}

			o, err := options.ApplyOptions(opts...)
			if tc.err != nil {
				assert.Equal(t, tc.err, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, o.Level)
			}
		})
	}
}

func TestWithSourceAdded(t *testing.T) {
	o, err := options.ApplyOptions(WithSourceAdded(), WithDefaultLogLevel(slog.LevelInfo))
	assert.NoError(t, err)
	assert.True(t, o.AddSource)
}

func TestWithReplaceAttr(t *testing.T) {
	s := slog.String("foo", "bar")
	var ra AttrMapper = func(groups []string, a slog.Attr) slog.Attr {
		return s
	}

	o, err := options.ApplyOptions(WithReplaceAttr(ra), WithDefaultLogLevel(slog.LevelInfo))
	assert.NoError(t, err)
	assert.Equal(t, s, o.ReplaceAttr(nil, slog.String("unused", "string")))
}

func TestApplyOptions_error(t *testing.T) {
	e := errors.New("expected")

	_, err := options.ApplyOptions(
		func(o *options.Options) error {
			return e
		},
		func(o *options.Options) error {
			return errors.New("ouch")
		},
	)
	assert.ErrorIs(t, err, e)
}
