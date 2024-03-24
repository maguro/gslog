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

package gslog

import (
	"context"

	"cloud.google.com/go/logging"
)

// Logger is wraps the set of methods that are used when interacting with a
// logging.Logger.  This interface facilitates stubbing out calls to the Logger
// for the purposes of testing and benchmarking.
type Logger interface {
	Log
	LogSync
}

type Log interface {
	Log(e logging.Entry)
}

type LogSync interface {
	LogSync(ctx context.Context, e logging.Entry) error
}

// The LoggerFunc type is an adapter to allow the use of
// ordinary functions as a Logger. If fn is a function
// with the appropriate signature, LoggerFunc(fn) is a
// Logger that calls fn.
type LoggerFunc func(e logging.Entry)

func (fn LoggerFunc) Log(e logging.Entry) {
	fn(e)
}

func (fn LoggerFunc) LogSync(_ context.Context, e logging.Entry) error {
	fn(e)
	return nil
}

// discard can be used as a do-nothing Logger that can be used for testing and
// to stub out Google Cloud Logging when benchmarking.
type discard struct{}

func (d discard) Log(_ logging.Entry) {}

func (d discard) LogSync(_ context.Context, _ logging.Entry) error {
	return nil
}

var Discard Logger = discard{}
