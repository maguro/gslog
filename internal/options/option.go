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

/*
Package options holds the options handling code.

The Options struct is held in this internal package to button down access.
*/
package options

import (
	"context"
	"log/slog"
	"math"

	"cloud.google.com/go/logging"
)

var (
	levelUnknown = slog.Level(math.MaxInt)
)

type EntryAugmentor func(ctx context.Context, e *logging.Entry, groups []string)

// Options holds information needed to construct an instance of GcpHandler.
type Options struct {
	ExplicitLogLevel slog.Leveler
	EnvVarLogLevel   slog.Level
	DefaultLogLevel  slog.Leveler

	EntryAugmentors []EntryAugmentor

	// AddSource causes the handler to compute the source code position
	// of the log statement and add a SourceKey attribute to the output.
	AddSource bool

	// Level reports the minimum record level that will be logged.
	// The handler discards records with lower levels.
	// If Level is nil, the handler assumes LevelInfo.
	// The handler calls Level.Level() for each record processed;
	// to adjust the minimum level dynamically, use a LevelVar.
	Level slog.Leveler

	// ReplaceAttr is called to rewrite each non-group attribute before it is logged.
	// The attribute's value has been resolved (see [Value.Resolve]).
	// If ReplaceAttr returns a zero Attr, the attribute is discarded.
	//
	// The built-in attributes with keys "time", "level", "source", and "msg"
	// are passed to this function, except that time is omitted
	// if zero, and source is omitted if addSource is false.
	//
	// The first argument is a list of currently open groups that contain the
	// Attr. It must not be retained or modified. ReplaceAttr is never called
	// for Group attributes, only their contents. For example, the attribute
	// list
	//
	//     Int("a", 1), Group("g", Int("b", 2)), Int("c", 3)
	//
	// results in consecutive calls to ReplaceAttr with the following arguments:
	//
	//     nil, Int("a", 1)
	//     []string{"g"}, Int("b", 2)
	//     nil, Int("c", 3)
	//
	// ReplaceAttr can be used to change the default keys of the built-in
	// attributes, convert types (for example, to replace a `time.Time` with the
	// integer seconds since the Unix epoch), sanitize personal information, or
	// remove attributes from the output.
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr
}

type OptionProcessor func(o *Options)

func ApplyOptions(opts ...OptionProcessor) *Options {
	o := &Options{
		EnvVarLogLevel:   levelUnknown,
		ExplicitLogLevel: levelUnknown,
		DefaultLogLevel:  levelUnknown,
	}
	for _, opt := range opts {
		opt(o)
	}

	o.Level = o.DefaultLogLevel
	if o.EnvVarLogLevel != levelUnknown {
		o.Level = o.EnvVarLogLevel
	}
	if o.ExplicitLogLevel != levelUnknown {
		o.Level = o.ExplicitLogLevel
	}
	if o.Level == levelUnknown {
		o.Level = slog.LevelInfo
	}

	return o
}
