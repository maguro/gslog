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

package k8s_test

import (
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"m4o.io/gslog/internal/entry"
	"m4o.io/gslog/internal/options"
	"m4o.io/gslog/k8s"
)

// formatMap mirrors FormatMap in the Kubernetes package pkg/fieldpath, which
// is the only writer of the podinfo labels file.  FormatMap writes the key
// without quotes, then the separator, then the value as a Go quoted string.
// FormatMap sorts the keys and puts one label on each line.  The last line has
// no line break.
//
// The Kubernetes documentation of the Downward API gives the shape of a line
// but not the escape rules, so this function is the reference for the tests.
func formatMap(m map[string]string) string {
	var sb strings.Builder

	for i, key := range slices.Sorted(maps.Keys(m)) {
		if i > 0 {
			sb.WriteByte('\n')
		}

		sb.WriteString(key)
		sb.WriteByte('=')
		sb.WriteString(strconv.Quote(m[key]))
	}

	return sb.String()
}

// loadLabels writes data to a labels file in a new directory, then returns the
// labels that WithPodinfoLabels gets from the file.
func loadLabels(t *testing.T, data string) map[string]string {
	t.Helper()

	dir := t.TempDir()

	path := filepath.Join(dir, "labels")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write labels file: %v", err)
	}

	o := &options.Options{}
	k8s.WithPodinfoLabels(dir)(o)

	e := &entry.Entry{}
	for _, augment := range o.EntryAugmentors {
		augment(t.Context(), e)
	}

	return e.Labels.Map
}

// discardLogs sends the default slog.Logger to a discard handler for the
// test.  A fuzz target gives WithPodinfoLabels many invalid files, and each
// one logs a warning.
func discardLogs(t *testing.T) {
	t.Helper()

	previous := slog.Default()
	slog.SetDefault(slog.New(slog.DiscardHandler))

	t.Cleanup(func() {
		slog.SetDefault(previous)
	})
}

// FuzzPodinfoLabels gives a label that FormatMap wrote to WithPodinfoLabels.
// The option must get the same key and value back, because the parser is the
// inverse of FormatMap.
func FuzzPodinfoLabels(f *testing.F) {
	f.Add("app", "hello-world")
	f.Add("app.kubernetes.io/name", "gslog")
	f.Add("empty", "")
	f.Add("tab", "a\tb")
	f.Add("quote", `a"b`)
	f.Add("backslash", `a\b`)
	f.Add("newline", "a\nb")
	f.Add("invalid", "\x00\xff utf8")
	f.Add("expansion", "${a}")
	f.Add("separator", "a=b")
	f.Add("padded", "  a  ")
	f.Add("comment", "#a")

	f.Fuzz(func(t *testing.T, key, val string) {
		// FormatMap writes the key without quotes, so the format cannot hold a
		// key that has a separator or a line break.  Kubernetes label
		// validation rejects every such key.
		if key == "" || strings.ContainsAny(key, "=\n\r") || key != strings.TrimSpace(key) {
			t.Skip("Kubernetes cannot have this label key")
		}

		discardLogs(t)

		data := formatMap(map[string]string{key: val})
		got := loadLabels(t, data)

		wantKey := k8s.PodPrefix + entry.ValidUTF8(key)
		wantVal := entry.ValidUTF8(val)

		if len(got) != 1 {
			t.Fatalf("file %q: got %d labels, want 1: %q", data, len(got), got)
		}

		if got[wantKey] != wantVal {
			t.Fatalf("file %q: got %q, want %q at key %q", data, got[wantKey], wantVal, wantKey)
		}
	})
}

// FuzzPodinfoLabelsFile gives an arbitrary file to WithPodinfoLabels.  The
// option must not panic, whatever the file holds.
func FuzzPodinfoLabelsFile(f *testing.F) {
	f.Add(`app="hello-world"`)
	f.Add("")
	f.Add("\n\n\n")
	f.Add("=")
	f.Add("a")
	f.Add(`a="unterminated`)
	f.Add("\xff=\xff")
	f.Add(`a="\q"`)

	f.Fuzz(func(t *testing.T, data string) {
		discardLogs(t)

		loadLabels(t, data)
	})
}
