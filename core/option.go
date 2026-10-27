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

package core

import (
	"log/slog"
	"os"

	"m4o.io/gslog/internal/level"
	"m4o.io/gslog/internal/options"
)

// Option configures a handler.
type Option = options.OptionProcessor

// WithLeveler returns an option that specifies the slog.Leveler for logging.
// This option has precedence over the other log level options.
func WithLeveler(leveler slog.Leveler) Option {
	return func(o *options.Options) {
		o.ExplicitLogLevel = leveler
	}
}

// WithLogLevelFromEnvVar returns an option that reads the log level from the
// environment variable that key names.  The option reads the variable when
// WithLogLevelFromEnvVar is called.  The value is an integer slog.Level, such
// as -4 for Debug, or the name of a level with an optional offset, such as
// "DEBUG", "warn", or "INFO+2".  The names are the slog names and "WARNING",
// "NOTICE", "CRITICAL", "ALERT", and "EMERGENCY".  The case of a name is not
// important.  The option ignores space at the start and end of the value.
//
// If the variable is not set, the option has no effect.  If the value does
// not name a level, WithLogLevelFromEnvVar logs a warning with the default
// slog.Logger, and the option has no effect.
func WithLogLevelFromEnvVar(key string) Option {
	if key == "" {
		panic("Env var key is empty")
	}

	str, ok := os.LookupEnv(key)
	if !ok {
		return func(_ *options.Options) {}
	}

	envVarLogLevel, ok := level.Parse(str)
	if !ok {
		slog.Warn("Invalid log level", "key", key, "value", str)

		return func(_ *options.Options) {}
	}

	return func(o *options.Options) {
		o.EnvVarLogLevel = envVarLogLevel
	}
}

// WithDefaultLeveler returns an option that specifies the default
// slog.Leveler for logging.
func WithDefaultLeveler(defaultLeveler slog.Leveler) Option {
	return func(o *options.Options) {
		o.DefaultLogLevel = defaultLeveler
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
