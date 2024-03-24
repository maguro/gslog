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

package gslog_test

import (
	"context"
	"testing"

	"cloud.google.com/go/logging"
	"github.com/stretchr/testify/assert"

	"m4o.io/gslog"
)

func TestLoggerFunc_Log(t *testing.T) {
	var called bool

	l := gslog.LoggerFunc(func(e logging.Entry) {
		called = true
	})

	l.Log(logging.Entry{})

	assert.True(t, called)
}

func TestLoggerFunc_LogSync(t *testing.T) {
	var called bool

	l := gslog.LoggerFunc(func(e logging.Entry) {
		called = true
	})

	ctx := context.Background()
	err := l.LogSync(ctx, logging.Entry{})

	assert.NoError(t, err)
	assert.True(t, called)
}

func TestDiscard_Log(t *testing.T) {
	l := gslog.Discard
	l.Log(logging.Entry{})
}

func TestDiscard_LogSync(t *testing.T) {
	l := gslog.Discard
	err := l.LogSync(context.Background(), logging.Entry{})
	assert.NoError(t, err)
}
