# gslog

![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.26-%23007d9c)
[![Documentation](https://pkg.go.dev/badge/m4o.io/gslog.svg)](https://pkg.go.dev/m4o.io/gslog)
[![Go Report Card](https://goreportcard.com/badge/m4o.io/gslog)](https://goreportcard.com/report/m4o.io/gslog)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/maguro/gslog/badge)](https://scorecard.dev/viewer/?uri=github.com/maguro/gslog)
[![codecov](https://codecov.io/gh/maguro/gslog/graph/badge.svg?token=3FAJJ2SIZB)](https://codecov.io/gh/maguro/gslog)
[![License](https://img.shields.io/github/license/maguro/gslog)](./LICENSE)

Structured logging to Google Cloud Logging from Go's `log/slog`. gslog
writes each `slog` record as a Cloud Logging entry with the correct severity,
a `jsonPayload`, and the fields that link the entry to its trace in Cloud
Trace. It also adds OpenTelemetry baggage, Kubernetes pod labels, and labels
from the context.

## Install

```sh
go get m4o.io/gslog
```

**Compatibility**: go >= 1.26

gslog uses the `log/slog` API that Go 1.21 introduced. The dependencies of
gslog set the minimum Go version to 1.26. The language features that gslog
uses do not set this minimum.

## Quick Start

On Cloud Run, Cloud Functions, GKE, and GCE with the Ops Agent, a logging
agent reads stdout. Write to stdout with the `stdout` handler:

```go
package main

import (
	"log/slog"
	"os"

	"m4o.io/gslog/stdout"
)

func main() {
	h := stdout.NewHandler(os.Stdout)
	slog.SetDefault(slog.New(h))

	slog.Info("How now brown cow?", "animal", "cow")
}
```

```json
{"severity":"INFO","message":"How now brown cow?","timestamp":"2026-09-19T00:53:53.833535Z","animal":"cow"}
```

The agent makes an INFO entry with `animal` in its `jsonPayload`.

### Link Logs to Traces

Add `otel.WithOtelTracing` and log with a context that holds an OpenTelemetry
span:

```go
import (
	"log/slog"
	"os"

	"m4o.io/gslog/otel"
	"m4o.io/gslog/stdout"
)

h := stdout.NewHandler(os.Stdout, otel.WithOtelTracing("my-project"))
logger := slog.New(h)

logger.InfoContext(ctx, "Order placed", "order_id", "A-1001")
```

```json
{"severity":"INFO","message":"Order placed","timestamp":"2026-09-19T00:53:54.173948Z","logging.googleapis.com/trace":"projects/my-project/traces/52fc1643a9381fc674742bb0067101e7","logging.googleapis.com/spanId":"d3e9e8c51cb190df","logging.googleapis.com/trace_sampled":true,"order_id":"A-1001"}
```

Cloud Logging shows the entry under its trace in Cloud Trace, and the Logs
Explorer groups the entries of one request.

To get the project ID from the metadata server, as the
`cloud.google.com/go/logging` client does, use the option of the
`m4o.io/gslog/otel/detect` package:

```go
import (
	"os"

	"m4o.io/gslog/otel/detect"
	"m4o.io/gslog/stdout"
)

h := stdout.NewHandler(os.Stdout, detect.WithOtelTracing())
```

The handler gets the span only from the context of the log call. Use a call
with a context, such as `logger.InfoContext(ctx, ...)`. The record of a call
with no context, such as `logger.Info(...)`, has no trace.

A program that uses the OpenTelemetry SDK with `otelhttp` already has the span
in the context of each request. A program with no SDK can read the W3C
`traceparent` header with a small wrapper:

```go
import (
	"net/http"

	"go.opentelemetry.io/otel/propagation"
)

func withTraceContext(next http.Handler) http.Handler {
	propagator := propagation.TraceContext{}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		carrier := propagation.HeaderCarrier(r.Header)
		ctx := propagator.Extract(r.Context(), carrier)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}
```

The [`otel.WithOtelTracing` (httpServer)](docs/examples.md#otelwithoteltracing-httpserver)
example runs this wrapper.

### Which Handler

| Handler  | Use it when                                                            |
|----------|------------------------------------------------------------------------|
| `stdout` | A logging agent reads stdout.                                          |
| `gcp`    | No agent reads stdout, or you want to use the Cloud Logging API client. |

## Packages

gslog has two handlers. Both handlers make the same Google Cloud Logging
(GCL) entry. The packages are:

- `gcp` holds a handler that sends each entry with the
  [Cloud Logging API client](https://pkg.go.dev/cloud.google.com/go/logging).
- `stdout` holds a handler that writes each entry as one line of JSON. A
  logging agent reads the line and makes the entry.
- `core` holds the options, the labels, and the levels that both handlers
  use.
- `otel` and `k8s` hold options that read information from other frameworks.

The `gcp` handler fills the GCL entry, `logging.Entry`, directly. The handler
gets the information from the `context.Context`, the `slog.Record`, and the
state of the handler itself. The handler sets `logging.Entry.Payload` to a
[Protobuf `structpb.Struct`](https://pkg.go.dev/google.golang.org/protobuf/types/known/structpb#Struct)
instance. The result is a `jsonPayload` in which the log message has the key
"message". The handler [sends log records asynchronously](https://pkg.go.dev/cloud.google.com/go/logging#Logger.Log).
The handler [sends log records at Critical level or higher synchronously](https://pkg.go.dev/cloud.google.com/go/logging#Logger.LogSync).

The options of both handlers include several ways to include information
from other frameworks:

- Labels in the context, set with `core.WithLabels(ctx, ...labels)`. The
  handlers add them to the labels of the GCL entry. The maximum number of
  labels is 64.
- [OpenTelemetry baggage](https://opentelemetry.io/docs/concepts/signals/baggage/) in the context. The handlers add
  the baggage as attributes, `slog.Attr`, to the logging record, `slog.Record`.
  The handlers add the prefix "otel-baggage/" to each baggage key. The prefix
  makes collisions with other log attributes less likely.
- [OpenTelemetry tracing](https://opentelemetry.io/docs/concepts/signals/traces/) in the context. The handlers add
  the tracing information directly to the tracing fields of the GCL entry.
- Labels from the [Kubernetes Downward API](https://kubernetes.io/docs/concepts/workloads/pods/downward-api/)
  podinfo `labels` file. The handlers add them to the labels of the GCL entry.
  The handlers add the prefix "k8s-pod/" to each label. This follows the GCL
  conventions for Kubernetes Pod labels.

## Using the gcp Handler

Create one [Google Cloud Logging](https://pkg.go.dev/cloud.google.com/go/logging)
`logging.Client`, and use this client throughout your application. Create a
`gcp.Handler` with a `logging.Logger` from the client. Pass the handler to
`slog.New()` to get a `slog` logger.

The logger adds log entries to a buffer. The logger flushes the buffer to the
Cloud Logging service periodically, automatically, and asynchronously. The
handler sends entries at Critical level or higher synchronously. Close the
client before the program exits. This flushes the buffered log entries.

```go
package main

import (
	"context"
	"log"
	"log/slog"

	"cloud.google.com/go/logging"

	"m4o.io/gslog/core"
	"m4o.io/gslog/gcp"
)

func main() {
	ctx := context.Background()

	client, err := logging.NewClient(ctx, "my-project")
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		cerr := client.Close()
		if cerr != nil {
			log.Print(cerr)
		}
	}()

	lg := client.Logger("my-log")
	h := gcp.NewHandler(lg)
	logger := slog.New(h)

	logger.Info("How now brown cow?")
	logger.Log(ctx, core.LevelCritical, "Danger, Will Robinson!")
}
```

### Log an HTTP Request

A Cloud Logging entry has a typed `httpRequest` field. The Logs Explorer
shows this field in the summary line of the entry: the method, the status, the
size, and the latency of the request.

Put a `*logging.HTTPRequest` in the context of the log call with
`gcp.WithHTTPRequest`. The handler sets the field from that context:

```go
import (
	"cloud.google.com/go/logging"

	"m4o.io/gslog/gcp"
)

request := &logging.HTTPRequest{Request: r, Status: status, ResponseSize: size, Latency: latency}
ctx := gcp.WithHTTPRequest(r.Context(), request)

logger.InfoContext(ctx, "Request completed")
```

Each record that a log call writes with that context has the field. The
[`gcp.WithHTTPRequest`](docs/examples.md#gcpwithhttprequest) example shows an
HTTP handler that logs one record for each request.

With the `stdout` handler, write a group attribute that has the key
`httpRequest` and the member names of the
[HttpRequest](https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest)
type. The logging agent moves that group to the field.

## Structured Logging to Stdout

On Cloud Run, Cloud Functions, GKE, and GCE with the Ops Agent, a logging
agent reads stdout. `stdout.NewHandler` writes each entry as one line of
JSON in the [structured logging format](https://cloud.google.com/logging/docs/structured-logging)
that the agent reads. The agent makes the same GCL entry that the API client
makes, with the same severity, labels, trace fields, and `jsonPayload`.

This handler does not use a `logging.Client`. The `stdout` package and the
`core` package, which holds the shared options, labels, and levels, import no
module outside the standard library. The handler writes each entry before the
log call returns, so no entry waits in a buffer when the instance stops. All
options in the table below work with both handlers.

The root `gslog` package is deprecated. It exports the names of release
v0.23.0 and imports the API client.

## Logger Configuration Options

`gcp.NewHandler(logger, ...options)` and `stdout.NewHandler(w, ...options)`
create a [Handler](https://pkg.go.dev/log/slog#Handler). They accept these
options from the `core` package:

| Configuration option                   |     Arguments      | Description                                                                                                                                                                                                                                                                                                                    |
|----------------------------------------|:------------------:|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `core.WithLogLeveler(leveler)`         |   `slog.Leveler`   | Specifies the `slog.Leveler` for logging. This option has precedence over the other log level options.                                                                                                                                                                                                                         |
| `core.WithLogLevelFromEnvVar(envVar)`  |      `string`      | Reads the log level from the environment variable that the key names.                                                                                                                                                                                                                                                          |
| `core.WithDefaultLogLeveler()`         |   `slog.Leveler`   | Specifies the default `slog.Leveler` for logging.                                                                                                                                                                                                                                                                              |
| `core.WithSourceAdded()`               |                    | Causes the handler to compute the source code position of the log statement. The gcp handler sets the `SourceLocation` field of the `logging.Entry`. The stdout handler writes the `logging.googleapis.com/sourceLocation` field.                                                                                             |
| `core.WithReplaceAttr(mapper)`         | `core.AttrMapper`  | Specifies an attribute mapper. The handler calls the mapper to rewrite each non-group attribute before the handler logs the attribute.                                                                                                                                                                                         |
| `otel.WithOtelBaggage()`               |                    | Causes the handler to include [OpenTelemetry baggage](https://opentelemetry.io/docs/concepts/signals/baggage/). The handler gets the `baggage.Baggage` from the context, if the context has one, and adds the baggage as attributes.                                                                                           |
| `otel.WithOtelTracing(id)`             |      `string`      | Causes the handler to include [OpenTelemetry tracing](https://opentelemetry.io/docs/concepts/signals/traces/). The handler gets the tracing information from the `trace.SpanContext` in the context, if the context has one. `id` is the ID of the project that holds the traces.                                                                                                   |
| `detect.WithOtelTracing()`             |                    | The same as `otel.WithOtelTracing(id)`, with the project ID from the metadata server and then from `GOOGLE_CLOUD_PROJECT`. If it finds no project ID, the handler includes no tracing. The `detect` package is `m4o.io/gslog/otel/detect`. |
| `k8s.WithPodinfoLabels(root)`          |      `string`      | Causes the handler to include labels from the [Kubernetes Downward API](https://kubernetes.io/docs/concepts/workloads/pods/downward-api/) podinfo `labels` file. The handler expects the labels file in the directory that root specifies. The file must be named "labels", as the Kubernetes Downward API for Pods specifies. |
| `errorreporting.WithService(service, version)` | `string`, `string` | Causes the handler to include the fields that [Google Cloud Error Reporting](https://cloud.google.com/error-reporting/docs/formatting-error-messages) reads in each record at level Error or higher: the `@type` of a `ReportedErrorEvent`, the `serviceContext`, and a `stack_trace`. The stack trace is the stack of the log call. |

The [examples](docs/examples.md) show the options of the `core`, `otel`,
`k8s`, and `errorreporting` packages in complete programs, with their output.

## Design Notes

There are several ways to map the `slog.Record` to a GCL entry,
`logging.Entry`.

- a JSON string
- a value that can be marshaled to a JSON object, like a `map[string]any` or a `struct`
- a `json.RawMessage`
- a Protobuf `*structpb.Struct`
- a Protobuf `*anypb.Any`

The pros and cons are

| Payload type                                 | pros                                                           | cons                                                                          |
|----------------------------------------------|----------------------------------------------------------------|-------------------------------------------------------------------------------|
| JSON string                                  | fast and efficient to generate on the `slog` side              | GCL logs it as a flat, unstructured `textPayload`                             |
| value that can be marshaled to a JSON object | GCL logs it as a structured `jsonPayload`                      | the marshalling effort is complicated and not amortized                       |
| `json.RawMessage`                            | GCL logs it as a structured `jsonPayload` with no marshal step | the GCL logger unmarshals it and builds a `structpb.Struct` for every message |
| Protobuf `*structpb.Struct`                  | GCL logs it as a structured `jsonPayload` with no conversion   | the `slog` side must build the `structpb.Struct`                              |
| Protobuf `*anypb.Any`                        | GCL logs it as a typed `protoPayload` with no conversion       | the message type must be registered, and `RedirectAsJSON` rejects it          |

A JSON string can be marshaled to a JSON object. But the GCL client only
examines the Go type of the value and treats the string as a flat text message.

For a value that is not a string and that can be marshaled to a JSON object,
the GCL logger first marshals the value to a JSON object,
`map[string]any`. The GCL logger then translates that JSON object to
an equivalent `structpb.Struct` Protobuf message. The GCL logger does this
marshalling and translation again for every message that it logs.

The handler sets the `Payload` field of `logging.Entry` to a Protobuf
`structpb.Struct`. `Logger.Log(e)` uses a `structpb.Struct` as is and does no
conversion. A `structpb.Struct` is the only `jsonPayload` type that the GCL
logger accepts without work on every message.
