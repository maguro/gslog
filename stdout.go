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
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"cloud.google.com/go/logging"
	logpb "cloud.google.com/go/logging/apiv2/loggingpb"
	"github.com/pkg/errors"
	spb "google.golang.org/protobuf/types/known/structpb"

	"m4o.io/gslog/internal/attr"
	"m4o.io/gslog/internal/level"
	"m4o.io/gslog/internal/options"
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
	hexDigits         = "0123456789abcdef"
	unicodeEscapeLen  = 4
	decimalBase       = 10
	float64Bits       = 64
)

// StdoutHandler is a slog.Handler that writes each record to an io.Writer
// as one line of JSON in the structured logging format that the Google
// Cloud logging agent reads.
//
// A top-level attribute with the same key as a field that the agent reads,
// such as "severity" or "message", is not written.
type StdoutHandler struct {
	out   *lineWriter
	level slog.Leveler

	addSource       bool
	entryAugmentors []options.EntryAugmentor
	replaceAttr     attr.Mapper

	// groups holds the open group names.  prefix[i] holds the serialized
	// attributes at group level i, without a leading comma.
	groups []string
	prefix [][]byte
}

var _ slog.Handler = (*StdoutHandler)(nil)

// NewStdoutHandler creates a StdoutHandler that writes to w.  If w is nil,
// the handler writes to os.Stdout.
func NewStdoutHandler(w io.Writer, opts ...options.OptionProcessor) *StdoutHandler {
	if w == nil {
		w = os.Stdout
	}

	o := options.ApplyOptions(opts...)

	return &StdoutHandler{
		out:   &lineWriter{w: w, mu: sync.Mutex{}},
		level: o.Level,

		addSource:       o.AddSource,
		entryAugmentors: o.EntryAugmentors,
		replaceAttr:     attr.WrapAttrMapper(o.ReplaceAttr),

		groups: nil,
		prefix: [][]byte{nil},
	}
}

// Enabled reports whether the handler handles records at the given level.
// The handler ignores a record that has a lower level.
func (h *StdoutHandler) Enabled(_ context.Context, l slog.Level) bool {
	return h.level.Level() <= l
}

// Handle writes the record as one line of JSON.  Handle returns the error
// from the writer.
func (h *StdoutHandler) Handle(ctx context.Context, record slog.Record) error {
	var entry logging.Entry

	h.fillEntry(ctx, &entry, &record)

	w, _ := writerPool.Get().(*jsonWriter)
	w.reset()

	h.appendHeader(w, &entry, &record)

	payload, _ := entry.Payload.(*spb.Struct)
	h.appendBody(w, payload, &record)

	w.buf = append(w.buf, '}', '\n')

	err := h.out.write(w.buf)

	if cap(w.buf) <= maxPooledBuffer {
		writerPool.Put(w)
	}

	if err != nil {
		return errors.Wrap(err, "failed to write log entry")
	}

	return nil
}

// WithAttrs returns a copy of the handler.  The copy serializes attrs once
// and writes the result with each record.
func (h *StdoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	depth := len(h.groups)
	w := jsonWriter{buf: slices.Clone(h.prefix[depth]), first: len(h.prefix[depth]) == 0}

	for _, a := range attrs {
		if h.replaceAttr != nil {
			a = h.replaceAttr(h.groups, a)
		}

		h.appendAttr(&w, a, depth == 0)
	}

	handler2 := h.clone()
	handler2.prefix[depth] = w.buf

	return handler2
}

