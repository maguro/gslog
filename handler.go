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
	"google.golang.org/protobuf/proto"
	spb "google.golang.org/protobuf/types/known/structpb"

	"m4o.io/gslog/v1/internal/attr"
	"m4o.io/gslog/v1/internal/level"
	"m4o.io/gslog/v1/internal/options"
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
	replaceAttr     AttrMapper

	payload *spb.Struct
	groups  []string
}

var _ slog.Handler = &GcpHandler{}

// NewGcpHandler creates a Google Cloud Logging backed log.Logger.
func NewGcpHandler(logger Logger, opts ...options.OptionProcessor) *GcpHandler {
	if logger == nil {
		panic("client is nil")
	}
	o := options.ApplyOptions(opts...)

	return newGcpLoggerWithOptions(logger, o)
}

func newGcpLoggerWithOptions(logger Logger, o *options.Options) *GcpHandler {
	h := &GcpHandler{
		log:   logger,
		level: o.Level,

		addSource:       o.AddSource,
		entryAugmentors: o.EntryAugmentors,
		replaceAttr:     attr.WrapAttrMapper(o.ReplaceAttr),

		payload: &spb.Struct{Fields: make(map[string]*spb.Value)},
	}

	return h
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

func (h *GcpHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.level.Level() <= level
}

// Handle will handle a slog.Record, as described in the interface's
// documentation.  It will translate the slog.Record into a logging.Entry
// that's filled with a *spb.Value as a Entry Payload.
func (h *GcpHandler) Handle(ctx context.Context, record slog.Record) error {
	payload2 := proto.Clone(h.payload).(*spb.Struct)
	if payload2.Fields == nil {
		payload2.Fields = make(map[string]*spb.Value)
	}

	setAndClean(h.groups, payload2, func(groups []string, payload *spb.Struct) {
		record.Attrs(func(a slog.Attr) bool {
			if h.replaceAttr != nil {
				a = h.replaceAttr(h.groups, a)
			}
			attr.DecorateWith(payload, a)
			return true
		})
	})

	msg := record.Message
	a := slog.String(MessageKey, msg)
	if h.replaceAttr != nil {
		a = h.replaceAttr(nil, a)
	}
	attr.DecorateWith(payload2, a)

	var e logging.Entry

	e.Payload = payload2
	e.Timestamp = record.Time.UTC()
	e.Severity = level.LevelToSeverity(record.Level)
	e.Labels = ExtractLabels(ctx)

	if h.addSource {
		addSourceLocation(&e, &record)
	}

	for _, b := range h.entryAugmentors {
		b(ctx, &e, h.groups)
	}

	if e.Severity >= logging.Critical {
		err := h.log.LogSync(ctx, e)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error logging: %s\n%s", record.Message, err)
		}
	} else {
		h.log.Log(e)
	}

	return nil
}

func (h *GcpHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	var h2 = h.clone()

	current := fromPath(h2.payload, h2.groups)

	for _, a := range attrs {
		if h.replaceAttr != nil {
			a = h.replaceAttr(h.groups, a)
		}
		attr.DecorateWith(current, a)
	}

	return h2
}

func (h *GcpHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	var h2 = h.clone()

	h2.payload = proto.Clone(h.payload).(*spb.Struct)

	current := fromPath(h2.payload, h2.groups)

	current.Fields[name] = &spb.Value{
		Kind: &spb.Value_StructValue{
			StructValue: &spb.Struct{
				Fields: make(map[string]*spb.Value),
			},
		},
	}

	h2.groups = append(h.groups, name)

	return h2
}

func (h *GcpHandler) clone() *GcpHandler {
	return &GcpHandler{
		log:   h.log,
		level: h.level,

		addSource:       h.addSource,
		entryAugmentors: h.entryAugmentors,
		replaceAttr:     h.replaceAttr,

		payload: proto.Clone(h.payload).(*spb.Struct),
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

func fromPath(p *spb.Struct, path []string) *spb.Struct {
	for _, k := range path {
		p = p.Fields[k].GetStructValue()
	}
	if p.Fields == nil {
		p.Fields = make(map[string]*spb.Value)
	}
	return p
}

func setAndClean(groups []string, payload *spb.Struct, decorate func(groups []string, payload *spb.Struct)) {
	if len(groups) == 0 {
		if payload.Fields == nil {
			payload.Fields = make(map[string]*spb.Value)
		}

		decorate(groups, payload)
		return
	}

	g := groups[0]

	s := payload.Fields[g].GetStructValue()
	setAndClean(groups[1:], s, decorate)

	if len(s.Fields) == 0 {
		delete(payload.Fields, g)
	}
}
