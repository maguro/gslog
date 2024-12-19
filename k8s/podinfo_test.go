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

package k8s_test

import (
	"context"

	"cloud.google.com/go/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"m4o.io/gslog/internal/options"
	"m4o.io/gslog/k8s"
)

var _ = Describe("Kubernetes podinfo labels", func() {
	var ctx context.Context
	var o *options.Options
	var root string

	BeforeEach(func() {
		ctx = context.Background()
		o = &options.Options{}
	})

	JustBeforeEach(func() {
		k8s.WithPodinfoLabels(root)(o)
	})

	When("the podinfo labels file exists", func() {
		BeforeEach(func() {
			root = "testdata/etc/podinfo"
		})

		It("the labels are loaded and properly prefixed",
			func() {
				e := &logging.Entry{}
				for _, a := range o.EntryAugmentors {
					a(ctx, e, nil)
				}

				Ω(e.Labels).Should(MatchAllKeys(Keys{
					k8s.PodPrefix + "app":         Equal("hello-world"),
					k8s.PodPrefix + "environment": Equal("stg"),
					k8s.PodPrefix + "tier":        Equal("backend"),
					k8s.PodPrefix + "track":       Equal("stable"),
				}))
			})
	})

	When("the podinfo labels file does not exists", func() {
		BeforeEach(func() {
			root = "ouch"
		})

		It("no error occurs and no labels are loaded",
			func() {
				e := &logging.Entry{}
				for _, a := range o.EntryAugmentors {
					a(ctx, e, nil)
				}

				Ω(e.Labels).Should(BeEmpty())
			})
	})

	When("the podinfo labels file exists but contents are bad", func() {
		BeforeEach(func() {
			root = "testdata/ouch/podinfo"
		})

		It("no error occurs and no labels are loaded",
			func() {
				e := &logging.Entry{}
				for _, a := range o.EntryAugmentors {
					a(ctx, e, nil)
				}

				Ω(e.Labels).Should(BeEmpty())
			})
	})
})
