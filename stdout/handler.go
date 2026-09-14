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

package stdout

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"

	"m4o.io/gslog/core"
	"m4o.io/gslog/internal/entry"
	"m4o.io/gslog/internal/level"
	"m4o.io/gslog/internal/mapper"
	"m4o.io/gslog/internal/options"
	"m4o.io/gslog/internal/timefmt"
)

// The keys that the Google Cloud logging agent reads from a JSON line.
const (
	agentSeverityKey       = "severity"
	agentTimestampKey      = "timestamp"
	agentLabelsKey         = "logging.googleapis.com/labels"
	agentSourceLocationKey = "logging.googleapis.com/sourceLocation"
	agentSpanIDKey         = "logging.googleapis.com/spanId"
	agentTraceKey          = "logging.googleapis.com/trace"
	agentTraceSampledKey   = "logging.googleapis.com/trace_sampled"
)

const (
	initialBufferSize = 1024
	maxPooledBuffer   = 16 << 10
)

// Handler is a slog.Handler that writes each record to an io.Writer as one
// line of JSON.  The line has the structured logging format that the Google
// Cloud logging agent reads.
//
// A top-level attribute or group with the same key as a field that the agent
// reads, such as "severity" or "message", is not written.
type Handler struct {
	out   *lineWriter
	level slog.Leveler

	addSource       bool
	entryAugmentors []entry.Augmentor
	replaceAttr     mapper.Mapper

	// groups holds the open group names.  prefix[i] holds the serialized
	// attributes at group level i, without a leading comma.
	groups []string
	prefix [][]byte
}

var _ slog.Handler = (*Handler)(nil)

// NewHandler creates a Handler that writes to w.  If w is nil, the handler
// writes to os.Stdout.
func NewHandler(w io.Writer, opts ...core.Option) *Handler {
	if w == nil {
		w = os.Stdout
	}

	o := options.ApplyOptions(opts...)

	return &Handler{
		out:   &lineWriter{w: w, mu: sync.Mutex{}},
		level: o.Level,

		addSource:       o.AddSource,
		entryAugmentors: o.EntryAugmentors,
		replaceAttr:     mapper.Wrap(o.ReplaceAttr),

		groups: nil,
		prefix: [][]byte{nil},
	}
}

// Enabled reports whether the handler handles records at the given level.
// The handler ignores a record that has a lower level.
func (h *Handler) Enabled(_ context.Context, l slog.Level) bool {
	return h.level.Level() <= l
}

// Handle writes the record as one line of JSON.  Handle returns the error
// from the writer.
func (h *Handler) Handle(ctx context.Context, record slog.Record) error {
	e := entry.Fill(ctx, &record, h.addSource, h.entryAugmentors)

	w, _ := writerPool.Get().(*jsonWriter)
	w.reset()

	h.appendHeader(w, &e, &record)
	h.appendBody(w, &record, e.Attrs)

	w.buf = append(w.buf, '}', '\n')

	err := h.out.write(w.buf)

	if cap(w.buf) <= maxPooledBuffer {
		writerPool.Put(w)
	}

	if err != nil {
		return fmt.Errorf("failed to write log entry: %w", err)
	}

	return nil
}

// WithAttrs returns a copy of the handler.  The copy serializes attrs once
// and writes the result with each record.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	depth := len(h.groups)
	w := jsonWriter{buf: slices.Clone(h.prefix[depth]), first: len(h.prefix[depth]) == 0}

	for _, a := range attrs {
		h.appendMapped(&w, a)
	}

	handler2 := h.clone()
	handler2.prefix[depth] = w.buf

	return handler2
}

// WithGroup returns a copy of the handler.  The groups of the copy are the
// groups of h followed by name.
func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	handler2 := h.clone()
	handler2.groups = append(handler2.groups, name)
	handler2.prefix = append(handler2.prefix, nil)

	return handler2
}

// clone returns a copy of the handler.  The copy shares the serialized
// prefixes of h.
func (h *Handler) clone() *Handler {
	return &Handler{
		out:   h.out,
		level: h.level,

		addSource:       h.addSource,
		entryAugmentors: h.entryAugmentors,
		replaceAttr:     h.replaceAttr,

		groups: slices.Clip(h.groups),
		prefix: slices.Clone(h.prefix),
	}
}

