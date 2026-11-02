// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

// Package gslog contains the options, labels, and levels that the gslog
// handlers for Google Cloud Logging share.
//
// The handlers are in the packages gcp and stdout.  This package and its
// dependencies import no module outside the standard library.  A program that
// uses only the stdout handler imports the packages gslog and stdout, and
// imports no client library.
package gslog
