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
	"log/slog"

	"cloud.google.com/go/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"m4o.io/gslog/v1"
	"m4o.io/gslog/v1/internal/level"
)

var _ = DescribeTable("Mapping slog.Level to logging.Severity",
	func(lvl slog.Level, expected logging.Severity) {
		Ω(level.LevelToSeverity(lvl)).Should(Equal(expected))
	},
	Entry("trace", slog.Level(-8), logging.Severity(0)),
	Entry("debug", slog.LevelDebug, logging.Debug),
	Entry("info", slog.LevelInfo, logging.Info),
	Entry("notice", gslog.LevelNotice, logging.Notice),
	Entry("warn", slog.LevelWarn, logging.Warning),
	Entry("error", slog.LevelError, logging.Error),
	Entry("critical", gslog.LevelCritical, logging.Critical),
	Entry("alert", gslog.LevelAlert, logging.Alert),
	Entry("emergency", gslog.LevelEmergency, logging.Emergency),
)
