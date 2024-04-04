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

package gslog

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"cloud.google.com/go/logging"
	"github.com/magiconair/properties"
)

const (
	K8sPodPrefix = "k8s-pod/"
)

// WithPodinfoLabels returns a Option that directs that the slog.Handler to
// include labels from the Kubernetes Downward API podinfo labels file.  The
// labels file is expected to be found in the directory specified by root and
// MUST be named "labels", per the Kubernetes Downward API for Pods.
//
// The labels are prefixed with "k8s-pod/" to adhere to the Google Cloud
// Logging conventions for Kubernetes Pod labels.
func WithPodinfoLabels(root string) Option {
	return OptionFunc(func(options *Options) error {
		options.EntryAugmentors = append(options.EntryAugmentors, podinfoAugmentor(root))
		return nil
	})
}

func podinfoAugmentor(root string) func(ctx context.Context, e *logging.Entry) {
	return func(ctx context.Context, e *logging.Entry) {
		if e.Labels == nil {
			e.Labels = make(map[string]string)
		}

		path := filepath.Join(root, "labels")
		p, err := properties.LoadFile(path, properties.UTF8)
		if err != nil {
			if os.IsNotExist(err) {
				slog.Warn("Podinfo file does not exist", "path", path)
			} else {
				slog.Warn("Unable to load podinfo labels", "path", path, "error", err)
			}
			return
		}

		for k, v := range p.Map() {
			if v[0] == '"' {
				v = v[1 : len(v)-1]
			}

			key := K8sPodPrefix + k
			e.Labels[key] = v
		}
	}
}
