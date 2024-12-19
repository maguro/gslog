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
)

// LabelPair represents a key-value string pair.
type LabelPair struct {
	valid bool
	key   string
	val   string
}

// Label returns a new LabelPair from a key and a value.
func Label(key, value string) LabelPair {
	return LabelPair{valid: true, key: key, val: value}
}

type labelsKey struct{}

type labeler func(ctx context.Context, labels map[string]string)

func doNothing(context.Context, map[string]string) {}

// WithLabels returns a new Context with labels to be used in the GCP log
// entries produced using that context.
func WithLabels(ctx context.Context, labelPairs ...LabelPair) context.Context {
	parent := labelsFrom(ctx)

	return context.WithValue(ctx, labelsKey{},
		labeler(func(ctx context.Context, labels map[string]string) {
			parent(ctx, labels)

			for _, labelPair := range labelPairs {
				if !labelPair.valid {
					panic("invalid label passed to WithLabels()")
				}

				labels[labelPair.key] = labelPair.val
			}
		}),
	)
}

// ExtractLabels extracts labels from the ctx.  These labels were associated
// with the context using WithLabels.
func ExtractLabels(ctx context.Context) map[string]string {
	labels := make(map[string]string)

	labeler := labelsFrom(ctx)
	labeler(ctx, labels)

	return labels
}

func labelsFrom(ctx context.Context) labeler {
	v, ok := ctx.Value(labelsKey{}).(labeler)
	if !ok {
		return doNothing
	}

	return v
}
