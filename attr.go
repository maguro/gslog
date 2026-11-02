// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package gslog

import "log/slog"

// MessageKey is the key that Google Cloud Logging specifies for the message
// of the log call.  The value is a string.
const MessageKey = "message"

// AttrMapper rewrites each non-group attribute before the handler logs the
// attribute.  The handler resolves the value of the attribute before the
// call (see [slog.Value.Resolve]).  If the AttrMapper returns a zero Attr,
// the handler discards the attribute.
//
// The handler passes the built-in attribute with key "message" to this
// function.
//
// The first argument is a list of the open groups that contain the Attr.  Do
// not retain or modify this list.  The handler never calls the AttrMapper for
// a Group attribute.  The handler calls the AttrMapper for the contents of
// the group.  For example, the attribute list
//
//	Int("a", 1), Group("g", Int("b", 2)), Int("c", 3)
//
// results in consecutive calls to the AttrMapper with these arguments:
//
//	nil, Int("a", 1)
//	[]string{"g"}, Int("b", 2)
//	nil, Int("c", 3)
//
// An AttrMapper can change the default keys of the built-in attributes,
// convert types (for example, replace a `time.Time` with the integer seconds
// since the Unix epoch), sanitize personal information, or remove attributes
// from the output.
type AttrMapper func(groups []string, a slog.Attr) slog.Attr
