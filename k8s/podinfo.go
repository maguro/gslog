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

// Package k8s contains options that include labels from the Kubernetes
// Downward API podinfo labels file in logging records.
//
// The options are in a separate package.  Because of this, a module that does
// not need labels from the Kubernetes Downward API has fewer dependencies.
package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"m4o.io/gslog"
	"m4o.io/gslog/internal/entry"
	"m4o.io/gslog/internal/options"
)

const (
	// PodPrefix is the prefix for labels that come from the Kubernetes
	// Downward API podinfo labels file.
	PodPrefix = "k8s-pod/"

	// separator divides the key from the value on a line of the podinfo
	// labels file.
	separator = "="
)

// WithPodinfoLabels returns an option that causes the handler to include
// labels from the Kubernetes Downward API podinfo labels file.  The handler
// expects the labels file in the directory that root specifies.  The file
// must be named "labels", as the Kubernetes Downward API for Pods specifies.
// If the options have more than one WithPodinfoLabels option, the handler
// uses the last WithPodinfoLabels option.
//
// WithPodinfoLabels reads the file once, when the handler applies the
// option.  A later change to the file has no effect.  If WithPodinfoLabels
// cannot read the file, it logs a warning with the default slog.Logger, and
// the option has no effect.
//
// The handler adds the prefix "k8s-pod/" to each label.  This follows the
// Google Cloud Logging conventions for Kubernetes Pod labels.
func WithPodinfoLabels(root string) gslog.Option {
	return func(options *options.Options) {
		augment := podinfoAugmentor(root)

		options.AddAugmentor("k8s.WithPodinfoLabels", augment)
	}
}

func podinfoAugmentor(root string) entry.Augmentor {
	path := filepath.Join(root, "labels")

	labels, err := podLabels(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("Podinfo file does not exist", "path", path)
		} else {
			slog.Warn("Unable to load podinfo labels", "path", path, "error", err)
		}

		return func(_ context.Context, _ *entry.Entry) {}
	}

	return func(_ context.Context, e *entry.Entry) {
		e.Labels = e.Labels.Merge(labels)
	}
}

// podLabels returns the labels in the podinfo labels file at path.  podLabels
// adds PodPrefix to each key.  podLabels replaces a byte that is not valid
// UTF-8 with U+FFFD.
//
// The Kubernetes Downward API writes one label per line, in the form
// key="value", where the value is a Go quoted string.  podLabels removes
// the whitespace around the key and the value.  A line that has no separator
// makes the whole file invalid, because a partial set of labels is more
// misleading than no labels.
func podLabels(path string) (entry.Labels, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return entry.Labels{}, err
	}

	labels := make(map[string]string)

	lineNo := 0

	for line := range strings.SplitSeq(string(data), "\n") {
		lineNo++

		if strings.TrimSpace(line) == "" {
			continue
		}

		key, val, found := strings.Cut(line, separator)
		if !found {
			return entry.Labels{}, fmt.Errorf("line %d has no %q separator", lineNo, separator)
		}

		// The quotes bound the value, so a trim cannot remove a space that
		// belongs to the value.
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)

		labels[PodPrefix+entry.ValidUTF8(key)] = entry.ValidUTF8(unquote(val))
	}

	return entry.NewLabels(labels), nil
}

// unquote removes the quotes from a Go quoted string.  unquote also expands
// the escape sequences of the string.  If val is not a Go quoted string,
// unquote returns val unchanged, because such a value is still usable as a
// label.
func unquote(val string) string {
	if n := len(val); n < 2 || val[0] != '"' || val[n-1] != '"' {
		return val
	}

	unquoted, err := strconv.Unquote(val)
	if err != nil {
		return val
	}

	return unquoted
}
