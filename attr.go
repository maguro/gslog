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

package gslog

import (
	"m4o.io/gslog/internal/attr"
)

// AttrMapper is called to rewrite each non-group attribute before it is logged.
// The attribute's value has been resolved (see [Value.Resolve]).
// If replaceAttr returns a zero Attr, the attribute is discarded.
//
// The built-in attribute with key "message" is passed to this function.
//
// The first argument is a list of currently open groups that contain the
// Attr. It must not be retained or modified. replaceAttr is never called
// for Group attributes, only their contents. For example, the attribute
// list
//
//	Int("a", 1), Group("g", Int("b", 2)), Int("c", 3)
//
// results in consecutive calls to replaceAttr with the following arguments:
//
//	nil, Int("a", 1)
//	[]string{"g"}, Int("b", 2)
//	nil, Int("c", 3)
//
// AttrMapper can be used to change the default keys of the built-in
// attributes, convert types (for example, to replace a `time.Time` with the
// integer seconds since the Unix epoch), sanitize personal information, or
// remove attributes from the output.
type AttrMapper attr.Mapper
