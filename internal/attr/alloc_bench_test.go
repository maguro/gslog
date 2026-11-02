// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package attr_test

import (
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	"m4o.io/gslog/internal/attr"
)

var sink *structpb.Value

func BenchmarkNewStringValue(b *testing.B) {
	b.ReportAllocs()

	for range b.N {
		sink = attr.NewStringValue("hello")
	}
}

func BenchmarkNewNumberValue(b *testing.B) {
	b.ReportAllocs()

	for range b.N {
		sink = attr.NewNumberValue(42)
	}
}

func BenchmarkNewBoolValue(b *testing.B) {
	b.ReportAllocs()

	for range b.N {
		sink = attr.NewBoolValue(true)
	}
}
