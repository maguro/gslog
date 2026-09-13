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

package attr_test

import (
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	"m4o.io/gslog/internal/attr"
)

//nolint:gochecknoglobals
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