// appendHeader appends the severity, the message, and the fields that the
// agent reads.
func (h *Handler) appendHeader(w *jsonWriter, e *entry.Entry, record *slog.Record) {
	severity := level.ToSeverity(record.Level)
	name := severity.String()

	w.key(agentSeverityKey)
	w.buf = appendString(w.buf, name)

	h.appendMessage(w, record)

	if !record.Time.IsZero() {
		w.key(agentTimestampKey)
		w.buf = append(w.buf, '"')
		w.buf = record.Time.UTC().AppendFormat(w.buf, time.RFC3339Nano)
		w.buf = append(w.buf, '"')
	}

	if len(e.Labels) > 0 {
		w.key(agentLabelsKey)
		w.buf = appendLabels(w.buf, e.Labels)
	}

	if e.Trace != "" {
		w.key(agentTraceKey)
		w.buf = appendString(w.buf, e.Trace)
	}

	if e.SpanID != "" {
		w.key(agentSpanIDKey)
		w.buf = appendString(w.buf, e.SpanID)
	}

	if e.TraceSampled {
		w.key(agentTraceSampledKey)
		w.buf = append(w.buf, "true"...)
	}

	if e.Source != nil {
		w.key(agentSourceLocationKey)
		w.buf = appendSourceLocation(w.buf, e.Source)
	}
}

// appendMessage appends the message.  The handler does not write a renamed
// message with a key that the agent reads.
func (h *Handler) appendMessage(w *jsonWriter, record *slog.Record) {
	message := slog.String(core.MessageKey, record.Message)
	if h.replaceAttr != nil {
		message = h.replaceAttr(nil, message)
	}

	if message.Key != core.MessageKey && isAgentKey(message.Key) {
		return
	}

	h.appendAttr(w, message, false)
}

// appendBody appends the serialized prefixes, the attributes of the record,
// and then extra.  Each group level is one nested object.  A group with no
// content at or below its level is not written.  A top-level group with a
// key that the agent reads is not written.
func (h *Handler) appendBody(w *jsonWriter, record *slog.Record, extra []slog.Attr) {
	w.raw(h.prefix[0])

	last := len(h.groups)
	if last > 0 && isAgentKey(h.groups[0]) {
		return
	}

	for i := 1; i <= last; i++ {
		w.openGroup(h.groups[i-1])
		w.raw(h.prefix[i])
	}

	record.Attrs(func(a slog.Attr) bool {
		h.appendMapped(w, a)

		return true
	})

	for _, a := range extra {
		h.appendMapped(w, a)
	}

	for range last {
		w.closeGroup()
	}
}

// appendMapped applies the mapper to the attribute and appends the result in
// the current group.
func (h *Handler) appendMapped(w *jsonWriter, a slog.Attr) {
	if h.replaceAttr != nil {
		a = h.replaceAttr(h.groups, a)
	}

	h.appendAttr(w, a, len(h.groups) == 0)
}

// appendAttr appends the attribute as a JSON member.  An attribute that
// cannot be written as JSON is not written.  At the top level, an attribute
// with a key that the agent reads is not written.
func (h *Handler) appendAttr(w *jsonWriter, a slog.Attr, topLevel bool) {
	v := a.Value.Resolve()

	if a.Key == "" && v.Any() == nil {
		return
	}

	if v.Kind() == slog.KindGroup {
		h.appendGroupAttr(w, a.Key, v.Group(), topLevel)

		return
	}

	if topLevel && isAgentKey(a.Key) {
		return
	}

	if v.Kind() == slog.KindAny {
		appendAnyAttr(w, a.Key, v.Any())

		return
	}

	w.key(a.Key)
	w.buf = appendSlogValue(w.buf, v)
}

// appendGroupAttr appends a group attribute as a nested object.  A group
// with an empty key is written in the current object.
func (h *Handler) appendGroupAttr(w *jsonWriter, key string, attrs []slog.Attr, topLevel bool) {
	if key == "" {
		for _, a := range attrs {
			h.appendAttr(w, a, topLevel)
		}

		return
	}

	if topLevel && isAgentKey(key) {
		return
	}

	w.openGroup(key)

	for _, a := range attrs {
		h.appendAttr(w, a, false)
	}

	w.closeGroup()
}

