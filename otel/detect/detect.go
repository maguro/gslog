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

// Package detect contains an option that includes OpenTelemetry tracing with
// a project ID from the environment of the process.
package detect

import (
	"context"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"

	"m4o.io/gslog/core"
	"m4o.io/gslog/internal/options"
	"m4o.io/gslog/otel"
)

const (
	projectIDEnvVar = "GOOGLE_CLOUD_PROJECT"
	projectIDPath   = "project/project-id"

	dialTimeout   = 1 * time.Second
	detectTimeout = 2 * time.Second
)

// WithOtelTracing returns the option of otel.WithOtelTracing with the project
// ID of the environment.  WithOtelTracing gets the project ID from the
// metadata server, as the client of cloud.google.com/go/logging does.  If the
// metadata server gives no project ID, WithOtelTracing uses the environment
// variable GOOGLE_CLOUD_PROJECT.  The detection can take two seconds when
// there is no metadata server.  If WithOtelTracing finds no project ID, the
// handler includes no tracing.
func WithOtelTracing() core.Option {
	source := newProjectSource()

	return withOtelTracing(source)
}

// withOtelTracing is WithOtelTracing with the source for the detection of
// the project ID.
func withOtelTracing(source projectSource) core.Option {
	projectID := detectProjectID(source)
	if projectID == "" {
		return func(*options.Options) {}
	}

	return otel.WithOtelTracing(projectID)
}

// projectSource holds the functions that detectProjectID calls.  metadata
// returns the value at a path of the metadata server.  getenv returns the
// value of an environment variable.
type projectSource struct {
	metadata func(ctx context.Context, path string) (string, error)
	getenv   func(name string) string
}

// newProjectSource returns the source that reads the metadata server and the
// environment of the process.
func newProjectSource() projectSource {
	dialer := &net.Dialer{Timeout: dialTimeout}
	transport := &http.Transport{DialContext: dialer.DialContext}
	client := metadata.NewClient(&http.Client{Transport: transport})

	return projectSource{
		metadata: client.GetWithContext,
		getenv:   os.Getenv,
	}
}

// detectProjectID returns the project ID from the metadata server.  If the
// metadata server gives no project ID, detectProjectID returns the value of
// the environment variable GOOGLE_CLOUD_PROJECT.  detectProjectID returns an
// empty string when it finds no project ID.
func detectProjectID(source projectSource) string {
	ctx, cancel := context.WithTimeout(context.Background(), detectTimeout)
	defer cancel()

	id, err := source.metadata(ctx, projectIDPath)
	if err == nil {
		id = strings.TrimSpace(id)
		if id != "" {
			return id
		}
	}

	return source.getenv(projectIDEnvVar)
}
