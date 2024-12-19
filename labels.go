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

	"cloud.google.com/go/logging"

	"m4o.io/gslog/internal/options"
)

const (
	maxLabels = 64
)

// LabelPair represents a key-value string pair.
type LabelPair struct {
	valid  bool
	ignore bool
	key    string
	val    string
}

// IsIgnored indicates if there's something wrong with the label pair and that it
// will not be passed in the logging record.
func (lp LabelPair) IsIgnored() bool {
	return lp.ignore
}

// LogValue returns the slog.Value of the label pair.
func (lp LabelPair) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("key", lp.key),
		slog.String("value", lp.val))
}

// Label returns a new LabelPair from a key and a value.
func Label(key, value string) LabelPair {
	return LabelPair{valid: true, ignore: false, key: key, val: value}
}

type labelsKey struct{}

func doNothing(context.Context, *logging.Entry, []string) {}

// WithLabels returns a new Context with labels to be used in the GCP log
// entries produced using that context.
func WithLabels(ctx context.Context, labelPairs ...LabelPair) context.Context {
	parentLabelClosure := labelsEntryAugmentorFrom(ctx)

	return context.WithValue(ctx, labelsKey{},
		options.EntryAugmentor(func(ctx context.Context, entry *logging.Entry, groups []string) {
			parentLabelClosure(ctx, entry, groups)

			if entry.Labels == nil {
				entry.Labels = make(map[string]string)
			}

			for _, labelPair := range labelPairs {
				if labelPair.ignore {
					continue
				}

				if !labelPair.valid {
					panic("invalid label passed to WithLabels()")
				}

				if len(entry.Labels) >= maxLabels {
					slog.Error("Too many labels", "ignored", labelPair)

					continue
				}

				entry.Labels[labelPair.key] = labelPair.val
			}
		}),
	)
}

// ExtractLabels extracts labels from the ctx.  These labels were associated
// with the context using WithLabels.
func ExtractLabels(ctx context.Context) map[string]string {
	//nolint:exhaustruct
	entry := &logging.Entry{}
	labelsEntryAugmentorFrom(ctx)(ctx, entry, nil)

	return entry.Labels
}

// labelsEntryAugmentorFrom extracts the latest labelClosure from the context.
func labelsEntryAugmentorFrom(ctx context.Context) options.EntryAugmentor {
	v, ok := ctx.Value(labelsKey{}).(options.EntryAugmentor)
	if !ok {
		return doNothing
	}

	return v
}
