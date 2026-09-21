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
	"bytes"
	"runtime"
	"sync"
)

const stackBufferSize = 16 << 10

var (
	newline     = []byte{'\n'}
	slogPrefix  = []byte("log/slog.")
	gslogPrefix = []byte("m4o.io/gslog/")
)

var stackPool = sync.Pool{
	New: func() any {
		buf := make([]byte, stackBufferSize)

		return &buf
	},
}

// captureStack returns the stack trace of the current goroutine in the
// format of runtime.Stack.  captureStack removes the frames of the log call
// at the top of the stack trace, as trimStack specifies.
func captureStack() string {
	buf, _ := stackPool.Get().(*[]byte)
	defer stackPool.Put(buf)

	n := runtime.Stack(*buf, false)

	return trimStack((*buf)[:n])
}

// trimStack returns the stack trace without the frames of the log call at
// the top.  trimStack removes the frames of the gslog module at the top.
// trimStack then removes the frames before the first frame of the log/slog
// package.  These frames are the frames of the handlers that call the gslog
// handler.  trimStack also removes that first frame and the frames of the
// log/slog package that follow that frame.  If the stack trace has no frame
// of the log/slog package, trimStack removes only the frames of the gslog
// module.  The result keeps the goroutine header line.  If no frame remains,
// trimStack returns the whole stack trace.
func trimStack(stack []byte) string {
	header, frames, found := bytes.Cut(stack, newline)
	if !found {
		return string(stack)
	}

	rest := skipFrames(frames, gslogPrefix)
	rest = skipToFrame(rest, slogPrefix)
	rest = skipFrames(rest, slogPrefix)

	if len(rest) == 0 {
		return string(stack)
	}

	return string(header) + "\n" + string(rest)
}

// skipFrames removes the first frame while the function line of that frame
// starts with prefix.  skipFrames returns the frames that remain.
func skipFrames(frames, prefix []byte) []byte {
	for bytes.HasPrefix(frames, prefix) {
		frames = nextFrame(frames)
	}

	return frames
}

// skipToFrame returns the frames from the first frame with a function line
// that starts with prefix.  If no frame has that function line, skipToFrame
// returns all the frames.
func skipToFrame(frames, prefix []byte) []byte {
	for rest := frames; len(rest) > 0; rest = nextFrame(rest) {
		if bytes.HasPrefix(rest, prefix) {
			return rest
		}
	}

	return frames
}

// nextFrame returns the frames that follow the first frame.  A frame is a
// function line and the lines that follow the function line.  The function
// line does not start with a tab.  The lines that follow the function line
// start with a tab.
func nextFrame(frames []byte) []byte {
	_, frames, _ = bytes.Cut(frames, newline)

	for len(frames) > 0 && frames[0] == '\t' {
		_, frames, _ = bytes.Cut(frames, newline)
	}

	return frames
}
