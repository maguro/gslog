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

package gcp

import (
	"context"

	"cloud.google.com/go/logging"
)

type httpRequestKey struct{}

// WithHTTPRequest returns a new Context that stores the HTTP request.  The
// handler sets the HTTPRequest field of the logging.Entry to the request for
// each record that a log call writes with that context.  The handler does not
// examine the request.
func WithHTTPRequest(ctx context.Context, request *logging.HTTPRequest) context.Context {
	return context.WithValue(ctx, httpRequestKey{}, request)
}

// httpRequestFrom returns the HTTP request that WithHTTPRequest stored in
// ctx.  httpRequestFrom returns nil when ctx has no request.
func httpRequestFrom(ctx context.Context) *logging.HTTPRequest {
	request, _ := ctx.Value(httpRequestKey{}).(*logging.HTTPRequest)

	return request
}
