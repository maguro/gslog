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

package gcp_test

import (
	"context"
	"log/slog"

	"cloud.google.com/go/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/structpb"

	"m4o.io/gslog/gcp"
	"m4o.io/gslog/internal/entry"
	"m4o.io/gslog/internal/testhelp"
)

const reportStack = "goroutine 7 [running]:\nmain.work()\n\t/app/work.go:12 +0x1c\n"

var _ = Describe("The error report", func() {
	var (
		entries []logging.Entry
		version string
		logger  *slog.Logger
	)

	serviceContext := map[string]any{"service": "svc", "version": "v1"}

	// logAt writes one record with the logger at the level.  logAt returns
	// the payload of the entry.
	logAt := func(l *slog.Logger, level slog.Level, args ...any) map[string]any {
		GinkgoHelper()

		l.Log(context.Background(), level, "message", args...)

		Expect(entries).To(HaveLen(1))

		payload, ok := entries[0].Payload.(*structpb.Struct)
		Expect(ok).To(BeTrue())

		return payload.AsMap()
	}

	BeforeEach(func() {
		entries = nil
		version = "v1"
	})

	JustBeforeEach(func() {
		sink := gcp.LoggerFunc(func(e logging.Entry) {
			entries = append(entries, e)
		})
		report := entry.ErrorReport{Service: "svc", Version: version, StackTrace: reportStack}
		option := testhelp.WithErrorReport(report)

		h := gcp.NewHandler(sink, option)
		logger = slog.New(h)
	})

	When("the record is at level Error", func() {
		It("is written at the top level, outside the open group", func() {
			grouped := logger.WithGroup("g")

			payload := logAt(grouped, slog.LevelError, "k", "v")

			Expect(payload).To(HaveKeyWithValue("@type", entry.ReportTypeValue))
			Expect(payload).To(HaveKeyWithValue("serviceContext", serviceContext))
			Expect(payload).To(HaveKeyWithValue("stack_trace", reportStack))
			Expect(payload).To(HaveKeyWithValue("g", map[string]any{"k": "v"}))
		})
	})

	When("the record is below level Error", func() {
		It("is not written", func() {
			payload := logAt(logger, slog.LevelWarn)

			Expect(payload).NotTo(HaveKey("@type"))
			Expect(payload).NotTo(HaveKey("serviceContext"))
			Expect(payload).NotTo(HaveKey("stack_trace"))
		})
	})

	When("the report has no version", func() {
		BeforeEach(func() {
			version = ""
		})

		It("has a service context with no version", func() {
			payload := logAt(logger, slog.LevelError)

			Expect(payload).To(HaveKeyWithValue("serviceContext", map[string]any{"service": "svc"}))
		})
	})

	Describe("an attribute with a key of the error report", func() {
		JustBeforeEach(func() {
			logger = logger.With("stack_trace", "with")
		})

		It("is replaced in a record with a report", func() {
			payload := logAt(logger, slog.LevelError, "@type", "user")

			Expect(payload).To(HaveKeyWithValue("@type", entry.ReportTypeValue))
			Expect(payload).To(HaveKeyWithValue("serviceContext", serviceContext))
			Expect(payload).To(HaveKeyWithValue("stack_trace", reportStack))
		})

		It("is written in a record with no report", func() {
			payload := logAt(logger, slog.LevelInfo, "@type", "user")

			Expect(payload).To(HaveKeyWithValue("@type", "user"))
			Expect(payload).To(HaveKeyWithValue("stack_trace", "with"))
			Expect(payload).NotTo(HaveKey("serviceContext"))
		})
	})

	Describe("a group of WithGroup with a key of the error report", func() {
		JustBeforeEach(func() {
			logger = logger.WithGroup("serviceContext")
		})

		It("is replaced in a record with a report", func() {
			payload := logAt(logger, slog.LevelError, "k", "v")

			Expect(payload).To(HaveKeyWithValue("serviceContext", serviceContext))
		})

		It("is written in a record with no report", func() {
			payload := logAt(logger, slog.LevelInfo, "k", "v")

			Expect(payload).To(HaveKeyWithValue("serviceContext", map[string]any{"k": "v"}))
		})
	})
})
