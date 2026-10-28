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

package gslog_test

import (
	"context"
	"log/slog"
	"os"
	"time"

	"m4o.io/gslog"
	"m4o.io/gslog/stdout"
)

// The options and the labels of this package apply to both handlers.  The
// example sets the default level of a stdout handler and adds a label from
// the context to a record.  The handler writes the record as one line of
// JSON.  A fixed time keeps the timestamp of the output the same on each run.
func Example() {
	h := stdout.NewHandler(os.Stdout, gslog.WithDefaultLeveler(slog.LevelDebug))

	ctx := context.Background()
	ctx = gslog.WithLabels(ctx, gslog.Label("tenant", "acme"))

	at := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	r := slog.NewRecord(at, slog.LevelDebug, "How now brown cow?", 0)
	r.AddAttrs(slog.String("animal", "cow"))

	_ = h.Handle(ctx, r)

	// Output: {"severity":"DEBUG","message":"How now brown cow?","timestamp":"2024-01-02T03:04:05Z","logging.googleapis.com/labels":{"tenant":"acme"},"animal":"cow"}
}
