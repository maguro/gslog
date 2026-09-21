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
