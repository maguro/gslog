// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

// Package otel contains options that include OpenTelemetry baggage and
// tracing in logging records.
//
// The options are in a separate package.  Because of this, a module that does
// not need OpenTelemetry tracing has fewer dependencies.
package otel
