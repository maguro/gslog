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
	"m4o.io/gslog/internal/entry"
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

// errorPrefix is the start of the string that the writer appends for a value
// that encoding/json cannot encode.  log/slog uses the same text.
const errorPrefix = "!ERROR:"

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

	// hasReport is true when the record has an error report.
	// separateReport is true for a writer from NewWriter.  That writer
	// appends the top-level members with a key of the error report to
	// reportMembers.  reportMembers is nil until the writer appends the
	// first of these members.
	hasReport      bool
	separateReport bool
	reportMembers  *Writer
}

// NewWriter returns a writer that appends to buf.  The writer does not open
// a top-level object.  Use NewWriter to build the members that AppendRaw
// takes.  The writer keeps the top-level members with a key of the error
// report apart.  ReportMembers returns these members.
func NewWriter(buf []byte) *Writer {
	return &Writer{
		buf:     buf,
		first:   len(buf) == 0,
		pending: nil,

		hasReport:      false,
		separateReport: true,
		reportMembers:  nil,
	}
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

// Member returns the writer for a top-level member with the key.  Member
// returns nil for a member that the handler does not write.  The handler does
// not write a member with a key that the agent reads.  In a record that has
// an error report, the handler does not write a member with a key of the
// error report.
func (w *Writer) Member(key string) *Writer {
	switch key {
	case core.MessageKey, agentSeverityKey, agentTimestampKey, agentLabelsKey, agentSourceLocationKey,
		agentSpanIDKey, agentTraceKey, agentTraceSampledKey:
		return nil
	case entry.ReportTypeKey, entry.ServiceContextKey, entry.StackTraceKey:
		return w.reportMember()
	default:
		return w
	}
}

// HasReport reports whether the record has an error report.
func (w *Writer) HasReport() bool { return w.hasReport }

// ReportMembers returns the serialized top-level members with a key of the
// error report.  ReportMembers returns nil when the writer has no such
// member.
func (w *Writer) ReportMembers() []byte {
	if w.reportMembers == nil {
		return nil
	}

	return w.reportMembers.buf
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

// AppendErrorReport appends the members that Error Reporting reads and sets
// hasReport.  Call AppendErrorReport before the writer appends the message or
// an attribute.
func (w *Writer) AppendErrorReport(report *entry.ErrorReport) {
	w.hasReport = true

	w.key(entry.ReportTypeKey)
	w.buf = appendString(w.buf, entry.ReportTypeValue)

	w.key(entry.ServiceContextKey)
	w.buf = appendServiceContext(w.buf, report)

	w.key(entry.StackTraceKey)
	w.buf = appendString(w.buf, report.StackTrace)
}

// AppendAnyAttr appends an attribute of kind slog.KindAny.  A nil value is
// null.  An error that does not implement json.Marshaler is written as its
// Error() string.  All other values are encoded with encoding/json.  If
// encoding/json returns an error, AppendAnyAttr appends the string "!ERROR:"
// followed by the error text.
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
		text := errorPrefix + err.Error()

		w.key(key)
		w.buf = appendString(w.buf, text)

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
	w.hasReport = false
}

// reportMember returns the writer for a top-level member with a key of the
// error report.  reportMember returns nil when the record has an error
// report.
func (w *Writer) reportMember() *Writer {
	switch {
	case w.hasReport:
		return nil
	case !w.separateReport:
		return w
	}

	if w.reportMembers == nil {
		w.reportMembers = &Writer{
			buf:     nil,
			first:   true,
			pending: nil,

			hasReport:      false,
			separateReport: false,
			reportMembers:  nil,
		}
	}

	return w.reportMembers
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

// JoinMembers returns the serialized members of a followed by the serialized
// members of b.  JoinMembers does not change a.
func JoinMembers(a, b []byte) []byte {
	if len(a) == 0 {
		return b
	}

	if len(b) == 0 {
		return a
	}

	joined := make([]byte, 0, len(a)+len(b)+1)
	joined = append(joined, a...)
	joined = append(joined, ',')

	return append(joined, b...)
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

// appendServiceContext appends the service context of the error report as a
// JSON object.  The object has the version only when the version is not
// empty.
func appendServiceContext(buf []byte, report *entry.ErrorReport) []byte {
	buf = append(buf, '{')
	buf = appendKey(buf, entry.ServiceKey)
	buf = appendString(buf, report.Service)

	if report.Version != "" {
		buf = append(buf, ',')
		buf = appendKey(buf, entry.VersionKey)
		buf = appendString(buf, report.Version)
	}

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
