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

package stdout_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"m4o.io/gslog/core"
	"m4o.io/gslog/internal/entry"
	"m4o.io/gslog/internal/testhelp"
	"m4o.io/gslog/stdout"
)

const reportStack = "goroutine 7 [running]:\nmain.work()\n\t/app/work.go:12 +0x1c\n"

var _ = Describe("The error report", func() {
	var (
		buf     *bytes.Buffer
		version string
		opts    []core.Option
		logger  *slog.Logger
	)

	serviceContext := map[string]any{"service": "svc", "version": "v1"}

	// logAt writes one record with the logger at the level.  logAt returns
	// the line and the members of the line.
	logAt := func(l *slog.Logger, level slog.Level, args ...any) (string, map[string]any) {
		GinkgoHelper()

		l.Log(context.Background(), level, "message", args...)

		line := buf.String()
		Expect(line).To(HaveSuffix("\n"))
		Expect(strings.Count(line, "\n")).To(Equal(1))

		var members map[string]any

		Expect(json.Unmarshal([]byte(line), &members)).To(Succeed())

		return line, members
	}

	BeforeEach(func() {
		buf = &bytes.Buffer{}
		version = "v1"
		opts = nil
	})

	JustBeforeEach(func() {
		report := entry.ErrorReport{Service: "svc", Version: version, StackTrace: reportStack}
		option := testhelp.WithErrorReport(report)
		all := append([]core.Option{option}, opts...)

		h := stdout.NewHandler(buf, all...)
		logger = slog.New(h)
	})

	When("the record is at level Error", func() {
		It("is written at the top level, outside the open group", func() {
			grouped := logger.WithGroup("g")

			_, members := logAt(grouped, slog.LevelError, "k", "v")

			Expect(members).To(HaveKeyWithValue("severity", "ERROR"))
			Expect(members).To(HaveKeyWithValue("@type", entry.ReportTypeValue))
			Expect(members).To(HaveKeyWithValue("serviceContext", serviceContext))
			Expect(members).To(HaveKeyWithValue("stack_trace", reportStack))
			Expect(members).To(HaveKeyWithValue("g", map[string]any{"k": "v"}))
		})
	})

	When("the record is below level Error", func() {
		It("is not written", func() {
			_, members := logAt(logger, slog.LevelWarn)

			Expect(members).NotTo(HaveKey("@type"))
			Expect(members).NotTo(HaveKey("serviceContext"))
			Expect(members).NotTo(HaveKey("stack_trace"))
		})
	})

	When("the report has no version", func() {
		BeforeEach(func() {
			version = ""
		})

		It("has a service context with no version", func() {
			_, members := logAt(logger, slog.LevelError)

			Expect(members).To(HaveKeyWithValue("serviceContext", map[string]any{"service": "svc"}))
		})
	})

	Describe("an attribute with a key of the error report", func() {
		Context("from WithAttrs and from the log call", func() {
			JustBeforeEach(func() {
				logger = logger.With("@type", "with", "stack_trace", "with")
			})

			It("is replaced in a record with a report", func() {
				line, members := logAt(logger, slog.LevelError,
					"@type", "user", "stack_trace", "user", slog.Group("serviceContext", "service", "user"))

				Expect(strings.Count(line, `"@type"`)).To(Equal(1))
				Expect(strings.Count(line, `"stack_trace"`)).To(Equal(1))
				Expect(strings.Count(line, `"serviceContext"`)).To(Equal(1))

				Expect(members).To(HaveKeyWithValue("@type", entry.ReportTypeValue))
				Expect(members).To(HaveKeyWithValue("serviceContext", serviceContext))
				Expect(members).To(HaveKeyWithValue("stack_trace", reportStack))
			})

			It("is written in a record with no report", func() {
				_, members := logAt(logger, slog.LevelInfo, slog.Group("serviceContext", "service", "user"))

				Expect(members).To(HaveKeyWithValue("@type", "with"))
				Expect(members).To(HaveKeyWithValue("stack_trace", "with"))
				Expect(members).To(HaveKeyWithValue("serviceContext", map[string]any{"service": "user"}))
			})
		})

		Context("in an inline group of WithAttrs", func() {
			JustBeforeEach(func() {
				logger = logger.With(slog.Group("", "stack_trace", "with", "k", "v"))
			})

			It("is replaced in a record with a report", func() {
				line, members := logAt(logger, slog.LevelError)

				Expect(strings.Count(line, `"stack_trace"`)).To(Equal(1))
				Expect(members).To(HaveKeyWithValue("stack_trace", reportStack))
				Expect(members).To(HaveKeyWithValue("k", "v"))
			})

			It("is written in a record with no report", func() {
				_, members := logAt(logger, slog.LevelInfo)

				Expect(members).To(HaveKeyWithValue("stack_trace", "with"))
				Expect(members).To(HaveKeyWithValue("k", "v"))
			})
		})

		Context("below the top level", func() {
			JustBeforeEach(func() {
				logger = logger.With(slog.Group("g", "stack_trace", "with"))
				logger = logger.WithGroup("h").With("serviceContext", "with")
			})

			It("is kept in a record with a report", func() {
				_, members := logAt(logger, slog.LevelError, "@type", "user")

				Expect(members).To(HaveKeyWithValue("g", map[string]any{"stack_trace": "with"}))
				Expect(members).To(HaveKeyWithValue("h", map[string]any{"serviceContext": "with", "@type": "user"}))
				Expect(members).To(HaveKeyWithValue("@type", entry.ReportTypeValue))
			})
		})

		Context("from two calls of WithAttrs", func() {
			var child, sibling *slog.Logger

			JustBeforeEach(func() {
				parent := logger.With("stack_trace", "parent")
				child = parent.With("@type", "child", "k", "v")
				sibling = parent.With("@type", "sibling")
			})

			It("is written for the child in a record with no report", func() {
				_, members := logAt(child, slog.LevelInfo)

				Expect(members).To(HaveKeyWithValue("stack_trace", "parent"))
				Expect(members).To(HaveKeyWithValue("@type", "child"))
				Expect(members).To(HaveKeyWithValue("k", "v"))
			})

			It("is written for the sibling without the attributes of the child", func() {
				_, members := logAt(sibling, slog.LevelInfo)

				Expect(members).To(HaveKeyWithValue("stack_trace", "parent"))
				Expect(members).To(HaveKeyWithValue("@type", "sibling"))
				Expect(members).NotTo(HaveKey("k"))
			})

			It("is replaced for the child in a record with a report", func() {
				line, members := logAt(child, slog.LevelError)

				Expect(strings.Count(line, `"@type"`)).To(Equal(1))
				Expect(strings.Count(line, `"stack_trace"`)).To(Equal(1))

				Expect(members).To(HaveKeyWithValue("@type", entry.ReportTypeValue))
				Expect(members).To(HaveKeyWithValue("k", "v"))
			})
		})

		Context("on a group of WithAttrs with no member", func() {
			JustBeforeEach(func() {
				logger = logger.With("stack_trace", "with").With(slog.Group("serviceContext"))
			})

			It("is not written", func() {
				_, members := logAt(logger, slog.LevelInfo)

				Expect(members).To(HaveKeyWithValue("stack_trace", "with"))
				Expect(members).NotTo(HaveKey("serviceContext"))
			})
		})
	})

	Describe("a group of WithGroup with a key of the error report", func() {
		JustBeforeEach(func() {
			logger = logger.WithGroup("serviceContext").With("with", "w")
		})

		It("is replaced in a record with a report", func() {
			line, members := logAt(logger, slog.LevelError, "k", "v")

			Expect(strings.Count(line, `"serviceContext"`)).To(Equal(1))
			Expect(members).To(HaveKeyWithValue("serviceContext", serviceContext))
		})

		It("is written in a record with no report", func() {
			_, members := logAt(logger, slog.LevelInfo, "k", "v")

			Expect(members).To(HaveKeyWithValue("serviceContext", map[string]any{"with": "w", "k": "v"}))
		})
	})

	Describe("a message that the mapper renames to a key of the error report", func() {
		BeforeEach(func() {
			rename := func(_ []string, a slog.Attr) slog.Attr {
				if a.Key == core.MessageKey {
					a.Key = "stack_trace"
				}

				return a
			}

			opts = []core.Option{core.WithReplaceAttr(rename)}
		})

		It("is not written in a record with a report", func() {
			line, members := logAt(logger, slog.LevelError)

			Expect(strings.Count(line, `"stack_trace"`)).To(Equal(1))
			Expect(members).To(HaveKeyWithValue("stack_trace", reportStack))
		})

		It("is written in a record with no report", func() {
			_, members := logAt(logger, slog.LevelInfo)

			Expect(members).To(HaveKeyWithValue("stack_trace", "message"))
		})
	})
})
