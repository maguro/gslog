# gslog

![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.26-%23007d9c)
[![Documentation](https://pkg.go.dev/badge/m4o.io/gslog.svg)](https://pkg.go.dev/m4o.io/gslog)
[![Go Report Card](https://goreportcard.com/badge/m4o.io/gslog)](https://goreportcard.com/report/m4o.io/gslog)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/maguro/gslog/badge)](https://scorecard.dev/viewer/?uri=github.com/maguro/gslog)
[![codecov](https://codecov.io/gh/maguro/gslog/graph/badge.svg?token=3FAJJ2SIZB)](https://codecov.io/gh/maguro/gslog)
[![License](https://img.shields.io/github/license/maguro/gslog)](./LICENSE)

A Google Cloud Logging [Handler](https://pkg.go.dev/log/slog#Handler) implementation
for [slog](https://go.dev/blog/slog).

---

This Google Cloud Logging (GCL) [slog.Handler](https://pkg.go.dev/log/slog#Handler)
implementation fills the GCL entry, `logging.Entry`, directly. The handler gets
the information from the `context.Context`, the `slog.Record`, and the state of
the handler itself. The handler sets `logging.Entry.Payload` to a
[Protobuf `structpb.Struct`](https://pkg.go.dev/google.golang.org/protobuf/types/known/structpb#Struct)
instance. The result is a `jsonPayload` in which the log message has the key
"message". The handler [sends log records asynchronously](https://pkg.go.dev/cloud.google.com/go/logging#Logger.Log).
The handler [sends log records at Critical level or higher synchronously](https://pkg.go.dev/cloud.google.com/go/logging#Logger.LogSync).

The options of the GCL handler include several ways to include information
from other frameworks:

- Labels in the context, set with `gslog.WithLabels(ctx, ...labels)`. The
  handler adds them to the `Labels` field of the GCL entry, `logging.Entry`.
  The maximum number of labels is 64.
- [OpenTelemetry baggage](https://opentelemetry.io/docs/concepts/signals/baggage/) in the context. The handler adds
  the baggage as attributes, `slog.Attr`, to the logging record, `slog.Record`.
  The handler adds the prefix "otel-baggage/" to each baggage key. The prefix
  makes collisions with other log attributes less likely.
- [OpenTelemetry tracing](https://opentelemetry.io/docs/concepts/signals/traces/) in the context. The handler adds
  the tracing information directly to the tracing fields of the GCL entry,
  `logging.Entry`.
- Labels from the [Kubernetes Downward API](https://kubernetes.io/docs/concepts/workloads/pods/downward-api/)
  podinfo `labels` file. The handler adds them to the `Labels` field of the GCL
  entry, `logging.Entry`. The handler adds the prefix "k8s-pod/" to each label.
  This follows the GCL conventions for Kubernetes Pod labels.

## Install

```sh
go get m4o.io/gslog
```

**Compatibility**: go >= 1.26

gslog uses the `log/slog` API that Go 1.21 introduced. The dependencies of
gslog set the minimum Go version to 1.26. The language features that gslog
uses do not set this minimum.

## Example Usage

First, create a [Google Cloud Logging](https://pkg.go.dev/cloud.google.com/go/logging)
`logging.Client`. Use this client throughout your application:

```go
ctx := context.Background()
client, err := logging.NewClient(ctx, "my-project")
if err != nil {
	// TODO: Handle error.
}
```

Usually, you want to add log entries to a buffer. The buffer is flushed to the
Cloud Logging service periodically, automatically, and asynchronously. Create a
`gslog.GcpHandler` with the logger. Pass the handler to `slog.New()` to get a
`slog` logger.

```go
loggger := client.Logger("my-log")

h := gslog.NewGcpHandler(loggger)
l := slog.New(h)

l.Info("How now brown cow?")
```

The handler sends entries at Critical level or higher synchronously.

```go
l.Log(context.Background(), gslog.LevelCritical, "Danger, Will Robinson!")
```

Close the client before the program exits. This flushes the buffered log
entries.

```go
err = client.Close()
if err != nil {
   // TODO: Handle error.
}
```

## Structured Logging to Stdout

On Cloud Run, Cloud Functions, GKE, and GCE with the Ops Agent, a logging
agent reads stdout. `gslog.NewStdoutHandler` writes each entry as one line of
JSON in the [structured logging format](https://cloud.google.com/logging/docs/structured-logging)
that the agent reads. The agent makes the same GCL entry that the API client
makes, with the same severity, labels, trace fields, and `jsonPayload`.

```go
h := gslog.NewStdoutHandler(os.Stdout)
l := slog.New(h)

l.Info("How now brown cow?")
```

This handler does not use a `logging.Client`. Each entry is written before
the log call returns, so no entry waits in a buffer when the instance stops.
All options in the table below work with both handlers.

## Logger Configuration Options

`gslog.NewGcpHandler(logger, ...options)` creates a Google Cloud Logging
[Handler](https://pkg.go.dev/log/slog#Handler). It accepts these options:

| Configuration option                   |     Arguments      | Description                                                                                                                                                                                                                                                                                                                    |
|----------------------------------------|:------------------:|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `gslog.WithLogLeveler(leveler)`        |   `slog.Leveler`   | Specifies the `slog.Leveler` for logging. This option has precedence over the other log level options.                                                                                                                                                                                                                         |
| `gslog.WithLogLevelFromEnvVar(envVar)` |      `string`      | Reads the log level from the environment variable that the key names.                                                                                                                                                                                                                                                          |
| `gslog.WithDefaultLogLeveler()`        |   `slog.Leveler`   | Specifies the default `slog.Leveler` for logging.                                                                                                                                                                                                                                                                              |
| `gslog.WithSourceAdded()`              |                    | Causes the handler to compute the source code position of the log statement. The handler sets the position in the `SourceLocation` field of the `logging.Entry`.                                                                                                                                                               |
| `gslog.WithReplaceAttr(mapper)`        | `gslog.AttrMapper` | Specifies an attribute mapper. The handler calls the mapper to rewrite each non-group attribute before the handler logs the attribute.                                                                                                                                                                                         |
| `otel.WithOtelBaggage()`               |                    | Causes the handler to include [OpenTelemetry baggage](https://opentelemetry.io/docs/concepts/signals/baggage/). The handler gets the `baggage.Baggage` from the context, if the context has one, and adds the baggage as attributes.                                                                                           |
| `otel.WithOtelTracing()`               |                    | Causes the handler to include [OpenTelemetry tracing](https://opentelemetry.io/docs/concepts/signals/traces/). The handler gets the tracing information from the `trace.SpanContext` in the context, if the context has one.                                                                                                   |
| `k8s.WithPodinfoLabels(root)`          |      `string`      | Causes the handler to include labels from the [Kubernetes Downward API](https://kubernetes.io/docs/concepts/workloads/pods/downward-api/) podinfo `labels` file. The handler expects the labels file in the directory that root specifies. The file must be named "labels", as the Kubernetes Downward API for Pods specifies. |

## Design Notes

There are several ways to map the `slog.Record` to a GCL entry,
`logging.Entry`.

- a JSON string
- a value that can be marshaled to a JSON object, like a `map[string]interface{}` or a `struct`
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
`map[string]interface{}`. The GCL logger then translates that JSON object to
an equivalent `structpb.Struct` Protobuf message. The GCL logger does this
marshalling and translation again for every message that it logs.

The handler sets the `Payload` field of `logging.Entry` to a Protobuf
`structpb.Struct`. `Logger.Log(e)` uses a `structpb.Struct` as is and does no
conversion. A `structpb.Struct` is the only `jsonPayload` type that the GCL
logger accepts without work on every message.
