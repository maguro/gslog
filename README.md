# gslog

![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.21-%23007d9c)
[![Documentation](https://godoc.org/github.com/maguro/gslog?status.svg)](http://godoc.org/github.com/maguro/gslog)
[![Go Report Card](https://goreportcard.com/badge/github.com/maguro/gslog)](https://goreportcard.com/report/github.com/maguro/gslog)
[![codecov](https://codecov.io/gh/maguro/gslog/graph/badge.svg?token=3FAJJ2SIZB)](https://codecov.io/gh/maguro/gslog)
[![License](https://img.shields.io/github/license/maguro/gslog)](./LICENSE)

A Google Cloud Logging [Handler](https://pkg.go.dev/log/slog#Handler) implementation
for [slog](https://go.dev/blog/slog).

---

This Google Cloud Logging (GCL) [slog.Handler](https://pkg.go.dev/log/slog#Handler)
implementation directly fills the GCL entry, `logging.Entry`, with information
obtained in the `context.Context`, `slog.Record`, and implied context within
the Handler itself. The `logging.Entry.Payload` is filled with a
[Protobuf `structpb.Struct`](https://pkg.go.dev/google.golang.org/protobuf/types/known/structpb#Struct)
instance, resulting in a `jsonPayload` with the log message having the key
"message". Log records are [sent asynchronously](https://pkg.go.dev/cloud.google.com/go/logging#Logger.Log).
Critical level, or higher, log records will
be [sent synchronously](https://pkg.go.dev/cloud.google.com/go/logging#Logger.LogSync).

The GCL Handler's options include a number of ways to include information from
"outside" frameworks:

- Labels attached to the context, via `gslog.WithLabels(ctx, ...labels)`, which
  are added to the GCL entry, `logging.Entry`, `Labels` field.
- [OpenTelemetry baggage](https://opentelemetry.io/docs/concepts/signals/baggage/) attached to the context which are
  added as attributes,
  `slog.Attr`, to the logging record, `slog.Record`. The baggage keys are prefixed
  with "otel-baggage/" to mitigate collision with other log attributes.
- [OpenTelemetry tracing](https://opentelemetry.io/docs/concepts/signals/traces/) attached to the context which are
  added directly to
  the GCL entry, `logging.Entry`, tracing fields.
- Labels from the [Kubernetes Downward API](https://kubernetes.io/docs/concepts/workloads/pods/downward-api/)
  podinfo `labels` file, which are added to the GCL entry, `logging.Entry`,
  `Labels` field. The labels are prefixed with "k8s-pod/" to adhere to the
  GCL conventions for Kubernetes Pod labels.

## Install

```sh
go get m4o.io/gslog
```

**Compatibility**: go >= 1.21

## Example Usage

First create a [Google Cloud Logging](https://pkg.go.dev/cloud.google.com/go/logging)
`logging.Client` to use throughout your application:

```go
ctx := context.Background()
client, err := logging.NewClient(ctx, "my-project")
if err != nil {
	// TODO: Handle error.
}
```

Usually, you'll want to add log entries to a buffer to be periodically flushed
(automatically and asynchronously) to the Cloud Logging service. Use the
logger when creating the new `gslog.GcpHandler` which is passed to `slog.New()`
to obtain a `slog`-based logger.

```go
loggger := client.Logger("my-log")

h := gslog.NewGcpHandler(loggger)
l := slog.New(h)

l.Info("How now brown cow?")
```

Writing critical, or higher, log level entries will be sent synchronously.

```go
l.Log(context.Background(), gslog.LevelCritical, "Danger, Will Robinson!")
```

Close your client before your program exits, to flush any buffered log entries.

```go
err = client.Close()
if err != nil {
   // TODO: Handle error.
}
```

## Logger Configuration Options

Creating a Google Cloud Logging [Handler](https://pkg.go.dev/log/slog#Handler)
using `gslog.NewGcpHandler(logger, ...options)` accepts the
following options:

| Configuration option                   |   Arguments    | Description                                                                                                                                                                                                                                                                                                                    |
|----------------------------------------|:--------------:|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `gslog.WithLogLeveler(leveler)`        | `slog.Leveler` | Specifies the `slog.Leveler` for logging. Explicitly setting the log level here takes precedence over the other options.                                                                                                                                                                                                       |
| `gslog.WithLogLevelFromEnvVar(envVar)` |    `string`    | Specifies the log level for logging comes from tne environmental variable specified by the key.                                                                                                                                                                                                                                |
| `gslog.WithDefaultLogLeveler()`        | `slog.Leveler` | Specifies the default `slog.Leveler` for logging.                                                                                                                                                                                                                                                                              |
| `gslog.WithSourceAdded()`              |                | Causes the handler to compute the source code position of the log statement and add a `slog.SourceKey` attribute to the output.                                                                                                                                                                                                |
| `gslog.WithLabels()`                   |                | Adds any labels found in the context to the `logging.Entry`'s `Labels` field.                                                                                                                                                                                                                                                  |
| `gslog.WithReplaceAttr(mapper)`        | `gslog.Mapper` | Specifies an attribute mapper used to rewrite each non-group attribute before it is logged.                                                                                                                                                                                                                                    |
| `otel.WithOtelBaggage()`               |                | Directs that the `slog.Handler` to include [OpenTelemetry baggage](https://opentelemetry.io/docs/concepts/signals/baggage/).  The `baggage.Baggage` is obtained from the context, if available, and added as attributes.                                                                                                       |
| `otel.WithOtelTracing()`               |                | Directs that the `slog.Handler` to include [OpenTelemetry tracing](https://opentelemetry.io/docs/concepts/signals/traces/).  Tracing information is obtained from the `trace.SpanContext` stored in the context, if provided.                                                                                                  |
| `k8s.WithPodinfoLabels(root)`          |    `string`    | Directs that the `slog.Handler` to include labels from the [Kubernetes Downward API](https://kubernetes.io/docs/concepts/workloads/pods/downward-api/) podinfo `labels` file. The labels file is expected to be found in the directory specified by root and MUST be named "labels", per the Kubernetes Downward API for Pods. |

## Design Notes

There's a number of different ways to map the `slog.Record` to a GCL entry,
`logging.Entry`.

- a JSON string
- a value that can be marshaled to a JSON object, like a `map[string]interface{}` or a `struct`
- a Protobuf `*anypb.Any`

The pros and cons are

| Payload type                                 | pros                                              | cons                                                    |
|----------------------------------------------|---------------------------------------------------|---------------------------------------------------------|
| JSON string                                  | fast and efficient to generate on the `slog` side | logged as a flat unstructured `textPayload` in GCL      |
| value that can be marshaled to a JSON object | logged as a structured `jsonPayload` in GCL       | the marshalling effort is complicated and not amortized |
| Protobuf `*anypb.Any`                        | not known at the moment                           | not known at the moment                                 |

Even though a JSON string can be marshaled to a JSON object, the GCL client 
merely looks at its Go type and decides to treat it as a flat text message.

If not a string, values that can be marshaled to a JSON object are actually
first marshalled into a JSON object, i.e. `map[string]interface{}`, and then 
that resulting JSON object is translated into an equivalent `structpb.Struct`
Protobuf message.  The GCL logger will redo this marshalling and translating
for every message logged.

The reason why the `logging.Entry` `Payload` field is set with a Protobuf
`structpb.Struct` when the end result is a `jsonPayload` GCL logging entry is
because that's what `Logger.Log(e)` does anyway, behind the scenes. When either
