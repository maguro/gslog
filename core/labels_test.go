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

package core_test

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"m4o.io/gslog/core"
)

var _ = Describe("core labels", func() {
	var ctx context.Context
	BeforeEach(func() {
		ctx = context.Background()
	})

	When("context is initialized with bad labels", func() {
		It("should panic", func() {
			Ω(func() {
				core.WithLabels(ctx, core.LabelPair{})
			}).Should(PanicWith("invalid label passed to WithLabels()"))
		})
	})

	When("context is initialized with several labels", func() {
		BeforeEach(func() {
			ctx = core.WithLabels(ctx,
				core.Label("how", "now"),
				core.Label("brown", "cow"),
			)
		})

		It("they can be extracted from the context", func() {
			labels := core.ExtractLabels(ctx)

			Ω(labels).Should(HaveLen(2))
			Ω(labels).Should(HaveKeyWithValue("how", "now"))
			Ω(labels).Should(HaveKeyWithValue("brown", "cow"))
		})

		Context("and a label overridden", func() {
			BeforeEach(func() {
				ctx = core.WithLabels(ctx, core.Label("brown", "cat"))
			})

			It("the overrides can be extracted from the context", func() {
				labels := core.ExtractLabels(ctx)

				Ω(labels).Should(HaveLen(2))
				Ω(labels).Should(HaveKeyWithValue("how", "now"))
				Ω(labels).Should(HaveKeyWithValue("brown", "cat"))
			})
		})
	})

	When("context is initialized with too many labels", func() {
		BeforeEach(func() {
			ctx = core.WithLabels(ctx,
				core.Label("how", "now"),
				core.Label("brown", "cow"),
			)
			for i := 0; i < 64; i++ {
				key := fmt.Sprintf("key_%06d", i)
				value := fmt.Sprintf("val_%06d", i)
				ctx = core.WithLabels(ctx,
					core.Label(key, value),
				)
			}
		})

		It("only 64 labels can be obtained from the context", func() {
			labels := core.ExtractLabels(ctx)

			Ω(labels).Should(HaveLen(64))
			Ω(labels).Should(HaveKeyWithValue("how", "now"))
			Ω(labels).Should(HaveKeyWithValue("brown", "cow"))
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

		ctx = core.WithLabels(ctx, core.Label(k, v))
		ctx = context.WithValue(ctx, mockKey{}, v)
	}

	for i := 1; i <= count; i++ {
		k := fmt.Sprintf("key_%06d", i)
		v := fmt.Sprintf("val_%06d", i)

		ctx = core.WithLabels(ctx, core.Label(k, v))
		ctx = context.WithValue(ctx, mockKey{}, v)
	}
}

func BenchmarkExtractLabels(b *testing.B) {
	core.ExtractLabels(ctx)
}
