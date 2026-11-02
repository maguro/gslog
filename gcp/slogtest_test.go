// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package gcp_test

import (
	"log/slog"
	"testing"
	"testing/slogtest"

	"cloud.google.com/go/logging"
	"github.com/stretchr/testify/require"
	spb "google.golang.org/protobuf/types/known/structpb"

	"m4o.io/gslog"
	"m4o.io/gslog/gcp"
)

// TestSlogtest runs the slog.Handler conformance suite of testing/slogtest
// against the gcp handler.
func TestSlogtest(t *testing.T) {
	var entries []logging.Entry

	newHandler := func(*testing.T) slog.Handler {
		entries = nil

		logger := gcp.LoggerFunc(func(e logging.Entry) {
			entries = append(entries, e)
		})

		return gcp.NewHandler(logger)
	}

	result := func(t *testing.T) map[string]any {
		require.Len(t, entries, 1)

		return entryToMap(t, entries[0])
	}

	slogtest.Run(t, newHandler, result)
}

// entryToMap returns the payload of e as a map.  The map has the severity,
// the message, and the timestamp of e at the keys that testing/slogtest
// expects.  The map has no timestamp when the timestamp of e is zero.
func entryToMap(t *testing.T, e logging.Entry) map[string]any {
	t.Helper()

	payload, ok := e.Payload.(*spb.Struct)
	require.True(t, ok)

	m := payload.AsMap()

	if v, ok := m[gslog.MessageKey]; ok {
		delete(m, gslog.MessageKey)
		m[slog.MessageKey] = v
	}

	m[slog.LevelKey] = e.Severity.String()

	if !e.Timestamp.IsZero() {
		m[slog.TimeKey] = e.Timestamp
	}

	return m
}
