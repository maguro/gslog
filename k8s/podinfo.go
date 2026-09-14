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

/*
Package k8s contains options that include labels from the Kubernetes Downward
API podinfo labels file in logging records.

The options are in a separate package.  Because of this, a module that does
not need labels from the Kubernetes Downward API has fewer dependencies.
*/
package k8s

import (
	"context"
	"log/slog"
	"maps"
	"os"
	"path/filepath"

	"github.com/magiconair/properties"

	"m4o.io/gslog/core"
	"m4o.io/gslog/internal/entry"
	"m4o.io/gslog/internal/options"
)

const (
	// PodPrefix is the prefix for labels that come from the Kubernetes
	// Downward API podinfo labels file.
	PodPrefix = "k8s-pod/"
)

// WithPodinfoLabels returns an option that causes the handler to include
// labels from the Kubernetes Downward API podinfo labels file.  The handler
// expects the labels file in the directory that root specifies.  The file
// must be named "labels", as the Kubernetes Downward API for Pods specifies.
//
// The handler adds the prefix "k8s-pod/" to each label.  This follows the
// Google Cloud Logging conventions for Kubernetes Pod labels.
func WithPodinfoLabels(root string) core.Option {
	return func(options *options.Options) {
		options.EntryAugmentors = append(options.EntryAugmentors, podinfoAugmentor(root))
	}
}

func podinfoAugmentor(root string) entry.Augmentor {
	path := filepath.Join(root, "labels")

	props, err := properties.LoadFile(path, properties.UTF8)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("Podinfo file does not exist", "path", path)
		} else {
			slog.Warn("Unable to load podinfo labels", "path", path, "error", err)
		}

		return func(_ context.Context, _ *entry.Entry) {}
	}

	labels := podLabels(props)

	return func(_ context.Context, e *entry.Entry) {
		if e.Labels == nil {
			e.Labels = make(map[string]string, len(labels))
		}

		maps.Copy(e.Labels, labels)
	}
}

// podLabels returns the labels in props.  The function removes the quotes
// from each value and adds PodPrefix to each key.
func podLabels(props *properties.Properties) map[string]string {
	labels := make(map[string]string, props.Len())

	for key, val := range props.Map() {
		if val[0] == '"' {
			val = val[1 : len(val)-1]
		}

		labels[PodPrefix+key] = val
	}

	return labels
}
