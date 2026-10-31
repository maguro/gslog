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

package gslog_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"m4o.io/gslog"
	"m4o.io/gslog/stdout"
)

// The options and the labels of this package apply to both handlers.  The
// example sets the default level of a stdout handler and adds a label from
// the context to a record.  The handler writes the record as one line of
// JSON.  A fixed time keeps the timestamp of the output the same on each run.
func Example() {
	h := stdout.NewHandler(os.Stdout, gslog.WithDefaultLeveler(slog.LevelDebug))

	ctx := context.Background()
	ctx = gslog.WithLabels(ctx, gslog.Label("tenant", "acme"))

	at := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	r := slog.NewRecord(at, slog.LevelDebug, "How now brown cow?", 0)
	r.AddAttrs(slog.String("animal", "cow"))

	_ = h.Handle(ctx, r)

	// Output: {"severity":"DEBUG","message":"How now brown cow?","timestamp":"2024-01-02T03:04:05Z","logging.googleapis.com/labels":{"tenant":"acme"},"animal":"cow"}
}

// newRecord returns an Info record with a fixed time and the message "How
// now brown cow?".  A fixed time keeps the timestamp of the example output
// the same on each run.
func newRecord(attrs ...slog.Attr) slog.Record {
	at := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

	r := slog.NewRecord(at, slog.LevelInfo, "How now brown cow?", 0)
	r.AddAttrs(attrs...)

	return r
}

// A program can change its log level while it runs, for example from an
// admin endpoint.  WithLeveler takes a slog.Leveler.  With a *slog.LevelVar,
// the handler reads the variable for each record.
func ExampleWithLeveler() {
	var level slog.LevelVar
	level.Set(slog.LevelInfo)

	h := stdout.NewHandler(os.Stdout, gslog.WithLeveler(&level))

	ctx := context.Background()

	fmt.Println(h.Enabled(ctx, slog.LevelDebug))

	level.Set(slog.LevelDebug)

	fmt.Println(h.Enabled(ctx, slog.LevelDebug))

	// Output:
	// false
	// true
}

// A deployment sets the log level in an environment variable.  The program
// needs no change.  The value can be a level of Google Cloud Logging, such
// as "notice".  The case of the value is not important.
func ExampleWithLogLevelFromEnvVar() {
	const envVar = "FOO_LOG_LEVEL"
	_ = os.Setenv(envVar, "notice")
	defer func() {
		_ = os.Unsetenv(envVar)
	}()

	h := stdout.NewHandler(os.Stdout, gslog.WithLogLevelFromEnvVar(envVar))

	ctx := context.Background()

	fmt.Println(h.Enabled(ctx, slog.LevelInfo), h.Enabled(ctx, gslog.LevelNotice))

	// Output: false true
}

// The program sets the default level for a deployment that does not set the
// environment variable.  A deployment that sets the variable overrides the
// default.
func ExampleWithDefaultLeveler() {
	const envVar = "FOO_LOG_LEVEL"
	_ = os.Unsetenv(envVar)

	h := stdout.NewHandler(os.Stdout,
		gslog.WithLogLevelFromEnvVar(envVar),
		gslog.WithDefaultLeveler(slog.LevelWarn),
	)

	ctx := context.Background()

	fmt.Println(h.Enabled(ctx, slog.LevelInfo), h.Enabled(ctx, slog.LevelWarn))

	// Output: false true
}

// The Logs Explorer shows the file and the line of the log call when the
// entry has a source location.  The stdout handler writes the location in
// the field "logging.googleapis.com/sourceLocation".
func ExampleWithSourceAdded() {
	var buf bytes.Buffer

	h := stdout.NewHandler(&buf, gslog.WithSourceAdded())
	logger := slog.New(h)

	logger.Info("How now brown cow?")

	// The timestamp is different on each run.  The line changes with each
	// edit of this file.  The example prints the name of the file and the
	// function.
	var line map[string]any

	_ = json.Unmarshal(buf.Bytes(), &line)

	loc, _ := line["logging.googleapis.com/sourceLocation"].(map[string]any)
	file, _ := loc["file"].(string)
	base := filepath.Base(file)

	fmt.Println(base, loc["function"])

	// Output: example_test.go m4o.io/gslog_test.ExampleWithSourceAdded
}

// redactPassword is a gslog.AttrMapper that replaces the value of each
// attribute with the key "password".
func redactPassword(_ []string, a slog.Attr) slog.Attr {
	if a.Key == "password" {
		return slog.String(a.Key, "REDACTED")
	}

	return a
}

// A mapper keeps a secret out of the log.  The handler calls the mapper for
// each attribute, also for an attribute inside a group.  The first argument
// of the mapper holds the groups of the attribute, from the outer group to
// the inner group.
func ExampleWithReplaceAttr() {
	h := stdout.NewHandler(os.Stdout, gslog.WithReplaceAttr(redactPassword))

	auth := slog.Group("auth", slog.String("user", "jan"), slog.String("password", "hunter2"))
	r := newRecord(auth)

	_ = h.Handle(context.Background(), r)

	// Output: {"severity":"INFO","message":"How now brown cow?","timestamp":"2024-01-02T03:04:05Z","auth":{"user":"jan","password":"REDACTED"}}
}

// One call adds the labels of a tenant to the context.  A later call adds a
// label for one operation.  The entries of that operation have the labels of
// both calls.  The handler writes the labels in key order.
func ExampleWithLabels() {
	h := stdout.NewHandler(os.Stdout)

	ctx := context.Background()
	ctx = gslog.WithLabels(ctx, gslog.Label("tenant", "acme"), gslog.Label("region", "eu"))
	ctx = gslog.WithLabels(ctx, gslog.Label("operation", "checkout"))

	_ = h.Handle(ctx, newRecord())

	// Output: {"severity":"INFO","message":"How now brown cow?","timestamp":"2024-01-02T03:04:05Z","logging.googleapis.com/labels":{"operation":"checkout","region":"eu","tenant":"acme"}}
}
