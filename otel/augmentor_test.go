// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

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
