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
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"
	"unicode/utf8"

	"m4o.io/gslog/core"
	"m4o.io/gslog/stdout"
)

// FuzzHandle gives an arbitrary message, group name, and attributes to the
// handler.  The handler must write one line of valid JSON that holds the
// message, and must not panic.
func FuzzHandle(f *testing.F) {
	f.Add("message", "group", "key", "value")
	f.Add("", "", "", "")
	f.Add("message", "a.b", "key.with.dots", "value")
	f.Add("message", "grp", "key", "\x00\xff invalid utf8")
	f.Add("message", "grp", "", "no key")
	f.Add("quote\"back\\slash", "severity", "message", "\n\t\u2028")

	f.Fuzz(func(t *testing.T, msg, group, key, value string) {
		var buf bytes.Buffer

		var h slog.Handler = stdout.NewHandler(&buf, core.WithDefaultLogLeveler(slog.LevelInfo))
		h = h.WithGroup(group)

		attrs := []slog.Attr{slog.String(key, value), slog.Group(group, slog.String(key, value))}
		h = h.WithAttrs(attrs)

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, msg, 0)
		record.AddAttrs(slog.String(key, value))

		if err := h.Handle(context.Background(), record); err != nil {
			t.Fatalf("handle record: %v", err)
		}

		line := buf.Bytes()
		if !json.Valid(line) {
			t.Fatalf("invalid JSON: %q", line)
		}

		if bytes.Count(line, []byte("\n")) != 1 {
			t.Fatalf("not one line: %q", line)
		}

		var got map[string]any

		if err := json.Unmarshal(line, &got); err != nil {
			t.Fatalf("decode line: %v", err)
		}

		if utf8.ValidString(msg) && got["message"] != msg {
			t.Fatalf("message: got %q, want %q", got["message"], msg)
		}
	})
}
