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

package gslog

import "m4o.io/gslog/gcp"

// GcpHandler is a slog.Handler that writes to Google Cloud Logging through
// the API client.
//
// Deprecated: Use gcp.Handler.
type GcpHandler = gcp.Handler

// Logger wraps the methods that the handler calls on a logging.Logger.
//
// Deprecated: Use gcp.Logger.
type Logger = gcp.Logger

// Log wraps the asynchronous buffered logging of records.
//
// Deprecated: Use gcp.Log.
type Log = gcp.Log

// LogSync wraps the synchronous logging of records.
//
// Deprecated: Use gcp.LogSync.
type LogSync = gcp.LogSync

// LoggerFunc is an adapter that lets an ordinary function operate as a
// Logger.
//
// Deprecated: Use gcp.LoggerFunc.
type LoggerFunc = gcp.LoggerFunc

// NewGcpHandler creates a GcpHandler that writes to Google Cloud Logging
// through the logger.
//
// Deprecated: Use gcp.NewHandler.
func NewGcpHandler(logger Logger, opts ...Option) *GcpHandler {
	return gcp.NewHandler(logger, opts...)
}
