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
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strconv"

	"cloud.google.com/go/logging"

	"m4o.io/gslog/internal/local"
)

const (
	envVarLogLevelKey = "GSLOG_LOG_LEVEL"

	LevelUnknown slog.Level = math.MaxInt
)

var (
	errNoLogLevelSet = fmt.Errorf("no level set for logging")
)

type AugmentEntryFn func(ctx context.Context, e *logging.Entry)

// OptionFunc is function type that is passed to the Logger initialization function.
type OptionFunc func(*Options) error

func (o OptionFunc) Process(options *Options) error {
	return o(options)
}

func (o OptionFunc) InternalOnly() {
}

type Option interface {
	local.InternalMarker
	Process(options *Options) error
}

// Options holds information needed to construct an instance of GcpHandler.
type Options struct {
	explicitLogLevel slog.Level
	envVarLogLevel   slog.Level
	defaultLogLevel  slog.Level

	EntryAugmentors []AugmentEntryFn

	// addSource causes the handler to compute the source code position
	// of the log statement and add a SourceKey attribute to the output.
	addSource bool

	// level reports the minimum record level that will be logged.
	// The handler discards records with lower levels.
	// If level is nil, the handler assumes LevelInfo.
	// The handler calls level.level for each record processed;
	// to adjust the minimum level dynamically, use a LevelVar.
	level slog.Leveler

	// replaceAttr is called to rewrite each non-group attribute before it is logged.
	// The attribute's value has been resolved (see [Value.Resolve]).
	// If replaceAttr returns a zero Attr, the attribute is discarded.
	//
	// The built-in attributes with keys "time", "level", "source", and "msg"
	// are passed to this function, except that time is omitted
	// if zero, and source is omitted if addSource is false.
	//
	// The first argument is a list of currently open groups that contain the
	// Attr. It must not be retained or modified. replaceAttr is never called
	// for Group attributes, only their contents. For example, the attribute
	// list
	//
	//     Int("a", 1), Group("g", Int("b", 2)), Int("c", 3)
	//
	// results in consecutive calls to replaceAttr with the following arguments:
	//
	//     nil, Int("a", 1)
	//     []string{"g"}, Int("b", 2)
	//     nil, Int("c", 3)
	//
	// replaceAttr can be used to change the default keys of the built-in
	// attributes, convert types (for example, to replace a `time.Time` with the
	// integer seconds since the Unix epoch), sanitize personal information, or
	// remove attributes from the output.
	replaceAttr AttrMapper
}

// WithLogLevel returns an option that specifies the log level for logging.
// Explicitly setting the log level here takes precedence over the other
// options.
func WithLogLevel(logLevel slog.Level) Option {
	return OptionFunc(func(o *Options) error {
		o.explicitLogLevel = logLevel
		return nil
	})
}

// WithLogLevelFromEnvVar returns an option that specifies the log level
// for logging comes from tne environmental variable GSLOG_LOG_LEVEL.
func WithLogLevelFromEnvVar() Option {
	return OptionFunc(func(o *Options) error {
		s, ok := os.LookupEnv(envVarLogLevelKey)
		if !ok {
			return nil
		}
		i, err := strconv.Atoi(s)
		if err == nil {
			o.envVarLogLevel = slog.Level(i)
			return nil
		}

		switch s {
		case "DEBUG":
			o.envVarLogLevel = slog.LevelDebug
		case "INFO":
			o.envVarLogLevel = slog.LevelInfo
		case "WARN":
			o.envVarLogLevel = slog.LevelWarn
		case "ERROR":
			o.envVarLogLevel = slog.LevelError
		default:
		}

		return nil
	})
}

// WithDefaultLogLevel returns an option that specifies the default log
// level for logging.
func WithDefaultLogLevel(defaultLogLevel slog.Level) Option {
	return OptionFunc(func(o *Options) error {
		o.defaultLogLevel = defaultLogLevel
		return nil
	})
}

// WithSourceAdded returns an option that causes the handler to compute the
// source code position of the log statement and add a slog.SourceKey attribute
// to the output.
func WithSourceAdded() Option {
	return OptionFunc(func(o *Options) error {
		o.addSource = true
		return nil
	})
}

// WithReplaceAttr returns an option that specifies an attribute mapper used to
// rewrite each non-group attribute before it is logged.
func WithReplaceAttr(replaceAttr AttrMapper) Option {
	return OptionFunc(func(o *Options) error {
		o.replaceAttr = replaceAttr
		return nil
	})
}

func applyOptions(opts ...Option) (*Options, error) {
	o := &Options{
		envVarLogLevel:   LevelUnknown,
		explicitLogLevel: LevelUnknown,
		defaultLogLevel:  LevelUnknown,
		replaceAttr:      noopAttrMapper,
	}
	for _, opt := range opts {
		if err := opt.Process(o); err != nil {
			return nil, err
		}
	}

	o.level = o.defaultLogLevel
	if o.envVarLogLevel != LevelUnknown {
		o.level = o.envVarLogLevel
	}
	if o.explicitLogLevel != LevelUnknown {
		o.level = o.explicitLogLevel
	}
	if o.level == LevelUnknown {
		return nil, errNoLogLevelSet
	}

	return o, nil
}
