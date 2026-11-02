// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

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
