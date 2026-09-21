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

package errorreporting

import (
	"log/slog"

	"m4o.io/gslog/core"
	"m4o.io/gslog/internal/entry"
	"m4o.io/gslog/internal/options"
)

// WithService returns an option that causes the handler to write the Google
// Cloud Error Reporting fields for the service.  The handler writes the
// fields in each record at level Error or higher.  The fields are the type
// of a ReportedErrorEvent, the service context, and a stack trace.  The
// service context has the service, and has the version only when version is
// not empty.  The stack trace is the stack of the log call.  The handler
// does not write a top-level attribute or group that has the key of a field
// in the same record.  If the options have more than one WithService option,
// the handler uses the last WithService option.  WithService panics if
// service is empty.
func WithService(service, version string) core.Option {
	if service == "" {
		panic("service is empty")
	}

	r := reporter(service, version)

	return func(o *options.Options) {
		o.ErrorReporter = r
	}
}

// reporter returns the reporter that gives an error report for a record at
// level Error or higher.
func reporter(service, version string) *entry.Reporter {
	return &entry.Reporter{
		MinLevel: slog.LevelError,
		Report:   report(service, version),
	}
}

// report returns the function that makes the error report for a record.
func report(service, version string) func() entry.ErrorReport {
	return func() entry.ErrorReport {
		stack := captureStack()

		return entry.ErrorReport{
			Service:    service,
			Version:    version,
			StackTrace: stack,
		}
	}
}
