// Copyright 2024 The original author or authors.
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
Package k8s contains options for including labels from the Kubernetes Downward
API podinfo labels file in logging records.

Placing the options in a separate package minimizes the dependencies pulled in
by those who do not need labels from the Kubernetes Downward API.
*/
package k8s

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"cloud.google.com/go/logging"
	"github.com/magiconair/properties"

	"m4o.io/gslog/internal/options"
)

const (
	// PodPrefix is the prefix for labels obtained from the Kubernetes
	// Downward API podinfo labels file.
	PodPrefix = "k8s-pod/"
)

// WithPodinfoLabels returns a Option that directs that the slog.Handler to
// include labels from the Kubernetes Downward API podinfo labels file.  The
// labels file is expected to be found in the directory specified by root and
// MUST be named "labels", per the Kubernetes Downward API for Pods.
//
// The labels are prefixed with "k8s-pod/" to adhere to the Google Cloud
// Logging conventions for Kubernetes Pod labels.
func WithPodinfoLabels(root string) options.OptionProcessor {
	return func(options *options.Options) {
		options.EntryAugmentors = append(options.EntryAugmentors, podinfoAugmentor(root))
	}
}

func podinfoAugmentor(root string) options.EntryAugmentor {
	path := filepath.Join(root, "labels")

	props, err := properties.LoadFile(path, properties.UTF8)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("Podinfo file does not exist", "path", path)
		} else {
			slog.Warn("Unable to load podinfo labels", "path", path, "error", err)
		}

		return func(_ context.Context, _ *logging.Entry, _ []string) {}
	}

	return func(_ context.Context, entry *logging.Entry, _ []string) {
		if entry.Labels == nil {
			entry.Labels = make(map[string]string)
		}

		for key, val := range props.Map() {
			if val[0] == '"' {
				val = val[1 : len(val)-1]
			}

			key = PodPrefix + key
			entry.Labels[key] = val
		}
	}
}
