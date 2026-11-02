// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package stdout_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
	"testing/slogtest"

	"github.com/stretchr/testify/require"

	"m4o.io/gslog"
	"m4o.io/gslog/stdout"
)

// The keys that the handler writes for the built-in attributes.
const (
	severityKey  = "severity"
	timestampKey = "timestamp"
)

// TestSlogtest runs the slog.Handler conformance suite of testing/slogtest
// against the stdout handler.
func TestSlogtest(t *testing.T) {
	var buf bytes.Buffer

	newHandler := func(*testing.T) slog.Handler {
		buf.Reset()

		return stdout.NewHandler(&buf)
	}

	result := func(t *testing.T) map[string]any {
		var m map[string]any

		err := json.Unmarshal(buf.Bytes(), &m)
		require.NoError(t, err)

		return withSlogKeys(m)
	}

	slogtest.Run(t, newHandler, result)
}

// withSlogKeys moves the severity, message, and timestamp members of m to
// the keys that testing/slogtest expects.  withSlogKeys returns m.
func withSlogKeys(m map[string]any) map[string]any {
	rename := func(from, to string) {
		v, ok := m[from]
		if !ok {
			return
		}

		delete(m, from)
		m[to] = v
	}

	rename(severityKey, slog.LevelKey)
	rename(gslog.MessageKey, slog.MessageKey)
	rename(timestampKey, slog.TimeKey)

	return m
}
