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

package gslog_test

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"m4o.io/gslog"
)

var _ = Describe("gslog labels", func() {
	var ctx context.Context
	BeforeEach(func() {
		ctx = context.Background()
	})

	When("context is initialized with bad labels", func() {
		BeforeEach(func() {
			ctx = gslog.WithLabels(ctx, gslog.LabelPair{})
		})

		It("should panic when extracting from the context", func() {
			Ω(func() {
				gslog.ExtractLabels(ctx)
			}).Should(PanicWith("invalid label passed to WithLabels()"))
		})
	})

	When("context is initialized with several labels", func() {
		BeforeEach(func() {
			ctx = gslog.WithLabels(ctx, gslog.Label("how", "now"), gslog.Label("brown", "cow"))
		})

		It("they can be extracted from the context", func() {
			lbls := gslog.ExtractLabels(ctx)

			Ω(lbls).Should(HaveLen(2))
			Ω(lbls).Should(HaveKeyWithValue("how", "now"))
			Ω(lbls).Should(HaveKeyWithValue("brown", "cow"))
		})

		Context("and a label overridden", func() {
			BeforeEach(func() {
				ctx = gslog.WithLabels(ctx, gslog.Label("brown", "cat"))
			})

			It("the overrides can be extracted from the context", func() {
				lbls := gslog.ExtractLabels(ctx)

				Ω(lbls).Should(HaveLen(2))
				Ω(lbls).Should(HaveKeyWithValue("how", "now"))
				Ω(lbls).Should(HaveKeyWithValue("brown", "cat"))
			})
		})
	})
})

const (
	count = 10
)

var (
	labels map[string]string
	ctx    context.Context
)

type mockKey struct{}

func init() {
	labels = make(map[string]string, count)
	for i := 1; i <= count; i++ {
		key := fmt.Sprintf("key_%06d", i)
		value := fmt.Sprintf("val_%06d", i)

		labels[key] = value
	}

	ctx = context.Background()

	for i := 1; i <= count; i++ {
		k := fmt.Sprintf("key_%06d", i)
		v := fmt.Sprintf("overridden_%06d", i)

		ctx = gslog.WithLabels(ctx, gslog.Label(k, v))
		ctx = context.WithValue(ctx, mockKey{}, v)
	}

	for i := 1; i <= count; i++ {
		k := fmt.Sprintf("key_%06d", i)
		v := fmt.Sprintf("val_%06d", i)

		ctx = gslog.WithLabels(ctx, gslog.Label(k, v))
		ctx = context.WithValue(ctx, mockKey{}, v)
	}
}

func BenchmarkExtractLabels(b *testing.B) {
	gslog.ExtractLabels(ctx)
}
