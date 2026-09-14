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

// Package timefmt formats times for log entries.
package timefmt

import "time"

// RFC3339InMs formats an instance of time.Time in the RFC3339 layout with
// millisecond resolution.  The function is optimized for speed.
func RFC3339InMs(t time.Time) string {
	// The layout is time.RFC3339Nano, truncated to millisecond resolution.
	const prefixLen = len("2006-01-02T15:04:05.000")

	// The layout trims trailing zeros.  The rounding of 1/10 millisecond
	// keeps exactly 4 digits after the period.
	const rounding = time.Millisecond / 10

	var arr [len(time.RFC3339Nano)]byte

	t = t.Truncate(time.Millisecond).Add(rounding)

	buf := t.AppendFormat(arr[:0], time.RFC3339Nano)
	buf = append(buf[:prefixLen], buf[prefixLen+1:]...) // drop the 4th digit

	return string(buf)
}
