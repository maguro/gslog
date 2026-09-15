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

package stdout_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
	"testing/slogtest"

	"github.com/stretchr/testify/require"

	"m4o.io/gslog/core"
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
	rename(core.MessageKey, slog.MessageKey)
	rename(timestampKey, slog.TimeKey)

	return m
}
