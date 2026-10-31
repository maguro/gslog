// Copyright the original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package benchmark_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"m4o.io/gslog"
	"m4o.io/gslog/k8s"
	"m4o.io/gslog/stdout"
)

// podinfoRoot writes a podinfo labels file with three labels in a temporary
// directory and returns the directory.
func podinfoRoot(b *testing.B) string {
	b.Helper()

	root := b.TempDir()
	path := filepath.Join(root, "labels")

	err := os.WriteFile(path, []byte("app=\"hello\"\ntier=\"backend\"\ntrack=\"stable\"\n"), 0o600)
	if err != nil {
		b.Fatal(err)
	}

	return root
}

func BenchmarkHandlePodinfoLabels(b *testing.B) {
	root := podinfoRoot(b)
	h := stdout.NewHandler(io.Discard, gslog.WithLeveler(slog.LevelInfo), k8s.WithPodinfoLabels(root))
	ctx := context.Background()

	handle(ctx, b, h, slog.String("s", "hello"))
}

func BenchmarkHandlePodinfoAndContextLabels(b *testing.B) {
	root := podinfoRoot(b)
	h := stdout.NewHandler(io.Discard, gslog.WithLeveler(slog.LevelInfo), k8s.WithPodinfoLabels(root))

	ctx := context.Background()
	ctx = gslog.WithLabels(ctx, gslog.Label("tenant", "acme"), gslog.Label("region", "eu"))

	handle(ctx, b, h, slog.String("s", "hello"))
}
