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

	"cloud.google.com/go/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"m4o.io/gslog"
)

var _ = Describe("Kubernetes podinfo labels", func() {
	var ctx context.Context
	var options *gslog.Options
	var root string

	BeforeEach(func() {
		ctx = context.Background()
		options = &gslog.Options{}
		Ω(1).Should(Equal(1))
	})

	JustBeforeEach(func() {
		err := gslog.WithPodinfoLabels(root).Process(options)
		Ω(err).ShouldNot(HaveOccurred())
	})

	When("the podinfo labels file exists", func() {
		BeforeEach(func() {
			root = "testdata/etc/podinfo"
		})

		It("the labels are loaded and properly prefixed",
			func() {
				e := &logging.Entry{}
				for _, a := range options.EntryAugmentors {
					a(ctx, e)
				}

				Ω(e.Labels).Should(MatchAllKeys(Keys{
					gslog.K8sPodPrefix + "app":         Equal("hello-world"),
					gslog.K8sPodPrefix + "environment": Equal("stg"),
					gslog.K8sPodPrefix + "tier":        Equal("backend"),
					gslog.K8sPodPrefix + "track":       Equal("stable"),
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
				for _, a := range options.EntryAugmentors {
					a(ctx, e)
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
				for _, a := range options.EntryAugmentors {
					a(ctx, e)
				}

				Ω(e.Labels).Should(BeEmpty())
			})
	})
})
