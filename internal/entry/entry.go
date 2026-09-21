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

package entry

import (
	"context"
	"log/slog"
	"maps"
)

// Entry holds the information for one log record that comes from the
// context and the options.  The record itself holds the message, the time,
// the level, and the attributes of the log call.
type Entry struct {
	Labels       map[string]string
	Trace        string
	SpanID       string
	TraceSampled bool
	Source       *slog.Source

	// Attrs holds the attributes that the augmentors add.  A handler writes
	// them after the attributes of the log call.
	Attrs []slog.Attr
}

// Augmentor sets fields of an entry from the context.
type Augmentor func(ctx context.Context, e *Entry)

// Fill returns the entry for the record.  Fill sets the source location when
// addSource is true, calls each augmentor, and then adds the labels of ctx.
func Fill(ctx context.Context, record *slog.Record, addSource bool, augmentors []Augmentor) Entry {
	var e Entry

	if addSource {
		e.Source = record.Source()
	}

	for _, augment := range augmentors {
		augment(ctx, &e)
	}

	AddLabels(ctx, &e)

	return e
}

type labelsKey struct{}

// WithLabels returns a new Context that stores the labels.  The map must not
// change after the call.
func WithLabels(ctx context.Context, labels map[string]string) context.Context {
	return context.WithValue(ctx, labelsKey{}, labels)
}

// LabelsFrom returns the labels stored in ctx by WithLabels.
func LabelsFrom(ctx context.Context) map[string]string {
	labels, _ := ctx.Value(labelsKey{}).(map[string]string)

	return labels
}

// AddLabels adds the labels of ctx to the entry.  If the entry has no
// labels, AddLabels gives the entry the map from ctx.
func AddLabels(ctx context.Context, e *Entry) {
	labels := LabelsFrom(ctx)
	if len(labels) == 0 {
		return
	}

	if e.Labels == nil {
		e.Labels = labels

		return
	}

	maps.Copy(e.Labels, labels)
}
