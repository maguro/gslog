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

package gslog

import (
	"log/slog"

	"m4o.io/gslog/core"
	"m4o.io/gslog/internal/options"
)

// Options holds the information necessary to construct an instance of
// GcpHandler.
//
// Deprecated: No function accepts Options.  Use the functions that return
// core.Option.
type Options struct {
	options.Options
}

// Option configures a handler.
//
// Deprecated: Use core.Option.
type Option = core.Option

// WithLogLeveler returns an option that specifies the slog.Leveler for
// logging.
//
// Deprecated: Use core.WithLogLeveler.
func WithLogLeveler(logLevel slog.Leveler) Option {
	return core.WithLogLeveler(logLevel)
}

// WithLogLevelFromEnvVar returns an option that reads the log level from the
// environment variable that key names.
//
// Deprecated: Use core.WithLogLevelFromEnvVar.
func WithLogLevelFromEnvVar(key string) Option {
	return core.WithLogLevelFromEnvVar(key)
}

// WithDefaultLogLeveler returns an option that specifies the default
// slog.Leveler for logging.
//
// Deprecated: Use core.WithDefaultLogLeveler.
func WithDefaultLogLeveler(defaultLogLevel slog.Leveler) Option {
	return core.WithDefaultLogLeveler(defaultLogLevel)
}

// WithSourceAdded returns an option that causes the handler to compute the
// source code position of the log statement.
//
// Deprecated: Use core.WithSourceAdded.
func WithSourceAdded() Option {
	return core.WithSourceAdded()
}

// WithReplaceAttr returns an option that specifies an attribute mapper.
//
// Deprecated: Use core.WithReplaceAttr.
func WithReplaceAttr(replaceAttr AttrMapper) Option {
	return core.WithReplaceAttr(replaceAttr)
}
