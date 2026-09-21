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

package jbuf

import (
	"encoding/json"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"sync"
	"time"

	"m4o.io/gslog/core"
	"m4o.io/gslog/internal/level"
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
	hexDigits        = "0123456789abcdef"
	unicodeEscapeLen = 4
	decimalBase      = 10
	float64Bits      = 64
)

const (
	initialBufferSize = 1024
	maxPooledBuffer   = 16 << 10
)

var writerPool = sync.Pool{
	New: func() any {
		return &Writer{buf: make([]byte, 0, initialBufferSize), first: true}
	},
}

// Writer appends JSON members to a buffer.  A group becomes a nested object
// when the writer appends the first member of the group.
type Writer struct {
	buf []byte

	// first is true when the next member is the first one in the open
	// object.  pending holds the keys of the groups that have no member yet.
	first   bool
	pending []string
}

// NewWriter returns a writer that appends to buf.  The writer does not open
// a top-level object.  Use NewWriter to build the members that AppendRaw
// takes.
func NewWriter(buf []byte) *Writer {
	return &Writer{buf: buf, first: len(buf) == 0}
}

// AllocWriter returns a writer from the pool.  The writer is at the start of
// a new top-level object.  Call ReleaseWriter after the last use of the
// writer.
func AllocWriter() *Writer {
	w, _ := writerPool.Get().(*Writer)
	w.reset()

	return w
}

// ReleaseWriter puts the writer in the pool.  ReleaseWriter does not put a
// writer in the pool when the capacity of its buffer is larger than
// maxPooledBuffer.  Do not use the writer after the call.
func ReleaseWriter(w *Writer) {
	if cap(w.buf) <= maxPooledBuffer {
		writerPool.Put(w)
	}
}

// AppendRaw appends serialized members.  An empty slice appends nothing.
func (w *Writer) AppendRaw(members []byte) {
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

// OpenGroup starts a group.  The writer appends the key and the nested
// object when it appends the first member of the group.
func (w *Writer) OpenGroup(key string) {
	w.pending = append(w.pending, key)
}

// CloseGroup ends the group.  The writer appends nothing for a group with
// no member.
func (w *Writer) CloseGroup() {
	if n := len(w.pending); n > 0 {
		w.pending = w.pending[:n-1]

		return
	}

	w.buf = append(w.buf, '}')
	w.first = false
}

// AppendSeverity appends the severity with the key that the agent reads.
func (w *Writer) AppendSeverity(severity level.Severity) {
	name := severity.String()

	w.key(agentSeverityKey)
	w.buf = appendString(w.buf, name)
}

// AppendTimestamp appends the time with the key that the agent reads.  The
// time is in UTC and has the format of time.RFC3339Nano.
func (w *Writer) AppendTimestamp(t time.Time) {
	w.key(agentTimestampKey)
	w.buf = append(w.buf, '"')
	w.buf = t.UTC().AppendFormat(w.buf, time.RFC3339Nano)
	w.buf = append(w.buf, '"')
}

// AppendLabels appends the labels with the key that the agent reads.
func (w *Writer) AppendLabels(labels map[string]string) {
	w.key(agentLabelsKey)
	w.buf = appendLabels(w.buf, labels)
}

// AppendTrace appends the trace with the key that the agent reads.
func (w *Writer) AppendTrace(trace string) {
	w.key(agentTraceKey)
	w.buf = appendString(w.buf, trace)
}

// AppendSpanID appends the span ID with the key that the agent reads.
func (w *Writer) AppendSpanID(spanID string) {
	w.key(agentSpanIDKey)
	w.buf = appendString(w.buf, spanID)
}

// AppendTraceSampled appends the value true with the key that the agent
// reads for a sampled trace.
func (w *Writer) AppendTraceSampled() {
	w.key(agentTraceSampledKey)
	w.buf = append(w.buf, "true"...)
}

// AppendSourceLocation appends the source location with the key that the
// agent reads.
func (w *Writer) AppendSourceLocation(loc *slog.Source) {
	w.key(agentSourceLocationKey)
	w.buf = appendSourceLocation(w.buf, loc)
}

// AppendAnyAttr appends an attribute of kind slog.KindAny.  A nil value is
// null.  An error that does not implement json.Marshaler is written as its
// Error() string.  All other values are encoded with encoding/json.
func (w *Writer) AppendAnyAttr(key string, value any) {
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

// AppendSlogValue appends the key and the value as a JSON member.  v must be
// a resolved value that is not a group and not of kind slog.KindAny.
func (w *Writer) AppendSlogValue(key string, v slog.Value) {
	w.key(key)
	w.buf = appendSlogValue(w.buf, v)
}

// CloseLine ends the top-level object and the line.
func (w *Writer) CloseLine() {
	w.buf = append(w.buf, '}', '\n')
}

// Buf returns the buffer of the writer.  Do not use the result after a call
// to ReleaseWriter.
func (w *Writer) Buf() []byte { return w.buf }

// reset starts a new top-level object.
func (w *Writer) reset() {
	w.buf = append(w.buf[:0], '{')
	w.first = true
	w.pending = w.pending[:0]
}

// key appends the separator, if needed, and the key.
func (w *Writer) key(key string) {
	w.openPending()

	if !w.first {
		w.buf = append(w.buf, ',')
	}

	w.first = false
	w.buf = appendKey(w.buf, key)
}

// openPending appends the key and the nested object of each pending group.
func (w *Writer) openPending() {
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

// IsAgentKey reports whether the agent reads the key.
func IsAgentKey(key string) bool {
	switch key {
	case core.MessageKey, agentSeverityKey, agentTimestampKey, agentLabelsKey, agentSourceLocationKey,
		agentSpanIDKey, agentTraceKey, agentTraceSampledKey:
		return true
	default:
		return false
	}
}

// appendLabels appends the labels as a JSON object with sorted keys.
func appendLabels(buf []byte, labels map[string]string) []byte {
	buf = append(buf, '{')

	for i, key := range slices.Sorted(maps.Keys(labels)) {
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
func appendSourceLocation(buf []byte, loc *slog.Source) []byte {
	buf = append(buf, '{')
	buf = appendKey(buf, "file")
	buf = appendString(buf, loc.File)
	buf = append(buf, ',')
	buf = appendKey(buf, "line")
	buf = append(buf, '"')
	buf = strconv.AppendInt(buf, int64(loc.Line), decimalBase)
	buf = append(buf, '"', ',')
	buf = appendKey(buf, "function")
	buf = appendString(buf, loc.Function)

	return append(buf, '}')
}

// appendKey appends the key as a JSON string followed by a colon.
func appendKey(buf []byte, key string) []byte {
	buf = appendString(buf, key)

	return append(buf, ':')
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
