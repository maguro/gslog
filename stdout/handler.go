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
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"sync"

	"m4o.io/gslog/core"
	"m4o.io/gslog/internal/entry"
	"m4o.io/gslog/internal/level"
	"m4o.io/gslog/internal/mapper"
	"m4o.io/gslog/internal/options"
	"m4o.io/gslog/stdout/internal/jbuf"
)

// Handler is a slog.Handler that writes each record to an io.Writer as one
// line of JSON.  The line has the structured logging format that the Google
// Cloud logging agent reads.
//
// The handler writes the fields that the agent reads.  The handler discards
// a top-level attribute or group that has the key of one of these fields:
//
//   - "severity", "message", and "timestamp"
//   - "logging.googleapis.com/labels"
//   - "logging.googleapis.com/sourceLocation"
//   - "logging.googleapis.com/spanId"
//   - "logging.googleapis.com/trace"
//   - "logging.googleapis.com/trace_sampled"
//
// The errorreporting.WithService option causes the handler to write the
// fields "@type", "serviceContext", and "stack_trace".  In a record that has
// these fields, the handler also discards a top-level attribute or group
// that has one of these three keys.  In a record that does not have these
// fields, the handler writes the attribute or group.
//
// For an attribute value that encoding/json cannot encode, the handler
// writes the string "!ERROR:" followed by the error text.
type Handler struct {
	out   *lineWriter
	level slog.Leveler

	addSource       bool
	entryAugmentors []entry.Augmentor
	errorReporter   *entry.Reporter
	replaceAttr     mapper.Mapper

	// groups holds the open group names.  prefix[i] holds the serialized
	// attributes at group level i, without a leading comma.  reportPrefix
	// holds the serialized top-level attributes with a key of the error
	// report, without a leading comma.
	groups       []string
	prefix       [][]byte
	reportPrefix []byte
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
		errorReporter:   o.ErrorReporter,
		replaceAttr:     mapper.Wrap(o.ReplaceAttr),

		groups:       nil,
		prefix:       [][]byte{nil},
		reportPrefix: nil,
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

	w := jbuf.AllocWriter()
	defer jbuf.ReleaseWriter(w)

	if report, ok := h.errorReporter.For(record.Level); ok {
		w.AppendErrorReport(&report)
	}

	h.appendHeader(w, &e, &record)
	h.appendBody(w, &record, e.Attrs)

	w.CloseLine()

	err := h.out.write(w.Buf())

	if err != nil {
		return fmt.Errorf("failed to write log entry: %w", err)
	}

	return nil
}

// WithAttrs returns a copy of the handler.  The copy serializes attrs once
// and writes the result with each record.  The copy writes a top-level
// attribute with a key of the error report only with a record that has no
// error report.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	depth := len(h.groups)
	prefix := slices.Clone(h.prefix[depth])
	w := jbuf.NewWriter(prefix)

	for _, a := range attrs {
		h.appendMapped(w, a)
	}

	handler2 := h.clone()
	handler2.prefix[depth] = w.Buf()
	handler2.reportPrefix = jbuf.JoinMembers(h.reportPrefix, w.ReportMembers())

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
		errorReporter:   h.errorReporter,
		replaceAttr:     h.replaceAttr,

		groups:       slices.Clip(h.groups),
		prefix:       slices.Clone(h.prefix),
		reportPrefix: h.reportPrefix,
	}
}

// appendHeader appends the severity, the message, and the fields that the
// agent reads.
func (h *Handler) appendHeader(w *jbuf.Writer, e *entry.Entry, record *slog.Record) {
	severity := level.ToSeverity(record.Level)
	w.AppendSeverity(severity)

	h.appendMessage(w, record)

	if !record.Time.IsZero() {
		w.AppendTimestamp(record.Time)
	}

	if len(e.Labels) > 0 {
		w.AppendLabels(e.Labels)
	}

	if e.Trace != "" {
		w.AppendTrace(e.Trace)
	}

	if e.SpanID != "" {
		w.AppendSpanID(e.SpanID)
	}

	if e.TraceSampled {
		w.AppendTraceSampled()
	}

	if e.Source != nil {
		w.AppendSourceLocation(e.Source)
	}
}

// appendMessage appends the message.  appendMessage does not write a renamed
// message when Member returns nil for the new key.
func (h *Handler) appendMessage(w *jbuf.Writer, record *slog.Record) {
	message := slog.String(core.MessageKey, record.Message)
	if h.replaceAttr != nil {
		message = h.replaceAttr(nil, message)
	}

	if message.Key != core.MessageKey && w.Member(message.Key) == nil {
		return
	}

	h.appendAttr(w, message, false)
}

// appendBody appends the serialized prefixes, the attributes of the record,
// and then extra.  Each group level is one nested object.  A group with no
// content at or below its level is not written.  appendBody does not write
// the groups when Member returns nil for the key of the top-level group.
func (h *Handler) appendBody(w *jbuf.Writer, record *slog.Record, extra []slog.Attr) {
	w.AppendRaw(h.prefix[0])

	if len(h.reportPrefix) > 0 && !w.HasReport() {
		w.AppendRaw(h.reportPrefix)
	}

	last := len(h.groups)
	if last > 0 && w.Member(h.groups[0]) == nil {
		return
	}

	for i := 1; i <= last; i++ {
		w.OpenGroup(h.groups[i-1])
		w.AppendRaw(h.prefix[i])
	}

	record.Attrs(func(a slog.Attr) bool {
		h.appendMapped(w, a)

		return true
	})

	for _, a := range extra {
		h.appendMapped(w, a)
	}

	for range last {
		w.CloseGroup()
	}
}

// appendMapped applies the mapper to the attribute and appends the result in
// the current group.
func (h *Handler) appendMapped(w *jbuf.Writer, a slog.Attr) {
	if h.replaceAttr != nil {
		a = h.replaceAttr(h.groups, a)
	}

	h.appendAttr(w, a, len(h.groups) == 0)
}

// appendAttr appends the attribute as a JSON member.  For a value that
// encoding/json cannot encode, the member is the string "!ERROR:" followed
// by the error text.  At the top level, appendAttr
// appends the attribute to the writer that Member returns for the key.
func (h *Handler) appendAttr(w *jbuf.Writer, a slog.Attr, topLevel bool) {
	v := a.Value.Resolve()

	if a.Key == "" && v.Any() == nil {
		return
	}

	if v.Kind() == slog.KindGroup {
		h.appendGroupAttr(w, a.Key, v.Group(), topLevel)

		return
	}

	if topLevel {
		w = w.Member(a.Key)
		if w == nil {
			return
		}
	}

	if v.Kind() == slog.KindAny {
		w.AppendAnyAttr(a.Key, v.Any())

		return
	}

	w.AppendSlogValue(a.Key, v)
}

// appendGroupAttr appends a group attribute as a nested object.  A group
// with an empty key is written in the current object.  At the top level,
// appendGroupAttr appends the group to the writer that Member returns for the
// key.
func (h *Handler) appendGroupAttr(w *jbuf.Writer, key string, attrs []slog.Attr, topLevel bool) {
	if key == "" {
		for _, a := range attrs {
			h.appendAttr(w, a, topLevel)
		}

		return
	}

	if topLevel {
		w = w.Member(key)
		if w == nil {
			return
		}
	}

	w.OpenGroup(key)

	for _, a := range attrs {
		h.appendAttr(w, a, false)
	}

	w.CloseGroup()
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
