// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package gslog_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"m4o.io/gslog"
)

var _ = Describe("labels", func() {
	var ctx context.Context
	BeforeEach(func() {
		ctx = context.Background()
	})

	When("context is initialized with bad labels", func() {
		It("should panic", func() {
			Ω(func() {
				gslog.WithLabels(ctx, gslog.LabelPair{})
			}).Should(PanicWith("invalid label passed to WithLabels()"))
		})

		It("should panic on an empty key", func() {
			Ω(func() {
				gslog.Label("", "x")
			}).Should(PanicWith("label key is empty"))
		})
	})

	When("a label has bytes that are not valid UTF-8", func() {
		BeforeEach(func() {
			ctx = gslog.WithLabels(ctx, gslog.Label("k\xff", "v\xfe"))
		})

		It("the bytes are replaced", func() {
			labels := gslog.LabelsFrom(ctx)

			Ω(labels).Should(HaveKeyWithValue("k�", "v�"))
		})
	})

	When("context is initialized with several labels", func() {
		BeforeEach(func() {
			ctx = gslog.WithLabels(ctx,
				gslog.Label("how", "now"),
				gslog.Label("brown", "cow"),
			)
		})

		It("they can be extracted from the context", func() {
			labels := gslog.LabelsFrom(ctx)

			Ω(labels).Should(HaveLen(2))
			Ω(labels).Should(HaveKeyWithValue("how", "now"))
			Ω(labels).Should(HaveKeyWithValue("brown", "cow"))
		})

		Context("and a label overridden", func() {
			BeforeEach(func() {
				ctx = gslog.WithLabels(ctx, gslog.Label("brown", "cat"))
			})

			It("the overrides can be extracted from the context", func() {
				labels := gslog.LabelsFrom(ctx)

				Ω(labels).Should(HaveLen(2))
				Ω(labels).Should(HaveKeyWithValue("how", "now"))
				Ω(labels).Should(HaveKeyWithValue("brown", "cat"))
			})
		})
	})

	When("context is initialized with too many labels", func() {
		BeforeEach(func() {
			ctx = gslog.WithLabels(ctx,
				gslog.Label("how", "now"),
				gslog.Label("brown", "cow"),
			)
			for i := 0; i < 64; i++ {
				key := fmt.Sprintf("key_%06d", i)
				value := fmt.Sprintf("val_%06d", i)
				ctx = gslog.WithLabels(ctx,
					gslog.Label(key, value),
				)
			}
		})

		It("only 64 labels can be obtained from the context", func() {
			labels := gslog.LabelsFrom(ctx)

			Ω(labels).Should(HaveLen(64))
			Ω(labels).Should(HaveKeyWithValue("how", "now"))
			Ω(labels).Should(HaveKeyWithValue("brown", "cow"))
		})

		Context("and an existing label replaced", func() {
			BeforeEach(func() {
				ctx = gslog.WithLabels(ctx, gslog.Label("how", "then"))
			})

			It("the replacement is kept", func() {
				labels := gslog.LabelsFrom(ctx)

				Ω(labels).Should(HaveLen(64))
				Ω(labels).Should(HaveKeyWithValue("how", "then"))
			})
		})
	})
})
