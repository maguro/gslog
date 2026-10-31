// Copyright the original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otel_test

import (
	"context"
	"log/slog"

	"m4o.io/gslog"
	"m4o.io/gslog/internal/entry"
	"m4o.io/gslog/internal/options"
)

// entryOf applies the options and returns the entry that their augmentors
// build for ctx.  An augmentor needs no handler, so a test of an option
// asserts on the entry and not on the output of a handler.  A handler has
// its own tests for the way that it writes the entry.
func entryOf(ctx context.Context, opts ...gslog.Option) entry.Entry {
	o := options.ApplyOptions(opts...)

	var e entry.Entry

	for _, augment := range o.EntryAugmentors {
		augment(ctx, &e)
	}

	return e
}

// rendered returns the string form of each attribute.  A slog.Value holds a
// pointer to the bytes of a string value, so a comparison of two attributes
// that hold equal strings can fail.  The string form holds the key, the
// value, and the members of a group.
func rendered(attrs []slog.Attr) []string {
	out := make([]string, 0, len(attrs))

	for _, a := range attrs {
		out = append(out, a.String())
	}

	return out
}
