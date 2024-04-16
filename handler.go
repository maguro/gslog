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
	"google.golang.org/protobuf/proto"
	spb "google.golang.org/protobuf/types/known/structpb"

	"m4o.io/gslog/internal/attr"
	"m4o.io/gslog/internal/level"
	"m4o.io/gslog/internal/options"
)

const (
	// MessageKey is the key used for the message of the log call, per Google
	// Cloud Logging. The associated value is a string.
	MessageKey = "message"
)

// GcpHandler is a Google Cloud Logging backed slog handler.
type GcpHandler struct {
	// *logging.Logger, except for testing
	log   Logger
	level slog.Leveler

	// addSource causes the handler to compute the source code position
	// of the log statement and add a SourceKey attribute to the output.
	addSource       bool
	entryAugmentors []options.EntryAugmentor
	replaceAttr     attr.Mapper

	payload *spb.Struct
	groups  []string
}

var _ slog.Handler = (*GcpHandler)(nil)

// NewGcpHandler creates a Google Cloud Logging backed log.Logger.
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

// WithLeveler returns a copy of the handler, provisioned with the supplied
// leveler.
func (h *GcpHandler) WithLeveler(leveler slog.Leveler) *GcpHandler {
	if leveler == nil {
		panic("Leveler is nil")
	}

	h2 := h.clone()
	h2.level = leveler

	return h2
}

// Enabled reports whether the handler handles records at the given level.
// The handler ignores records whose level is lower.
func (h *GcpHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.level.Level() <= level
}

// Handle will handle a slog.Record, as described in the interface's
// documentation.  It will translate the slog.Record into a logging.Entry
// that's filled with a *spb.Value as a Entry Payload.
func (h *GcpHandler) Handle(ctx context.Context, record slog.Record) error {
	//nolint:forcetypeassert
	payload2 := proto.Clone(h.payload).(*spb.Struct)

	if payload2.Fields == nil {
		payload2.Fields = make(map[string]*spb.Value)
	}

	setAndClean(h.groups, payload2, func(_ []string, payload *spb.Struct) {
		record.Attrs(func(a slog.Attr) bool {
			if h.replaceAttr != nil {
				a = h.replaceAttr(h.groups, a)
			}

			attr.DecorateWith(payload, a)

			return true
		})
	})

	a := slog.String(MessageKey, record.Message)
	if h.replaceAttr != nil {
		a = h.replaceAttr(nil, a)
	}

	attr.DecorateWith(payload2, a)

	var entry logging.Entry

	entry.Payload = payload2
	entry.Timestamp = record.Time.UTC()
	entry.Severity = level.ToSeverity(record.Level)
	entry.Labels = ExtractLabels(ctx)

	if h.addSource {
		addSourceLocation(&entry, &record)
	}

	for _, b := range h.entryAugmentors {
		b(ctx, &entry, h.groups)
	}

	if entry.Severity >= logging.Critical {
		err := h.log.LogSync(ctx, entry)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error logging: %s\n%s", record.Message, err)
		}
	} else {
		h.log.Log(entry)
	}

	return nil
}

// WithAttrs returns a copy of the handler whose attributes consists
// of h's attributes followed by attrs.
func (h *GcpHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handler2 := h.clone()

	current := fromPath(handler2.payload, handler2.groups)

	for _, a := range attrs {
		if h.replaceAttr != nil {
			a = h.replaceAttr(h.groups, a)
		}

		attr.DecorateWith(current, a)
	}

	return handler2
}

// WithGroup returns a copy of the handler with the given group
// appended to the receiver's existing groups.
func (h *GcpHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	handler2 := h.clone()

	//nolint:forcetypeassert
	payload2 := proto.Clone(h.payload).(*spb.Struct)

	handler2.payload = payload2

	current := fromPath(handler2.payload, handler2.groups)

	current.Fields[name] = &spb.Value{
		Kind: &spb.Value_StructValue{
			StructValue: &spb.Struct{
				Fields: make(map[string]*spb.Value),
			},
		},
	}

	handler2.groups = h.groups
	handler2.groups = append(handler2.groups, name)

	return handler2
}

// Flush blocks until all currently buffered log entries are sent.
//
// If any errors occurred since the last call to Flush from any Logger, or the
// creation of the client if this is the first call, then Flush returns a non-nil
// error with summary information about the errors. This information is unlikely to
// be actionable. For more accurate error reporting, set Client.OnError.
func (h *GcpHandler) Flush() error {
	if err := h.log.Flush(); err != nil {
		return errors.Wrap(err, "failed to flush handler")
	}

	return nil
}

func (h *GcpHandler) clone() *GcpHandler {
	//nolint:forcetypeassert
	payload2 := proto.Clone(h.payload).(*spb.Struct)

	return &GcpHandler{
		log:   h.log,
		level: h.level,

		addSource:       h.addSource,
		entryAugmentors: h.entryAugmentors,
		replaceAttr:     h.replaceAttr,

		payload: payload2,
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

func fromPath(payload *spb.Struct, path []string) *spb.Struct {
	for _, k := range path {
		payload = payload.GetFields()[k].GetStructValue()
	}

	if payload.Fields == nil {
		payload.Fields = make(map[string]*spb.Value)
	}

	return payload
}

func setAndClean(groups []string, payload *spb.Struct, decorate func(groups []string, payload *spb.Struct)) {
	if len(groups) == 0 {
		if payload.Fields == nil {
			payload.Fields = make(map[string]*spb.Value)
		}

		decorate(groups, payload)

		return
	}

	group := groups[0]

	s := payload.GetFields()[group].GetStructValue()
	setAndClean(groups[1:], s, decorate)

	if len(s.GetFields()) == 0 {
		delete(payload.GetFields(), group)
	}
}
