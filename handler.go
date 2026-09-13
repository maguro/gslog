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
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"slices"

	"cloud.google.com/go/logging"
	logpb "cloud.google.com/go/logging/apiv2/loggingpb"
	"github.com/pkg/errors"
	spb "google.golang.org/protobuf/types/known/structpb"

	"m4o.io/gslog/internal/attr"
	"m4o.io/gslog/internal/level"
	"m4o.io/gslog/internal/options"
)

const (
	// MessageKey is the key that Google Cloud Logging specifies for the
	// message of the log call.  The value is a string.
	MessageKey = "message"
)

// GcpHandler is a slog.Handler that writes to Google Cloud Logging.
type GcpHandler struct {
	// log is a *logging.Logger, except in tests.
	log   Logger
	level slog.Leveler

	// addSource causes the handler to compute the source code position
	// of the log statement.  The handler sets the position in the
	// SourceLocation field of the entry.
	addSource       bool
	entryAugmentors []options.EntryAugmentor
	replaceAttr     attr.Mapper

	payload *spb.Struct
	groups  []string
}

var _ slog.Handler = (*GcpHandler)(nil)

// NewGcpHandler creates a GcpHandler that writes to Google Cloud Logging.
func NewGcpHandler(logger Logger, opts ...options.OptionProcessor) *GcpHandler {
	if logger == nil {
		panic("client is nil")
	}

	o := options.ApplyOptions(opts...)

	return newGcpLoggerWithOptions(logger, o)
}

func newGcpLoggerWithOptions(logger Logger, opts *options.Options) *GcpHandler {
	handler := &GcpHandler{
		log:   logger,
		level: opts.Level,

		addSource:       opts.AddSource,
		entryAugmentors: opts.EntryAugmentors,
		replaceAttr:     attr.WrapAttrMapper(opts.ReplaceAttr),

		payload: &spb.Struct{Fields: make(map[string]*spb.Value)},
		groups:  nil,
	}

	return handler
}

// WithLeveler returns a copy of the handler that uses the supplied leveler.
func (h *GcpHandler) WithLeveler(leveler slog.Leveler) *GcpHandler {
	if leveler == nil {
		panic("Leveler is nil")
	}

	h2 := h.clone()
	h2.level = leveler

	return h2
}

// Enabled reports whether the handler handles records at the given level.
// The handler ignores a record that has a lower level.
func (h *GcpHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.level.Level() <= level
}

// Handle handles a slog.Record as the slog.Handler interface specifies.
// Handle translates the slog.Record into a logging.Entry.  The Payload of
// the entry is a *spb.Struct.
func (h *GcpHandler) Handle(ctx context.Context, record slog.Record) error {
	payload := h.decorate(h.payload, h.groups, &record)

	a := slog.String(MessageKey, record.Message)
	if h.replaceAttr != nil {
		a = h.replaceAttr(nil, a)
	}

	attr.DecorateWith(payload, a)

	var entry logging.Entry

	entry.Payload = payload
	entry.Timestamp = record.Time.UTC()
	entry.Severity = level.ToSeverity(record.Level)

	if h.addSource {
		addSourceLocation(&entry, &record)
	}

	for _, b := range h.entryAugmentors {
		b(ctx, &entry, h.groups)
	}

	addLabels(ctx, &entry)

	if entry.Severity >= logging.Critical {
		if err := h.log.LogSync(ctx, entry); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error logging: %s\n%s", record.Message, err)
		}
	} else {
		h.log.Log(entry)
	}

	return nil
}

// WithAttrs returns a copy of the handler.  The attributes of the copy are
// the attributes of h followed by attrs.
func (h *GcpHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handler2 := h.clone()

	payload, current := copyPath(h.payload, h.groups)
	handler2.payload = payload

	for _, a := range attrs {
		if h.replaceAttr != nil {
			a = h.replaceAttr(h.groups, a)
		}

		attr.DecorateWith(current, a)
	}

	return handler2
}

// WithGroup returns a copy of the handler.  The groups of the copy are the
// groups of h followed by name.
func (h *GcpHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	handler2 := h.clone()

	payload, current := copyPath(h.payload, h.groups)
	handler2.payload = payload

	current.Fields[name] = attr.NewStructValue(&spb.Struct{Fields: make(map[string]*spb.Value)})

	handler2.groups = append(handler2.groups, name)

	return handler2
}

// Flush blocks until all log entries that are currently buffered are sent.
//
// Flush returns a non-nil error if errors occurred since the last call to
// Flush from any Logger.  If this is the first call, the errors count from
// the creation of the client.  The error contains summary information about
// the errors.  This information is unlikely to be actionable.  For more
// accurate error reports, set Client.OnError.
func (h *GcpHandler) Flush() error {
	if err := h.log.Flush(); err != nil {
		return errors.Wrap(err, "failed to flush handler")
	}

	return nil
}

// decorate returns a copy of src.  The copy has the attributes of record in
// the group at the end of groups.  The copy shares all values of src that
// are not on the group path.  If a group on the path is empty after decorate
// adds the attributes, decorate removes that group from the copy.
func (h *GcpHandler) decorate(src *spb.Struct, groups []string, record *slog.Record) *spb.Struct {
	dst := &spb.Struct{Fields: cloneFields(src)}

	if len(groups) == 0 {
		record.Attrs(func(a slog.Attr) bool {
			if h.replaceAttr != nil {
				a = h.replaceAttr(h.groups, a)
			}

			attr.DecorateWith(dst, a)

			return true
		})

		return dst
	}

	name := groups[0]
	group := src.GetFields()[name].GetStructValue()
	child := h.decorate(group, groups[1:], record)

	if len(child.Fields) == 0 {
		delete(dst.Fields, name)

		return dst
	}

	dst.Fields[name] = attr.NewStructValue(child)

	return dst
}

// clone returns a copy of the handler.  The copy shares the payload of h.
func (h *GcpHandler) clone() *GcpHandler {
	return &GcpHandler{
		log:   h.log,
		level: h.level,

		addSource:       h.addSource,
		entryAugmentors: h.entryAugmentors,
		replaceAttr:     h.replaceAttr,

		payload: h.payload,
		groups:  slices.Clip(h.groups),
	}
}

func addSourceLocation(e *logging.Entry, r *slog.Record) {
	fs := runtime.CallersFrames([]uintptr{r.PC})
	f, _ := fs.Next()

	e.SourceLocation = &logpb.LogEntrySourceLocation{
		File:     f.File,
		Line:     int64(f.Line),
		Function: f.Function,
	}
}

// copyPath returns a copy of src with a new struct at each level of path.
// The copy shares all values of src that are not on path.  copyPath also
// returns the struct at the end of path.
func copyPath(src *spb.Struct, path []string) (top, current *spb.Struct) {
	top = &spb.Struct{Fields: cloneFields(src)}
	current = top

	for _, name := range path {
		group := current.Fields[name].GetStructValue()
		child := &spb.Struct{Fields: cloneFields(group)}

		current.Fields[name] = attr.NewStructValue(child)
		current = child
	}

	return top, current
}

// cloneFields returns a new map with the same keys and values as the fields
// of s.
func cloneFields(s *spb.Struct) map[string]*spb.Value {
	fields := s.GetFields()
	dst := make(map[string]*spb.Value, len(fields))

	for k, v := range fields {
		dst[k] = v
	}

	return dst
}
