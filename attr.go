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

package gslog

import "m4o.io/gslog/core"

// MessageKey is the key that Google Cloud Logging specifies for the message
// of the log call.
//
// Deprecated: Use core.MessageKey.
const MessageKey = core.MessageKey

// AttrMapper rewrites each non-group attribute before the handler logs the
// attribute.
//
// Deprecated: Use core.AttrMapper.
type AttrMapper = core.AttrMapper
