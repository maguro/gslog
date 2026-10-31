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

package detect_test

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"

	"m4o.io/gslog/otel/detect"
	"m4o.io/gslog/stdout"
)

// A service on Google Cloud gets its project ID from the metadata server.
// The program then needs no configuration for the trace prefix.  Off Google
// Cloud, the environment variable GOOGLE_CLOUD_PROJECT gives the project ID.
// The example has no output, because the project ID depends on the
// environment of the process.
func ExampleWithTracing() {
	h := stdout.NewHandler(os.Stdout, detect.WithTracing())
	logger := slog.New(h)

	traceID, _ := trace.TraceIDFromHex("52fc1643a9381fc674742bb0067101e7")
	spanID, _ := trace.SpanIDFromHex("d3e9e8c51cb190df")
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})

	ctx := trace.ContextWithSpanContext(context.Background(), spanContext)

	logger.InfoContext(ctx, "How now brown cow?")
}
