// SPDX-FileCopyrightText: the original author or authors
// SPDX-License-Identifier: Apache-2.0

package errorreporting_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"m4o.io/gslog/errorreporting"
	"m4o.io/gslog/stdout"
)

// Error Reporting reads an entry that has the fields of a ReportedErrorEvent.
// The handler writes these fields in each record at level Error or higher.
// The Info record has no report fields.  The Error record has the type, the
// service context, and a stack trace.
func ExampleWithService() {
	var buf bytes.Buffer

	h := stdout.NewHandler(&buf, errorreporting.WithService("checkout", "v1.2.0"))
	logger := slog.New(h)

	logger.Info("Payment started")
	logger.Error("Payment failed")

	// The timestamp and the stack trace are different on each run.  The
	// example removes these two fields and prints the rest of each line.
	for line := range strings.Lines(buf.String()) {
		var fields map[string]any

		_ = json.Unmarshal([]byte(line), &fields)

		delete(fields, "timestamp")
		delete(fields, "stack_trace")

		b, _ := json.Marshal(fields)

		fmt.Println(string(b))
	}

	// Output:
	// {"message":"Payment started","severity":"INFO"}
	// {"@type":"type.googleapis.com/google.devtools.clouderrorreporting.v1beta1.ReportedErrorEvent","message":"Payment failed","serviceContext":{"service":"checkout","version":"v1.2.0"},"severity":"ERROR"}
}
