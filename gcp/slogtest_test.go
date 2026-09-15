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

package gcp_test

import (
	"log/slog"
	"testing"
	"testing/slogtest"

	"cloud.google.com/go/logging"
	"github.com/stretchr/testify/require"
	spb "google.golang.org/protobuf/types/known/structpb"

	"m4o.io/gslog/core"
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

	if v, ok := m[core.MessageKey]; ok {
		delete(m, core.MessageKey)
		m[slog.MessageKey] = v
	}

	m[slog.LevelKey] = e.Severity.String()

	if !e.Timestamp.IsZero() {
		m[slog.TimeKey] = e.Timestamp
	}

	return m
}
