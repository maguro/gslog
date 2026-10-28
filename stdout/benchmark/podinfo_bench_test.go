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
