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

package gcp

import (
	"context"

	"cloud.google.com/go/logging"
)

// Logger wraps the methods that the handler calls on a logging.Logger.  Tests
// and benchmarks can implement this interface to stub the Logger.
type Logger interface {
	Log
	LogSync

	// Flush blocks until all log entries that are currently buffered are sent.
	//
	// Flush returns a non-nil error if errors occurred since the last call to
	// Flush from any Logger.  If this is the first call, the errors count from
	// the creation of the client.  The error contains summary information about
	// the errors.  This information is unlikely to be actionable.  For more
	// accurate error reports, set Client.OnError.
	Flush() error
}

// Log wraps the asynchronous buffered logging of records to
// Google Cloud Logging.
type Log interface {
	// Log buffers the Entry for output to the logging service.  Log never
	// blocks.
	Log(e logging.Entry)
}

// LogSync wraps the synchronous logging of records to
// Google Cloud Logging.
type LogSync interface {
	// LogSync logs the Entry synchronously with no buffer.  LogSync is slow
	// and blocks.  Because of this, use LogSync primarily for debugging or
	// critical errors.  Prefer Log for most uses.
	LogSync(ctx context.Context, e logging.Entry) error
}

// LoggerFunc is an adapter that lets an ordinary function operate as a
// Logger.  If fn is a function with the correct signature, LoggerFunc(fn) is
// a Logger that calls fn.
type LoggerFunc func(e logging.Entry)

// Log implements Log.Log.
func (fn LoggerFunc) Log(e logging.Entry) {
	fn(e)
}

// LogSync implements LogSync.LogSync.
func (fn LoggerFunc) LogSync(_ context.Context, e logging.Entry) error {
	fn(e)

	return nil
}

// Flush implements Logger.Flush.
func (fn LoggerFunc) Flush() error {
	return nil
}
