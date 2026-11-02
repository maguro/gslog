// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

// Package testhelp contains helpers that the handler tests share.
package testhelp

import (
	"log/slog"
	"runtime"
	"time"

	"m4o.io/gslog"
	"m4o.io/gslog/internal/entry"
	"m4o.io/gslog/internal/options"
)

// Time is the fixed time of the test records.
var Time = time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)

// CallerPC returns the program counter at the given stack depth.  Depth 1 is
// the caller of CallerPC.
func CallerPC(depth int) uintptr {
	var pcs [1]uintptr

	runtime.Callers(depth, pcs[:])

	return pcs[0]
}

// WithErrorReport returns an option that gives the error report to each
// record at level Error or higher.
func WithErrorReport(report entry.ErrorReport) gslog.Option {
	reporter := &entry.Reporter{
		MinLevel: slog.LevelError,
		Report: func() entry.ErrorReport {
			return report
		},
	}

	return func(o *options.Options) {
		o.ErrorReporter = reporter
	}
}
