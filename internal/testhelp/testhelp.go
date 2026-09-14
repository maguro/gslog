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

// Package testhelp contains helpers that the handler tests share.
package testhelp

import (
	"runtime"
	"time"
)

// Time is the fixed time of the test records.
//
//nolint:gochecknoglobals
var Time = time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)

// CallerPC returns the program counter at the given stack depth.  Depth 1 is
// the caller of CallerPC.
func CallerPC(depth int) uintptr {
	var pcs [1]uintptr

	runtime.Callers(depth, pcs[:])

	return pcs[0]
}
