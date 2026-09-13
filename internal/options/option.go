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

The Options struct is in this internal package to limit access to it.
*/
package options

import (
	"context"
	"log/slog"
	"math"

	"cloud.google.com/go/logging"
)

const (
	levelUnknown = slog.Level(math.MaxInt)
)

// EntryAugmentor augments an instance of logging.Entry.  The handler supplies
// the current context and group path for the augmentor to use.
//
// The entry payload shares values with the handler.  Add new fields to the
// payload.  Do not modify a value that is already in the payload.
type EntryAugmentor func(ctx context.Context, e *logging.Entry, groups []string)

// Options holds the information necessary to construct an instance of
// GcpHandler.
type Options struct {
	ExplicitLogLevel slog.Leveler
	EnvVarLogLevel   slog.Level
	DefaultLogLevel  slog.Leveler

	EntryAugmentors []EntryAugmentor

	// AddSource causes the handler to compute the source code position
	// of the log statement.  The handler sets the position in the
	// SourceLocation field of the entry.
	AddSource bool

	// Level reports the minimum record level that the handler logs.
	// The handler discards records with lower levels.
	// If Level is nil, the handler assumes LevelInfo.
	// The handler calls Level.Level() for each record that it processes.
	// To adjust the minimum level dynamically, use a LevelVar.
	Level slog.Leveler

	// ReplaceAttr rewrites each non-group attribute before the handler logs
	// the attribute.  The handler resolves the value of the attribute before
	// the call (see [slog.Value.Resolve]).  If ReplaceAttr returns a zero
	// Attr, the handler discards the attribute.
	//
	// The handler passes the built-in attribute with key "message" to this
	// function.
	//
	// The first argument is a list of the open groups that contain the Attr.
	// Do not retain or modify this list.  The handler never calls ReplaceAttr
	// for a Group attribute.  The handler calls ReplaceAttr for the contents
	// of the group.  For example, the attribute list
	//
	//     Int("a", 1), Group("g", Int("b", 2)), Int("c", 3)
	//
	// results in consecutive calls to ReplaceAttr with these arguments:
	//
	//     nil, Int("a", 1)
	//     []string{"g"}, Int("b", 2)
	//     nil, Int("c", 3)
	//
	// ReplaceAttr can change the default keys of the built-in attributes,
	// convert types (for example, replace a `time.Time` with the integer
	// seconds since the Unix epoch), sanitize personal information, or remove
	// attributes from the output.
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr
}

// OptionProcessor interacts with the supplied Options instance.
type OptionProcessor func(o *Options)

// ApplyOptions applies the option processors to an instance of Options.
// ApplyOptions returns that instance.
func ApplyOptions(options ...OptionProcessor) *Options {
	opts := &Options{
		EnvVarLogLevel:   levelUnknown,
		ExplicitLogLevel: levelUnknown,
		DefaultLogLevel:  levelUnknown,

		EntryAugmentors: nil,
		AddSource:       false,
		Level:           slog.LevelInfo,
		ReplaceAttr:     nil,
	}
	for _, opt := range options {
		opt(opts)
	}

	opts.Level = opts.DefaultLogLevel

	if opts.EnvVarLogLevel != levelUnknown {
		opts.Level = opts.EnvVarLogLevel
	}

	if opts.ExplicitLogLevel != levelUnknown {
		opts.Level = opts.ExplicitLogLevel
	}

	if opts.Level == levelUnknown {
		opts.Level = slog.LevelInfo
	}

	return opts
}
