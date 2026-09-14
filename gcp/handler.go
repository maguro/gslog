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

package gcp

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"slices"

	"cloud.google.com/go/logging"
	logpb "cloud.google.com/go/logging/apiv2/loggingpb"
	"github.com/pkg/errors"
	spb "google.golang.org/protobuf/types/known/structpb"

	"m4o.io/gslog/core"
	"m4o.io/gslog/internal/attr"
	"m4o.io/gslog/internal/entry"
	"m4o.io/gslog/internal/level"
	"m4o.io/gslog/internal/mapper"
	"m4o.io/gslog/internal/options"
)

// Handler is a slog.Handler that writes to Google Cloud Logging through
// the API client.
//
// A top-level attribute or group with the key "message" is replaced by the
// message of the log call.
type Handler struct {
	// log is a *logging.Logger, except in tests.
	log   Logger
	level slog.Leveler

	// addSource causes the handler to compute the source code position
	// of the log statement.  The handler sets the position in the
	// SourceLocation field of the entry.
	addSource       bool
	entryAugmentors []entry.Augmentor
	replaceAttr     mapper.Mapper

	payload *spb.Struct
	groups  []string
}

var _ slog.Handler = (*Handler)(nil)

// NewHandler creates a Handler that writes to Google Cloud Logging through
// the logger.
func NewHandler(logger Logger, opts ...core.Option) *Handler {
	if logger == nil {
		panic("client is nil")
	}

	o := options.ApplyOptions(opts...)

	return newHandlerWithOptions(logger, o)
}

func newHandlerWithOptions(logger Logger, opts *options.Options) *Handler {
	handler := &Handler{
		log:   logger,
		level: opts.Level,

		addSource:       opts.AddSource,
		entryAugmentors: opts.EntryAugmentors,
		replaceAttr:     mapper.Wrap(opts.ReplaceAttr),

		payload: &spb.Struct{Fields: make(map[string]*spb.Value)},
		groups:  nil,
	}

	return handler
}

// WithLeveler returns a copy of the handler that uses the supplied leveler.
func (h *Handler) WithLeveler(leveler slog.Leveler) *Handler {
	if leveler == nil {
		panic("Leveler is nil")
	}

	h2 := h.clone()
	h2.level = leveler

	return h2
}

// Enabled reports whether the handler handles records at the given level.
// The handler ignores a record that has a lower level.
func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	return h.level.Level() <= level
}

// Handle handles a slog.Record as the slog.Handler interface specifies.
// Handle translates the slog.Record into a logging.Entry.  The Payload of
// the entry is a *spb.Struct.
func (h *Handler) Handle(ctx context.Context, record slog.Record) error {
	e := entry.Fill(ctx, &record, h.addSource, h.entryAugmentors)

	payload := h.decorate(h.payload, h.groups, &record, e.Attrs)

	a := slog.String(core.MessageKey, record.Message)
	if h.replaceAttr != nil {
		a = h.replaceAttr(nil, a)
	}

	attr.DecorateWith(payload, a)

	logEntry := toLogEntry(&e, &record, payload)

	if logEntry.Severity >= logging.Critical {
		if err := h.log.LogSync(ctx, logEntry); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error logging: %s\n%s", record.Message, err)
		}
	} else {
		h.log.Log(logEntry)
	}

	return nil
}

// WithAttrs returns a copy of the handler.  The attributes of the copy are
// the attributes of h followed by attrs.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handler2 := h.clone()

	payload, current := copyPath(h.payload, h.groups)
	handler2.payload = payload

	for _, a := range attrs {
		h.decorateAttr(current, a)
	}

	return handler2
}

// WithGroup returns a copy of the handler.  The groups of the copy are the
// groups of h followed by name.
func (h *Handler) WithGroup(name string) slog.Handler {
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
func (h *Handler) Flush() error {
	if err := h.log.Flush(); err != nil {
		return errors.Wrap(err, "failed to flush handler")
	}

	return nil
}

// decorate returns a copy of src.  The copy has the attributes of record,
// followed by extra, in the group at the end of groups.  The copy shares all
// values of src that are not on the group path.  If a group on the path is
// empty after decorate adds the attributes, decorate removes that group from
// the copy.
func (h *Handler) decorate(src *spb.Struct, groups []string, record *slog.Record, extra []slog.Attr) *spb.Struct {
	dst := &spb.Struct{Fields: cloneFields(src)}

	if len(groups) == 0 {
		record.Attrs(func(a slog.Attr) bool {
			h.decorateAttr(dst, a)

			return true
		})

		for _, a := range extra {
			h.decorateAttr(dst, a)
		}

		return dst
	}

	name := groups[0]
	group := src.GetFields()[name].GetStructValue()
	child := h.decorate(group, groups[1:], record, extra)

	if len(child.Fields) == 0 {
		delete(dst.Fields, name)

		return dst
	}

	dst.Fields[name] = attr.NewStructValue(child)

	return dst
}

// decorateAttr applies the mapper to the attribute and adds the result to
// the struct.
func (h *Handler) decorateAttr(dst *spb.Struct, a slog.Attr) {
	if h.replaceAttr != nil {
		a = h.replaceAttr(h.groups, a)
	}

	attr.DecorateWith(dst, a)
}

// clone returns a copy of the handler.  The copy shares the payload of h.
func (h *Handler) clone() *Handler {
	return &Handler{
		log:   h.log,
		level: h.level,

		addSource:       h.addSource,
		entryAugmentors: h.entryAugmentors,
		replaceAttr:     h.replaceAttr,

		payload: h.payload,
		groups:  slices.Clip(h.groups),
	}
}

// toLogEntry converts the entry, the record, and the payload to a
// logging.Entry.
func toLogEntry(e *entry.Entry, record *slog.Record, payload *spb.Struct) logging.Entry {
	logEntry := logging.Entry{
		Timestamp:    record.Time.UTC(),
		Severity:     logging.Severity(level.ToSeverity(record.Level)),
		Labels:       e.Labels,
		Trace:        e.Trace,
		SpanID:       e.SpanID,
		TraceSampled: e.TraceSampled,
		Payload:      payload,
	}

	if e.Source != nil {
		logEntry.SourceLocation = &logpb.LogEntrySourceLocation{
			File:     e.Source.File,
			Line:     int64(e.Source.Line),
			Function: e.Source.Function,
		}
	}

	return logEntry
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
// of s.  The map of a nil struct is empty.
func cloneFields(s *spb.Struct) map[string]*spb.Value {
	fields := maps.Clone(s.GetFields())
	if fields == nil {
		return make(map[string]*spb.Value)
	}

	return fields
}
