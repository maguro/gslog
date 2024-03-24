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

	"m4o.io/gslog/v1/internal/options"
)

// Options holds information needed to construct an instance of GcpHandler.
type Options struct {
	options.Options
}

// WithLogLevel returns an option that specifies the log level for logging.
// Explicitly setting the log level here takes precedence over the other
// options.
func WithLogLevel(logLevel slog.Level) options.OptionProcessor {
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

	return func(o *options.Options) {
		s, ok := os.LookupEnv(key)
		if !ok {
			return
		}
		i, err := strconv.Atoi(s)
		if err == nil {
			o.EnvVarLogLevel = slog.Level(i)
			return
		}

		switch s {
		case "DEBUG":
			o.EnvVarLogLevel = slog.LevelDebug
		case "INFO":
			o.EnvVarLogLevel = slog.LevelInfo
		case "WARN":
			o.EnvVarLogLevel = slog.LevelWarn
		case "ERROR":
			o.EnvVarLogLevel = slog.LevelError
		default:
			o.EnvVarLogLevel = slog.LevelInfo
		}
	}
}

// WithDefaultLogLevel returns an option that specifies the default log
// level for logging.
func WithDefaultLogLevel(defaultLogLevel slog.Level) options.OptionProcessor {
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
