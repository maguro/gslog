// Copyright the original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package mapper contains the wrapper of the attribute mapper that rewrites
// attributes before a handler logs them.
package mapper

import (
	"log/slog"

	"m4o.io/gslog"
)

// Wrap returns the mapper that a handler calls for each attribute.  The
// wrapper resolves the value of the attribute.  For an attribute that is not
// a group, the wrapper calls mapper.  For a group, the wrapper calls itself
// for each member, discards a member that maps to the zero Attr, and
// discards the group when no member remains.  Wrap returns nil for a nil
// mapper.
func Wrap(mapper gslog.AttrMapper) gslog.AttrMapper {
	if mapper == nil {
		return nil
	}

	var wrapped gslog.AttrMapper

	wrapped = func(groups []string, attr slog.Attr) slog.Attr {
		attr.Value = attr.Value.Resolve()

		if attr.Value.Kind() == slog.KindGroup {
			var attrs []slog.Attr

			// Concurrent calls can share the backing array of groups.
			path := make([]string, len(groups), len(groups)+1)
			copy(path, groups)
			path = append(path, attr.Key)

			for _, ga := range attr.Value.Group() {
				mapped := wrapped(path, ga)

				// The wrapper discards a member that maps to the zero Attr.
				if mapped.Key == "" && mapped.Value.Any() == nil {
					continue
				}

				attrs = append(attrs, mapped)
			}

			if len(attrs) == 0 {
				return slog.Attr{}
			}

			return slog.Attr{Key: attr.Key, Value: slog.GroupValue(attrs...)}
		}

		return mapper(groups, attr)
	}

	return wrapped
}
