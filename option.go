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
	"log/slog"
	"os"
	"strconv"

	"m4o.io/gslog/internal/options"
)

// Options holds information needed to construct an instance of GcpHandler.
type Options struct {
	options.Options
}

// WithLogLeveler returns an option that specifies the slog.Leveler for logging.
// Explicitly setting the log level here takes precedence over the other
// options.
func WithLogLeveler(logLevel slog.Leveler) options.OptionProcessor {
	return func(o *options.Options) {
		o.ExplicitLogLevel = logLevel
	}
}

// WithLogLevelFromEnvVar returns an option that specifies the log level
// for logging comes from tne environmental variable specified by the key.
func WithLogLevelFromEnvVar(key string) options.OptionProcessor {
	if key == "" {
		panic("Env var key is empty")
	}

	var envVarLogLevel slog.Level

	setLogLevel := func(o *options.Options) {
		o.EnvVarLogLevel = envVarLogLevel
	}

	s, ok := os.LookupEnv(key)
	if !ok {
		return func(o *options.Options) {}
	}
	i, err := strconv.Atoi(s)
	if err == nil {
		envVarLogLevel = slog.Level(i)
		return setLogLevel
	}

	switch s {
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
func WithDefaultLogLeveler(defaultLogLevel slog.Leveler) options.OptionProcessor {
	return func(o *options.Options) {
		o.DefaultLogLevel = defaultLogLevel
	}
}

// WithSourceAdded returns an option that causes the handler to compute the
// source code position of the log statement and add a slog.SourceKey attribute
// to the output.
func WithSourceAdded() options.OptionProcessor {
	return func(o *options.Options) {
		o.AddSource = true
	}
}

// WithReplaceAttr returns an option that specifies an attribute mapper used to
// rewrite each non-group attribute before it is logged.
func WithReplaceAttr(replaceAttr AttrMapper) options.OptionProcessor {
	return func(o *options.Options) {
		o.ReplaceAttr = replaceAttr
	}
}
