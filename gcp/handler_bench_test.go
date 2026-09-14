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

	"cloud.google.com/go/logging"

	"m4o.io/gslog/core"
	"m4o.io/gslog/gcp"
)

type noopLogger struct{}

func (noopLogger) Log(logging.Entry) {}

func (noopLogger) LogSync(context.Context, logging.Entry) error {
	return nil
}

func (noopLogger) Flush() error {
	return nil
}

func BenchmarkHandleThreeAttrs(b *testing.B) {
	h := gcp.NewHandler(noopLogger{}, core.WithLogLeveler(slog.LevelInfo))
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		r := slog.NewRecord(time.Time{}, slog.LevelInfo, "message", 0)
		r.AddAttrs(slog.String("s", "hello"), slog.Int("n", 42), slog.Bool("b", true))

		if err := h.Handle(ctx, r); err != nil {
			b.Fatal(err)
		}
	}
}
