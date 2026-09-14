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

// Package mapper contains the attribute mapper that rewrites attributes
// before a handler logs them.
package mapper

import "log/slog"

// Mapper rewrites each non-group attribute before the handler logs the
// attribute.
type Mapper func(groups []string, attr slog.Attr) slog.Attr

// Wrap wraps a mapper with checks for empty groups.  The wrapper elides an
// empty group.
func Wrap(mapper Mapper) Mapper {
	if mapper == nil {
		return nil
	}

	var wrapped Mapper

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

				// elide empty attributes
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
