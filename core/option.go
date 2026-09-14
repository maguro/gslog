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

package core

import (
	"log/slog"
	"os"
	"strconv"

	"m4o.io/gslog/internal/options"
)

// Option configures a handler.
type Option = options.OptionProcessor

// WithLogLeveler returns an option that specifies the slog.Leveler for logging.
// This option has precedence over the other log level options.
func WithLogLeveler(logLevel slog.Leveler) Option {
	return func(o *options.Options) {
		o.ExplicitLogLevel = logLevel
	}
}

// WithLogLevelFromEnvVar returns an option that reads the log level from the
// environment variable that key names.
func WithLogLevelFromEnvVar(key string) Option {
	if key == "" {
		panic("Env var key is empty")
	}

	var envVarLogLevel slog.Level

	setLogLevel := func(o *options.Options) {
		o.EnvVarLogLevel = envVarLogLevel
	}

	str, ok := os.LookupEnv(key)
	if !ok {
		return func(_ *options.Options) {}
	}

	lvl, err := strconv.Atoi(str)
	if err == nil {
		envVarLogLevel = slog.Level(lvl)

		return setLogLevel
	}

	switch str {
	case "DEBUG":
		envVarLogLevel = slog.LevelDebug
	case "INFO":
		envVarLogLevel = slog.LevelInfo
	case "WARN":
		envVarLogLevel = slog.LevelWarn
	case "ERROR":
		envVarLogLevel = slog.LevelError
	default:
		envVarLogLevel = slog.LevelInfo
	}

	return setLogLevel
}

// WithDefaultLogLeveler returns an option that specifies the default
// slog.Leveler for logging.
func WithDefaultLogLeveler(defaultLogLevel slog.Leveler) Option {
	return func(o *options.Options) {
		o.DefaultLogLevel = defaultLogLevel
	}
}

// WithSourceAdded returns an option that causes the handler to compute the
// source code position of the log statement.  The handler sets the position
// in the SourceLocation field of the entry.
func WithSourceAdded() Option {
	return func(o *options.Options) {
		o.AddSource = true
	}
}

// WithReplaceAttr returns an option that specifies an attribute mapper.  The
// handler calls the mapper to rewrite each non-group attribute before the
// handler logs the attribute.
func WithReplaceAttr(replaceAttr AttrMapper) Option {
	return func(o *options.Options) {
		o.ReplaceAttr = replaceAttr
	}
}
