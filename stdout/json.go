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

package stdout

import (
	"log/slog"
	"maps"
	"math"
	"slices"
	"strconv"
	"unicode/utf8"
)

const (
	hexDigits        = "0123456789abcdef"
	unicodeEscapeLen = 4
	decimalBase      = 10
	float64Bits      = 64
)

// appendLabels appends the labels as a JSON object with sorted keys.
func appendLabels(buf []byte, labels map[string]string) []byte {
	buf = append(buf, '{')

	for i, key := range slices.Sorted(maps.Keys(labels)) {
		if i > 0 {
			buf = append(buf, ',')
		}

		buf = appendKey(buf, key)
		buf = appendString(buf, labels[key])
	}

	return append(buf, '}')
}

// appendSourceLocation appends the source location as a JSON object.  The
// line is a string, as the agent expects.
func appendSourceLocation(buf []byte, loc *slog.Source) []byte {
	buf = append(buf, '{')
	buf = appendKey(buf, "file")
	buf = appendString(buf, loc.File)
	buf = append(buf, ',')
	buf = appendKey(buf, "line")
	buf = append(buf, '"')
	buf = strconv.AppendInt(buf, int64(loc.Line), decimalBase)
	buf = append(buf, '"', ',')
	buf = appendKey(buf, "function")
	buf = appendString(buf, loc.Function)

	return append(buf, '}')
}

// appendNumber appends the number in the format that encoding/json uses.
// appendNumber writes NaN and the infinities as strings.
func appendNumber(buf []byte, f float64) []byte {
	switch {
	case math.IsNaN(f):
		return appendString(buf, "NaN")
	case math.IsInf(f, 1):
		return appendString(buf, "Infinity")
	case math.IsInf(f, -1):
		return appendString(buf, "-Infinity")
	}

	const (
		lowExponent  = 1e-6
		highExponent = 1e21
	)

	format := byte('f')

	if abs := math.Abs(f); abs != 0 && (abs < lowExponent || abs >= highExponent) {
		format = 'e'
	}

	start := len(buf)
	buf = strconv.AppendFloat(buf, f, format, -1, float64Bits)

	if format == 'e' {
		buf = trimExponent(buf, start)
	}

	return buf
}

// trimExponent removes the leading zero from a two-digit exponent.  The
// exponent "e-09" becomes "e-9".  The number starts at start.
func trimExponent(buf []byte, start int) []byte {
	const exponentLen = 4

	n := len(buf)
	if n-start < exponentLen {
		return buf
	}

	if buf[n-exponentLen] == 'e' && buf[n-exponentLen+1] == '-' && buf[n-exponentLen+2] == '0' {
		buf[n-exponentLen+2] = buf[n-1]
		buf = buf[:n-1]
	}

	return buf
}

// appendKey appends the key as a JSON string followed by a colon.
func appendKey(buf []byte, key string) []byte {
	buf = appendString(buf, key)

	return append(buf, ':')
}

// appendString appends s as a JSON string.  Invalid UTF-8 bytes become the
// Unicode replacement character.
func appendString(buf []byte, s string) []byte {
	buf = append(buf, '"')
	start := 0

	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if b >= ' ' && b != '"' && b != '\\' {
				i++

				continue
			}

			buf = append(buf, s[start:i]...)
			buf = appendEscape(buf, b)
			i++
			start = i

			continue
		}

		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			buf = append(buf, s[start:i]...)
			buf = append(buf, `�`...)
			i += size
			start = i

			continue
		}

		i += size
	}

	buf = append(buf, s[start:]...)

	return append(buf, '"')
}

// appendEscape appends the JSON escape sequence for the byte b.
func appendEscape(buf []byte, b byte) []byte {
	switch b {
	case '"', '\\':
		return append(buf, '\\', b)
	case '\n':
		return append(buf, '\\', 'n')
	case '\r':
		return append(buf, '\\', 'r')
	case '\t':
		return append(buf, '\\', 't')
	default:
		buf = append(buf, '\\', 'u', '0', '0')

		return append(buf, hexDigits[b>>unicodeEscapeLen], hexDigits[b&0xF])
	}
}
