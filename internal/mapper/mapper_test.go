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

package mapper_test

import (
	"log/slog"
	"reflect"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"m4o.io/gslog/internal/mapper"
)

type attrFunc func(a slog.Attr) slog.Attr

func removeMapper(_ slog.Attr) slog.Attr {
	return slog.Attr{}
}

func genReplace(r slog.Attr, groups ...string) mapper.Mapper {
	return func(g []string, a slog.Attr) slog.Attr {
		if reflect.DeepEqual(groups, g) {
			return r
		}
		return a
	}
}

func genMapper(fn attrFunc, groups []string, keys ...string) mapper.Mapper {
	return func(g []string, a slog.Attr) slog.Attr {
		for _, key := range keys {
			if reflect.DeepEqual(groups, g) && a.Key == key {
				return fn(a)
			}
		}
		return a
	}
}

func groups(groups ...string) []string {
	return groups
}

func TestWrap(t *testing.T) {
	tests := map[string]struct {
		groups   []string
		attr     slog.Attr
		mapper   mapper.Mapper
		expected slog.Attr
	}{
		"simple replacement":  {nil, slog.Int("a", 1), genReplace(slog.Int("b", 2)), slog.Int("b", 2)},
		"inside group":        {groups("g", "h"), slog.Int("a", 1), genReplace(slog.Int("b", 2), "g", "h"), slog.Int("b", 2)},
		"with group":          {groups("g"), slog.Group("h", slog.Int("a", 1)), genReplace(slog.Int("b", 2), "g", "h"), slog.Group("h", slog.Int("b", 2))},
		"group replace":       {groups("g"), slog.Group("h", slog.Int("a", 1), slog.Int("b", 2)), genMapper(removeMapper, groups("g", "h"), "a"), slog.Group("h", slog.Int("b", 2))},
		"group replace empty": {groups("g"), slog.Group("h", slog.Int("a", 1)), genReplace(slog.Attr{}, "g", "h"), slog.Attr{}},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			m := mapper.Wrap(tc.mapper)
			actual := m(tc.groups, tc.attr)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestWrap_nil(t *testing.T) {
	assert.Nil(t, mapper.Wrap(nil))
}

type groupValuer struct{}

func (groupValuer) LogValue() slog.Value {
	return slog.GroupValue(slog.String("password", "s3cret"), slog.Int("n", 1))
}

type stringValuer struct{}

func (stringValuer) LogValue() slog.Value {
	return slog.StringValue("v")
}

// TestWrap_resolvesLogValuer verifies that the wrapper resolves a LogValuer
// before it inspects the kind.  The mapper then sees the members of a group
// value and the kind of a scalar value.
func TestWrap_resolvesLogValuer(t *testing.T) {
	m := mapper.Wrap(genMapper(removeMapper, groups("h"), "password"))
	actual := m(nil, slog.Any("h", groupValuer{}))

	assert.Equal(t, slog.Group("h", slog.Int("n", 1)), actual)

	var kind slog.Kind

	m = mapper.Wrap(func(_ []string, a slog.Attr) slog.Attr {
		kind = a.Value.Kind()

		return a
	})
	m(nil, slog.Any("s", stringValuer{}))

	assert.Equal(t, slog.KindString, kind)
}

// TestWrap_doesNotWriteIntoCallerGroups verifies that the wrapper does not
// write into the spare capacity of the groups slice.  Handlers share that
// slice between concurrent calls.
func TestWrap_doesNotWriteIntoCallerGroups(t *testing.T) {
	backing := [2]string{"g", "untouched"}
	groups := backing[:1]

	var seen []string

	m := mapper.Wrap(func(g []string, a slog.Attr) slog.Attr {
		seen = slices.Clone(g)

		return a
	})

	m(groups, slog.Group("h", slog.Int("a", 1)))

	assert.Equal(t, []string{"g", "h"}, seen)
	assert.Equal(t, "untouched", backing[1])
}