// WithGroup returns a copy of the handler.  The groups of the copy are the
// groups of h followed by name.
func (h *StdoutHandler) WithGroup(name string) slog.Handler {
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
func (h *StdoutHandler) clone() *StdoutHandler {
	return &StdoutHandler{
		out:   h.out,
		level: h.level,

		addSource:       h.addSource,
		entryAugmentors: h.entryAugmentors,
		replaceAttr:     h.replaceAttr,

		groups: slices.Clip(h.groups),
		prefix: slices.Clone(h.prefix),
	}
}

// fillEntry sets the fields of the entry that the agent reads.  The entry
// gets a payload only if the handler has entry augmentors.
func (h *StdoutHandler) fillEntry(ctx context.Context, entry *logging.Entry, record *slog.Record) {
	entry.Timestamp = record.Time.UTC()
	entry.Severity = level.ToSeverity(record.Level)

	if h.addSource {
		addSourceLocation(entry, record)
	}

	if len(h.entryAugmentors) > 0 {
		entry.Payload = &spb.Struct{Fields: make(map[string]*spb.Value)}

		for _, b := range h.entryAugmentors {
			b(ctx, entry, h.groups)
		}
	}

	addLabels(ctx, entry)
}

// appendHeader appends the severity, the message, and the fields that the
// agent reads.
func (h *StdoutHandler) appendHeader(w *jsonWriter, entry *logging.Entry, record *slog.Record) {
	w.key(agentSeverityKey)
	w.buf = appendString(w.buf, severityName(entry.Severity))

	message := slog.String(MessageKey, record.Message)
	if h.replaceAttr != nil {
		message = h.replaceAttr(nil, message)
	}

	// The handler does not write a renamed message with a key that the agent reads.
	h.appendAttr(w, message, message.Key != MessageKey)

	if !entry.Timestamp.IsZero() {
		w.key(agentTimestampKey)
		w.buf = append(w.buf, '"')
		w.buf = entry.Timestamp.AppendFormat(w.buf, time.RFC3339Nano)
		w.buf = append(w.buf, '"')
	}

	if len(entry.Labels) > 0 {
		w.key(agentLabelsKey)
		w.buf = appendLabels(w.buf, entry.Labels)
	}

	if entry.Trace != "" {
		w.key(agentTraceKey)
		w.buf = appendString(w.buf, entry.Trace)
	}

	if entry.SpanID != "" {
		w.key(agentSpanIDKey)
		w.buf = appendString(w.buf, entry.SpanID)
	}

	if entry.TraceSampled {
		w.key(agentTraceSampledKey)
		w.buf = append(w.buf, "true"...)
	}

	if entry.SourceLocation != nil {
		w.key(agentSourceLocationKey)
		w.buf = appendSourceLocation(w.buf, entry.SourceLocation)
	}
}

// appendBody appends the serialized prefixes, the payload fields, and the
// attributes of the record.  Each group level is one nested object.  A group
// with no content at or below its level is not written.
func (h *StdoutHandler) appendBody(w *jsonWriter, payload *spb.Struct, record *slog.Record) {
	last := len(h.groups)
	current := payload

	for i := 0; i <= last; i++ {
		if i > 0 {
			w.openGroup(h.groups[i-1])
		}

		w.raw(h.prefix[i])

		if i < last {
			appendPayloadLevel(w, current, h.groups[i], i == 0)
			current = current.GetFields()[h.groups[i]].GetStructValue()

			continue
		}

		appendPayloadLevel(w, current, "", i == 0)
		h.appendRecordAttrs(w, record, i == 0)
	}

	for range last {
		w.closeGroup()
	}
}

// appendRecordAttrs appends the attributes of the record.
func (h *StdoutHandler) appendRecordAttrs(w *jsonWriter, record *slog.Record, topLevel bool) {
	record.Attrs(func(a slog.Attr) bool {
		if h.replaceAttr != nil {
			a = h.replaceAttr(h.groups, a)
		}

		h.appendAttr(w, a, topLevel)

		return true
	})
}

// appendAttr appends the attribute as a JSON member.  An attribute that
// cannot be written as JSON is not written.  At the top level, an attribute
// with a key that the agent reads is not written.
func (h *StdoutHandler) appendAttr(w *jsonWriter, a slog.Attr, topLevel bool) {
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
func (h *StdoutHandler) appendGroupAttr(w *jsonWriter, key string, attrs []slog.Attr, topLevel bool) {
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

// appendAnyAttr appends an attribute of kind slog.KindAny.  An error that
// does not implement json.Marshaler is written as its Error() string.  All
// other values are encoded with encoding/json.
func appendAnyAttr(w *jsonWriter, key string, value any) {
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
		return appendString(buf, attr.TimeToRFC3339InMs(v.Time()))
	case slog.KindAny, slog.KindGroup, slog.KindLogValuer:
		return append(buf, "null"...)
	default:
		return append(buf, "null"...)
	}
}

// appendPayloadLevel appends the fields of the struct, except the field with
// the key skip.  At the top level, a field with a key that the agent reads
// is not written.
func appendPayloadLevel(w *jsonWriter, s *spb.Struct, skip string, topLevel bool) {
	fields := s.GetFields()
	if len(fields) == 0 {
		return
	}

	for _, key := range sortedKeys(fields) {
		if key == skip || key == MessageKey || (topLevel && isAgentKey(key)) {
			continue
		}

		w.key(key)
		w.buf = appendValue(w.buf, fields[key])
	}
}

// isAgentKey reports whether the agent reads the key.
func isAgentKey(key string) bool {
	switch key {
	case MessageKey, agentSeverityKey, agentTimestampKey, agentLabelsKey, agentSourceLocationKey,
		agentSpanIDKey, agentTraceKey, agentTraceSampledKey:
		return true
	default:
		return false
	}
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
		return &jsonWriter{buf: make([]byte, 0, initialBufferSize), first: true, pending: nil}
	},
}

// appendLabels appends the labels as a JSON object with sorted keys.
func appendLabels(buf []byte, labels map[string]string) []byte {
	buf = append(buf, '{')

	for i, key := range sortedKeys(labels) {
		if i > 0 {
			buf = append(buf, ',')
		}

		buf = appendKey(buf, key)
		buf = appendString(buf, labels[key])
	}

	return append(buf, '}')
}

// appendSourceLocation appends the source location as a JSON object.  The
// line is a string, as the agent expects.
func appendSourceLocation(buf []byte, loc *logpb.LogEntrySourceLocation) []byte {
	buf = append(buf, '{')
	buf = appendKey(buf, "file")
	buf = appendString(buf, loc.GetFile())
	buf = append(buf, ',')
	buf = appendKey(buf, "line")
	buf = append(buf, '"')
	buf = strconv.AppendInt(buf, loc.GetLine(), decimalBase)
	buf = append(buf, '"', ',')
	buf = appendKey(buf, "function")
	buf = appendString(buf, loc.GetFunction())

	return append(buf, '}')
}

// appendStruct appends the struct as a JSON object with sorted keys.
func appendStruct(buf []byte, s *spb.Struct) []byte {
	fields := s.GetFields()

	buf = append(buf, '{')

	for i, key := range sortedKeys(fields) {
		if i > 0 {
			buf = append(buf, ',')
		}

		buf = appendKey(buf, key)
		buf = appendValue(buf, fields[key])
	}

	return append(buf, '}')
}

// appendValue appends the value as JSON.  A nil value is null.
func appendValue(buf []byte, v *spb.Value) []byte {
	switch kind := v.GetKind().(type) {
	case *spb.Value_NumberValue:
		return appendNumber(buf, kind.NumberValue)
	case *spb.Value_StringValue:
		return appendString(buf, kind.StringValue)
	case *spb.Value_BoolValue:
		return strconv.AppendBool(buf, kind.BoolValue)
	case *spb.Value_StructValue:
		return appendStruct(buf, kind.StructValue)
	case *spb.Value_ListValue:
		buf = append(buf, '[')

		for i, item := range kind.ListValue.GetValues() {
			if i > 0 {
				buf = append(buf, ',')
			}

			buf = appendValue(buf, item)
		}

		return append(buf, ']')
	default:
		return append(buf, "null"...)
	}
}

// appendNumber appends the number in the format that encoding/json uses.
// NaN and the infinities are not valid JSON numbers, so they are strings.
func appendNumber(buf []byte, f float64) []byte {
	switch {
	case math.IsNaN(f):
		return appendString(buf, "NaN")
	case math.IsInf(f, 1):
		return appendString(buf, "Infinity")
	case math.IsInf(f, -1):
		return appendString(buf, "-Infinity")
	}

	const (
		lowExponent  = 1e-6
		highExponent = 1e21
	)

	format := byte('f')

	if abs := math.Abs(f); abs != 0 && (abs < lowExponent || abs >= highExponent) {
		format = 'e'
	}

	start := len(buf)
	buf = strconv.AppendFloat(buf, f, format, -1, float64Bits)

	if format == 'e' {
		buf = trimExponent(buf, start)
	}

	return buf
}

// trimExponent removes the leading zero from a two-digit exponent, so that
// "e-09" becomes "e-9".  The number starts at start.
func trimExponent(buf []byte, start int) []byte {
	const exponentLen = 4

	n := len(buf)
	if n-start < exponentLen {
		return buf
	}

	if buf[n-exponentLen] == 'e' && buf[n-exponentLen+1] == '-' && buf[n-exponentLen+2] == '0' {
		buf[n-exponentLen+2] = buf[n-1]
		buf = buf[:n-1]
	}

	return buf
}

// appendKey appends the key as a JSON string followed by a colon.
func appendKey(buf []byte, key string) []byte {
	buf = appendString(buf, key)

	return append(buf, ':')
}

// appendString appends s as a JSON string.  Invalid UTF-8 bytes become the
// Unicode replacement character.
func appendString(buf []byte, s string) []byte {
	buf = append(buf, '"')
	start := 0

	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if b >= ' ' && b != '"' && b != '\\' {
				i++

				continue
			}

			buf = append(buf, s[start:i]...)
			buf = appendEscape(buf, b)
			i++
			start = i

			continue
		}

		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			buf = append(buf, s[start:i]...)
			buf = append(buf, `�`...)
			i += size
			start = i

			continue
		}

		i += size
	}

	buf = append(buf, s[start:]...)

	return append(buf, '"')
}

// appendEscape appends the JSON escape sequence for the byte b.
func appendEscape(buf []byte, b byte) []byte {
	switch b {
	case '"', '\\':
		return append(buf, '\\', b)
	case '\n':
		return append(buf, '\\', 'n')
	case '\r':
		return append(buf, '\\', 'r')
	case '\t':
		return append(buf, '\\', 't')
	default:
		buf = append(buf, '\\', 'u', '0', '0')

		return append(buf, hexDigits[b>>unicodeEscapeLen], hexDigits[b&0xF])
	}
}

// severityName returns the name of the severity as the Cloud Logging API
// spells it.
func severityName(s logging.Severity) string {
	switch s {
	case logging.Default:
		return "DEFAULT"
	case logging.Debug:
		return nameDebug
	case logging.Info:
		return nameInfo
	case logging.Notice:
		return "NOTICE"
	case logging.Warning:
		return "WARNING"
	case logging.Error:
		return nameError
	case logging.Critical:
		return "CRITICAL"
	case logging.Alert:
		return "ALERT"
	case logging.Emergency:
		return "EMERGENCY"
	default:
		return strconv.Itoa(int(s))
	}
}

// sortedKeys returns the keys of m in sorted order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))

	for key := range m {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	return keys
}
