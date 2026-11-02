// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

// Package errorreporting contains an option for Google Cloud Error Reporting.
// The option causes the handler to write the fields that Error Reporting
// reads.  The handler writes the fields in each record at level Error or
// higher.  The package imports no module outside the standard library.
package errorreporting
