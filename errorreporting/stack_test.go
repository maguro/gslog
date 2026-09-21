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

package errorreporting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrimStack(t *testing.T) {
	const (
		header = "goroutine 1 [running]:\n"
		gslog  = "m4o.io/gslog/errorreporting.captureStack()\n\t/g/stack.go:1 +0x1\n"
		slog   = "log/slog.(*Logger).log(0x1, 0x2)\n\t/s/logger.go:2 +0x2\n"
		test   = "m4o.io/gslog/errorreporting_test.TestX(0x3)\n\t/t/x_test.go:3 +0x3\n"
		user   = "main.main()\n\t/u/main.go:4 +0x4\n"
		wrap   = "example.com/wrap.(*Handler).Handle(0x5)\n\t/w/wrap.go:5 +0x5\n"
		elided = "...additional frames elided...\n"
	)

	for name, tt := range map[string]struct {
		in, want string
	}{
		"gslog then slog then user":  {header + gslog + gslog + slog + slog + user, header + user},
		"gslog then slog then test":  {header + gslog + slog + test + user, header + test + user},
		"gslog only at the top":      {header + gslog + user, header + user},
		"slog only at the top":       {header + slog + user, header + user},
		"no frame to remove":         {header + user, header + user},
		"all frames removed":         {header + gslog + slog, header + gslog + slog},
		"header only":                {header, header},
		"no line end":                {"goroutine 1 [running]:", "goroutine 1 [running]:"},
		"elided line kept":           {header + slog + user + elided, header + user + elided},
		"slog before gslog not seen": {header + slog + gslog + user, header + gslog + user},
		"handler before slog":        {header + gslog + wrap + wrap + slog + user, header + user},
		"handler and no slog":        {header + gslog + wrap + user, header + wrap + user},
		"first slog frames only":     {header + gslog + slog + user + slog + user, header + user + slog + user},
	} {
		t.Run(name, func(t *testing.T) {
			got := trimStack([]byte(tt.in))
			assert.Equal(t, tt.want, got)
		})
	}
}