// lineWriter writes whole lines to an io.Writer.  A mutex keeps the lines
// of concurrent writers separate.
type lineWriter struct {
	mu sync.Mutex
	w  io.Writer
}

// write writes one line.
func (l *lineWriter) write(line []byte) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	_, err := l.w.Write(line)

	return err
}

// jsonWriter appends JSON members to a buffer.  first is true when the next
// member is the first one in the open object.  A group becomes a nested
// object when the writer appends the first member of the group.  pending
// holds the keys of the groups that have no member yet.
type jsonWriter struct {
	buf     []byte
	first   bool
	pending []string
}

// reset starts a new top-level object.
func (w *jsonWriter) reset() {
	w.buf = append(w.buf[:0], '{')
	w.first = true
	w.pending = w.pending[:0]
}

// key appends the separator, if needed, and the key.
func (w *jsonWriter) key(key string) {
	w.openPending()

	if !w.first {
		w.buf = append(w.buf, ',')
	}

	w.first = false
	w.buf = appendKey(w.buf, key)
}

// raw appends serialized members.  An empty slice appends nothing.
func (w *jsonWriter) raw(members []byte) {
	if len(members) == 0 {
		return
	}

	w.openPending()

	if !w.first {
		w.buf = append(w.buf, ',')
	}

	w.first = false
	w.buf = append(w.buf, members...)
}

// openGroup starts a group.  The writer appends the key and the nested
// object when it appends the first member of the group.
func (w *jsonWriter) openGroup(key string) {
	w.pending = append(w.pending, key)
}

// closeGroup ends the group.  The writer appends nothing for a group with
// no member.
func (w *jsonWriter) closeGroup() {
	if n := len(w.pending); n > 0 {
		w.pending = w.pending[:n-1]

		return
	}

	w.buf = append(w.buf, '}')
	w.first = false
}

// openPending appends the key and the nested object of each pending group.
func (w *jsonWriter) openPending() {
	for _, key := range w.pending {
		if !w.first {
			w.buf = append(w.buf, ',')
		}

		w.buf = appendKey(w.buf, key)
		w.buf = append(w.buf, '{')
		w.first = true
	}

	w.pending = w.pending[:0]
}

var writerPool = sync.Pool{
	New: func() any {
		return &jsonWriter{buf: make([]byte, 0, initialBufferSize), first: true}
	},
}

// appendAnyAttr appends an attribute of kind slog.KindAny.  A nil value is
// null.  An error that does not implement json.Marshaler is written as its
// Error() string.  All other values are encoded with encoding/json.
func appendAnyAttr(w *jsonWriter, key string, value any) {
	if value == nil {
		w.key(key)
		w.buf = append(w.buf, "null"...)

		return
	}

	_, marshaler := value.(json.Marshaler)
	if err, ok := value.(error); ok && !marshaler {
		w.key(key)
		w.buf = appendString(w.buf, err.Error())

		return
	}

	data, err := json.Marshal(value)
	if err != nil {
		return
	}

	w.key(key)
	w.buf = append(w.buf, data...)
}

// appendSlogValue appends a resolved slog.Value that is not a group and not
// of kind slog.KindAny.
func appendSlogValue(buf []byte, v slog.Value) []byte {
	switch v.Kind() {
	case slog.KindString:
		return appendString(buf, v.String())
	case slog.KindInt64:
		return strconv.AppendInt(buf, v.Int64(), decimalBase)
	case slog.KindUint64:
		return strconv.AppendUint(buf, v.Uint64(), decimalBase)
	case slog.KindFloat64:
		return appendNumber(buf, v.Float64())
	case slog.KindBool:
		return strconv.AppendBool(buf, v.Bool())
	case slog.KindDuration:
		return strconv.AppendInt(buf, int64(v.Duration()), decimalBase)
	case slog.KindTime:
		formatted := timefmt.RFC3339InMs(v.Time())

		return appendString(buf, formatted)
	default:
		return append(buf, "null"...)
	}
}

// isAgentKey reports whether the agent reads the key.
func isAgentKey(key string) bool {
	switch key {
	case core.MessageKey, agentSeverityKey, agentTimestampKey, agentLabelsKey, agentSourceLocationKey,
		agentSpanIDKey, agentTraceKey, agentTraceSampledKey:
		return true
	default:
		return false
	}
}
