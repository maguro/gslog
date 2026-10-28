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

package gslog

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
	valid bool
	key   string
	val   string
}

// Label returns a new LabelPair from a key and a value.  Label panics if key
// is empty.
func Label(key, value string) LabelPair {
	if key == "" {
		panic("label key is empty")
	}

	return LabelPair{valid: true, key: key, val: value}
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
// ctx replaces the label of ctx.  WithLabels panics if a label pair is the
// zero LabelPair.
func WithLabels(ctx context.Context, labelPairs ...LabelPair) context.Context {
	parent := entry.LabelsFrom(ctx)
	labels := make(map[string]string, len(parent)+len(labelPairs))

	maps.Copy(labels, parent)

	for _, labelPair := range labelPairs {
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

// LabelsFrom returns the labels that WithLabels stored in ctx.  The
// returned map is a copy.
func LabelsFrom(ctx context.Context) map[string]string {
	labels := entry.LabelsFrom(ctx)

	return maps.Clone(labels)
}
