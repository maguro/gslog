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

package attr_test

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"m4o.io/gslog/internal/attr"
)

func TestCombinedValueEquivalence(t *testing.T) {
	inner := &structpb.Struct{Fields: map[string]*structpb.Value{
		"k": {Kind: &structpb.Value_StringValue{StringValue: "v"}},
	}}

	tests := []struct {
		name string
		got  *structpb.Value
		want *structpb.Value
	}{
		{"string", attr.NewStringValue("hello"), &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: "hello"}}},
		{"number", attr.NewNumberValue(3.5), &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: 3.5}}},
		{"bool", attr.NewBoolValue(true), &structpb.Value{Kind: &structpb.Value_BoolValue{BoolValue: true}}},
		{"struct", attr.NewStructValue(inner), &structpb.Value{Kind: &structpb.Value_StructValue{StructValue: inner}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, proto.Equal(tt.got, tt.want), "proto.Equal")

			gotJSON, err := protojson.Marshal(tt.got)
			require.NoError(t, err)
			wantJSON, err := protojson.Marshal(tt.want)
			require.NoError(t, err)
			assert.Equal(t, string(wantJSON), string(gotJSON), "protojson.Marshal")
		})
	}
}

func TestCombinedValueCloneIsIndependent(t *testing.T) {
	orig := attr.NewStringValue("before")

	//nolint:forcetypeassert
	cl := proto.Clone(orig).(*structpb.Value)
	require.True(t, proto.Equal(orig, cl))

	cl.Kind.(*structpb.Value_StringValue).StringValue = "after"
	assert.Equal(t, "before", orig.GetStringValue())
	assert.Equal(t, "after", cl.GetStringValue())

	origKind := unsafe.Pointer(orig.Kind.(*structpb.Value_StringValue))
	clKind := unsafe.Pointer(cl.Kind.(*structpb.Value_StringValue))
	assert.NotEqual(t, origKind, clKind, "clone must not share the wrapper")
}

// TestCombinedValueWrapperIsSameObject verifies that the wrapper address
// lies inside the object that holds the value.  The value keeps the wrapper
// alive.
func TestCombinedValueWrapperIsSameObject(t *testing.T) {
	v := attr.NewStringValue("hello")

	base := uintptr(unsafe.Pointer(v))
	kind := uintptr(unsafe.Pointer(v.Kind.(*structpb.Value_StringValue)))
	assert.Equal(t, unsafe.Sizeof(structpb.Value{}), kind-base, "wrapper sits right after the Value")

	for range 10 {
		runtime.GC()
	}

	assert.Equal(t, "hello", v.GetStringValue())
}
