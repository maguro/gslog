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

	"cloud.google.com/go/logging"
	logpb "cloud.google.com/go/logging/apiv2/loggingpb"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"google.golang.org/protobuf/proto"
)

const (
	fieldMessage = "message"
)

// GcpHandler is a Google Cloud Logging backed slog handler.
type GcpHandler struct {
	// *logging.Logger, except for testing
	log   logger
	level slog.Leveler

	// addSource causes the handler to compute the source code position
	// of the log statement and add a SourceKey attribute to the output.
	addSource       bool
	entryAugmentors []AugmentEntryFn
	replaceAttr     AttrMapper

	payload *structpb.Struct
	groups  []string
}

var _ slog.Handler = &GcpHandler{}

// NewGcpHandler creates a Google Cloud Logging backed log.Logger.
func NewGcpHandler(logger *logging.Logger, opts ...Option) (*GcpHandler, error) {
	if logger == nil {
		panic("client is nil")
	}
	o, err := applyOptions(opts...)
	if err != nil {
		return nil, err
	}

	return newGcpLoggerWithOptions(logger, o), nil
}

func NewGcpLoggerWithOptions(logger logger, opts ...Option) *GcpHandler {
	if logger == nil {
		panic("client is nil")
	}
	o, err := applyOptions(opts...)
	if err != nil {
		panic(err)
	}

	return newGcpLoggerWithOptions(logger, o)
}

func newGcpLoggerWithOptions(logger logger, o *Options) *GcpHandler {
	h := &GcpHandler{
		log:   logger,
		level: o.level,

		addSource:       o.addSource,
		entryAugmentors: o.EntryAugmentors,
		replaceAttr:     o.replaceAttr,

		payload: &structpb.Struct{Fields: make(map[string]*structpb.Value)},
	}

	return h
}

func (h *GcpHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.level.Level() <= level
}

func (h *GcpHandler) Handle(ctx context.Context, record slog.Record) error {
	payload2 := proto.Clone(h.payload).(*structpb.Struct)
	current := fromPath(payload2, h.groups)

	record.Attrs(func(attr slog.Attr) bool {
		decorateWith(current, attr)
		return true
	})

	if payload2.Fields == nil {
		payload2.Fields = make(map[string]*structpb.Value)
	}
	payload2.Fields[fieldMessage] = &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: record.Message}}

	var e logging.Entry

	e.Payload = payload2
	e.Timestamp = record.Time.UTC()
	e.Severity = levelToSeverity(record.Level)
	e.Labels = ExtractLabels(ctx)

	if h.addSource {
		addSourceLocation(&e, &record)
	}

	for _, b := range h.entryAugmentors {
		b(ctx, &e)
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
		decorateWith(current, a)
	}

	return h2
}

func (h *GcpHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	var h2 = h.clone()

	h2.payload = proto.Clone(h.payload).(*structpb.Struct)

	current := fromPath(h2.payload, h2.groups)

	current.Fields[name] = &structpb.Value{
		Kind: &structpb.Value_StructValue{
			StructValue: &structpb.Struct{
				Fields: make(map[string]*structpb.Value),
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

		payload: proto.Clone(h.payload).(*structpb.Struct),
		groups:  h.groups,
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
