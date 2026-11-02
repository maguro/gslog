// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package entry

import "log/slog"

// The keys and the type value that Google Cloud Error Reporting reads at the
// top level of the payload of a log entry.
const (
	ReportTypeKey     = "@type"
	ReportTypeValue   = "type.googleapis.com/google.devtools.clouderrorreporting.v1beta1.ReportedErrorEvent"
	ServiceContextKey = "serviceContext"
	ServiceKey        = "service"
	VersionKey        = "version"
	StackTraceKey     = "stack_trace"
)

// ErrorReport holds the data that Google Cloud Error Reporting reads for one
// log record.  A handler writes the error report at the top level of the
// payload.
type ErrorReport struct {
	Service    string
	Version    string
	StackTrace string
}

// Reporter gives the error report for a record.  A record below MinLevel has
// no error report.  Report returns the error report for a record at MinLevel
// or higher.
type Reporter struct {
	MinLevel slog.Level
	Report   func() ErrorReport
}

// For returns the error report for a record at the level.  ok is false when
// r is nil, or when the level is below MinLevel.
func (r *Reporter) For(level slog.Level) (report ErrorReport, ok bool) {
	if r == nil || level < r.MinLevel {
		return ErrorReport{}, false
	}

	return r.Report(), true
}
