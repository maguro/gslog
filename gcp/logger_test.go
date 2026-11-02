// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package gcp_test

import (
	"context"
	"testing"

	"cloud.google.com/go/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"m4o.io/gslog/gcp"
)

type discard struct{}

func (d discard) Log(_ logging.Entry) {}

func (d discard) LogSync(_ context.Context, _ logging.Entry) error {
	return nil
}

func (d discard) Flush() error {
	return nil
}

// Discard is a Logger that does nothing.  Tests and benchmarks can use
// Discard as a stub for Google Cloud Logging.
var Discard gcp.Logger = discard{}

func TestLoggerFunc_Log(t *testing.T) {
	var called bool

	l := gcp.LoggerFunc(func(e logging.Entry) {
		called = true
	})

	l.Log(logging.Entry{})

	assert.True(t, called)
}

func TestLoggerFunc_LogSync(t *testing.T) {
	var called bool

	l := gcp.LoggerFunc(func(e logging.Entry) {
		called = true
	})

	ctx := context.Background()
	err := l.LogSync(ctx, logging.Entry{})

	require.NoError(t, err)
	assert.True(t, called)
}

func TestLoggerFunc_Flush(t *testing.T) {
	l := gcp.LoggerFunc(func(_ logging.Entry) {})

	assert.NoError(t, l.Flush())
}

func TestDiscard_Log(t *testing.T) {
	l := Discard
	l.Log(logging.Entry{})
}

func TestDiscard_LogSync(t *testing.T) {
	l := Discard
	err := l.LogSync(context.Background(), logging.Entry{})
	assert.NoError(t, err)
}
