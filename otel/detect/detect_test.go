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

package detect

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace"

	"m4o.io/gslog/internal/entry"
	"m4o.io/gslog/internal/options"
)

// fakeSource returns a source with a fixed answer from the metadata server
// and a fixed value of GOOGLE_CLOUD_PROJECT.
func fakeSource(id string, err error, env string) projectSource {
	return projectSource{
		metadata: func(_ context.Context, path string) (string, error) {
			if path != projectIDPath {
				return "", errors.New("unexpected path " + path)
			}

			return id, err
		},
		getenv: func(name string) string {
			if name != projectIDEnvVar {
				return ""
			}

			return env
		},
	}
}

// traceOf applies the option and returns the entry for a context that has a
// sampled span.
func traceOf(t *testing.T, option func(*options.Options)) entry.Entry {
	t.Helper()

	traceID, err := trace.TraceIDFromHex("52fc1643a9381fc674742bb0067101e7")
	assert.NoError(t, err)

	spanID, err := trace.SpanIDFromHex("d3e9e8c51cb190df")
	assert.NoError(t, err)

	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), spanContext)

	var o options.Options

	option(&o)

	var e entry.Entry

	for _, augment := range o.EntryAugmentors {
		augment(ctx, &e)
	}

	return e
}

func TestDetectProjectID(t *testing.T) {
	unreachable := errors.New("no metadata server")

	for name, tt := range map[string]struct {
		source projectSource
		want   string
	}{
		"metadata server":                {fakeSource("from-metadata\n", nil, "from-env"), "from-metadata"},
		"metadata server has no ID":      {fakeSource("  ", nil, "from-env"), "from-env"},
		"no metadata server":             {fakeSource("", unreachable, "from-env"), "from-env"},
		"no metadata server, no env":     {fakeSource("", unreachable, ""), ""},
		"error with a value is not used": {fakeSource("stale", unreachable, ""), ""},
	} {
		t.Run(name, func(t *testing.T) {
			got := detectProjectID(tt.source)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWithOtelTracing_detectedProjectID(t *testing.T) {
	source := fakeSource("detected", nil, "")

	e := traceOf(t, withOtelTracing(source))

	assert.Equal(t, "projects/detected/traces/52fc1643a9381fc674742bb0067101e7", e.Trace)
	assert.Equal(t, "d3e9e8c51cb190df", e.SpanID)
	assert.True(t, e.TraceSampled)
}

func TestWithOtelTracing_noProjectID(t *testing.T) {
	source := fakeSource("", errors.New("no metadata server"), "")

	e := traceOf(t, withOtelTracing(source))

	assert.Empty(t, e.Trace)
	assert.Empty(t, e.SpanID)
	assert.False(t, e.TraceSampled)
}

// The source of WithOtelTracing reads the metadata server at the host in
// GCE_METADATA_HOST.
func TestWithOtelTracing_metadataServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/computeMetadata/v1/"+projectIDPath || r.Header.Get("Metadata-Flavor") != "Google" {
			http.NotFound(w, r)

			return
		}

		w.Header().Set("Metadata-Flavor", "Google")
		_, _ = w.Write([]byte("from-server\n"))
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	t.Setenv("GCE_METADATA_HOST", host)

	e := traceOf(t, WithOtelTracing())

	assert.Equal(t, "projects/from-server/traces/52fc1643a9381fc674742bb0067101e7", e.Trace)
}
