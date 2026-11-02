// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package gslog_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGo(t *testing.T) {
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	reporterConfig.Verbose = true
	RunSpecs(t, "gslog Suite", suiteConfig, reporterConfig)
}
