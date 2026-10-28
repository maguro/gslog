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
	"slices"
	"strings"
	"sync"
	"unicode/utf8"
)

// Entry holds the information for one log record that comes from the
// context and the options.  The record itself holds the message, the time,
// the level, and the attributes of the log call.
type Entry struct {
	Labels       Labels
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

// pool holds the entries that Alloc returns.
var pool = sync.Pool{
	New: func() any {
		return new(Entry)
	},
}

// Alloc returns an empty entry from the pool.  Call Release after the last
// use of the entry.
func Alloc() *Entry {
	e, _ := pool.Get().(*Entry)

	return e
}

// Release clears the entry and puts it in the pool.  Release keeps the
// capacity of Attrs for the next use.  Do not use the entry after the call.
func Release(e *Entry) {
	clear(e.Attrs)
	*e = Entry{Attrs: e.Attrs[:0]}

	pool.Put(e)
}

// Fill fills e for the record.  Fill sets the source location when addSource
// is true, calls each augmentor, and then adds the labels of ctx.
func Fill(ctx context.Context, record *slog.Record, addSource bool, augmentors []Augmentor, e *Entry) {
	if addSource {
		e.Source = record.Source()
	}

	for _, augment := range augmentors {
		augment(ctx, e)
	}

	AddLabels(ctx, e)
}

// Labels holds the labels of an entry.  Keys holds the keys of Map in sorted
// order.  Several entries can share one Labels value, so do not change Map or
// Keys after NewLabels returns them.
type Labels struct {
	Map  map[string]string
	Keys []string
}

// NewLabels returns the labels with the map and its sorted keys.
func NewLabels(m map[string]string) Labels {
	if len(m) == 0 {
		return Labels{}
	}

	keys := slices.Sorted(maps.Keys(m))

	return Labels{Map: m, Keys: keys}
}

// Len returns the number of labels.
func (l Labels) Len() int {
	return len(l.Keys)
}

// Merge returns the labels of l and of other.  A label of other replaces a
// label of l with the same key.  Merge returns l when other has no label, and
// other when l has no label.
func (l Labels) Merge(other Labels) Labels {
	if other.Len() == 0 {
		return l
	}

	if l.Len() == 0 {
		return other
	}

	m := make(map[string]string, len(l.Map)+len(other.Map))
	maps.Copy(m, l.Map)
	maps.Copy(m, other.Map)

	keys := mergeKeys(l.Keys, other.Keys)

	return Labels{Map: m, Keys: keys}
}

// mergeKeys returns the keys of two sorted lists in one sorted list.  A key
// that is in both lists is in the result once.
func mergeKeys(a, b []string) []string {
	keys := make([]string, 0, len(a)+len(b))

	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] < b[j]:
			keys = append(keys, a[i])
			i++
		case a[i] > b[j]:
			keys = append(keys, b[j])
			j++
		default:
			keys = append(keys, a[i])
			i++
			j++
		}
	}

	keys = append(keys, a[i:]...)

	return append(keys, b[j:]...)
}

type labelsKey struct{}

// WithLabels returns a new Context that stores the labels.
func WithLabels(ctx context.Context, labels Labels) context.Context {
	return context.WithValue(ctx, labelsKey{}, labels)
}

// LabelsFrom returns the labels stored in ctx by WithLabels.
func LabelsFrom(ctx context.Context) Labels {
	labels, _ := ctx.Value(labelsKey{}).(Labels)

	return labels
}

// AddLabels adds the labels of ctx to the entry.  A label of ctx replaces a
// label of the entry with the same key.
func AddLabels(ctx context.Context, e *Entry) {
	labels := LabelsFrom(ctx)

	e.Labels = e.Labels.Merge(labels)
}

// ValidUTF8 returns s when s is valid UTF-8.  Otherwise, ValidUTF8 returns a
// copy of s in which each byte that is not valid UTF-8 is the replacement
// character U+FFFD.  encoding/json makes the same replacement.
func ValidUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}

	var b strings.Builder

	b.Grow(len(s))

	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b.WriteRune(utf8.RuneError)
		} else {
			b.WriteString(s[i : i+size])
		}

		i += size
	}

	return b.String()
}
