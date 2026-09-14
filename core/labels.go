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

package core

import (
	"context"
	"log/slog"
	"maps"

	"m4o.io/gslog/internal/entry"
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

// Label returns a new LabelPair from a key and a value.
func Label(key, value string) LabelPair {
	return LabelPair{valid: true, ignore: false, key: key, val: value}
}

// IsIgnored reports whether there is a problem with the label pair.  The
// handler does not add an ignored label pair to the logging record.
func (lp LabelPair) IsIgnored() bool {
	return lp.ignore
}

// LogValue returns the slog.Value of the label pair.
func (lp LabelPair) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("key", lp.key),
		slog.String("value", lp.val))
}

// WithLabels returns a new Context that has the labels of ctx and the
// supplied labels.  The handler adds these labels to each log entry that it
// makes with that context.  A supplied label with the same key as a label of
// ctx replaces the label of ctx.  The function panics if a label pair is not
// valid.
func WithLabels(ctx context.Context, labelPairs ...LabelPair) context.Context {
	parent := entry.LabelsFrom(ctx)
	labels := make(map[string]string, len(parent)+len(labelPairs))

	maps.Copy(labels, parent)

	for _, labelPair := range labelPairs {
		if labelPair.ignore {
			continue
		}

		if !labelPair.valid {
			panic("invalid label passed to WithLabels()")
		}

		if len(labels) >= maxLabels {
			slog.Error("Too many labels", "ignored", labelPair)

			continue
		}

		labels[labelPair.key] = labelPair.val
	}

	return entry.WithLabels(ctx, labels)
}

// ExtractLabels returns the labels that WithLabels stored in ctx.  The
// returned map is a copy.
func ExtractLabels(ctx context.Context) map[string]string {
	labels := entry.LabelsFrom(ctx)

	return maps.Clone(labels)
}
