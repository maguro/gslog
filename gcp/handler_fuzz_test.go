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
	"context"
	"log/slog"
	"testing"
	"time"

	"m4o.io/gslog/core"
	"m4o.io/gslog/gcp"
)

// FuzzHandle gives an arbitrary message, group name, and attributes to the
// handler. An application supplies these values, and the handler must put
// each one into the payload without a panic.
func FuzzHandle(f *testing.F) {
	f.Add("message", "group", "key", "value")
	f.Add("", "", "", "")
	f.Add("message", "a.b", "key.with.dots", "value")
	f.Add("message", "grp", "key", "\x00\xff invalid utf8")
	f.Add("message", "grp", "", "no key")

	f.Fuzz(func(t *testing.T, msg, group, key, value string) {
		got := &Got{}

		var h slog.Handler = gcp.NewHandler(got, core.WithDefaultLogLeveler(slog.LevelInfo))
		h = h.WithGroup(group)

		attrs := []slog.Attr{slog.String(key, value), slog.Group(group, slog.String(key, value))}
		h = h.WithAttrs(attrs)

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, msg, 0)
		record.AddAttrs(slog.String(key, value))

		if err := h.Handle(context.Background(), record); err != nil {
			t.Fatalf("handle record: %v", err)
		}
	})
}
