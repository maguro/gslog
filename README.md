# gslog

![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.21-%23007d9c)
[![Documentation](https://godoc.org/github.com/maguro/gslog?status.svg)](http://godoc.org/github.com/maguro/gslog)
[![Go Report Card](https://goreportcard.com/badge/github.com/maguro/gslog)](https://goreportcard.com/report/github.com/maguro/gslog)
[![codecov](https://codecov.io/gh/maguro/gslog/graph/badge.svg?token=3FAJJ2SIZB)](https://codecov.io/gh/maguro/gslog)
[![License](https://img.shields.io/github/license/maguro/gslog)](./LICENSE)

A Google Cloud Logging [Handler](https://pkg.go.dev/log/slog#Handler) implementation
for [slog](https://go.dev/blog/slog).

---

Critical level log records will be sent synchronously.

## Install

```sh
go get m4o.io/gslog
```

**Compatibility**: go >= 1.21

## Example Usage

```go
package main

import (
	"context"
	"log/slog"

	"cloud.google.com/go/logging"

	"m4o.io/gslog"
)

func main() {
	ctx := context.Background()
	client, err := logging.NewClient(ctx, "my-project")
	if err != nil {
		// TODO: Handle error.
	}

	lg := client.Logger("my-log")

	h := gslog.NewGcpHandler(lg)
	l := slog.New(h)

	l.Info("How now brown cow?")
}
```

### Logger configuration options

Creating a Google Cloud Logging [Handler](https://pkg.go.dev/log/slog#Handler) using `gslog.NewGcpHandler(logger, ...options)` accepts the
following options:

| Configuration option                   |     Arguments      | Description                                                                                                                                                                                                                                                                                                                    |
|----------------------------------------|:------------------:|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `gslog.WithLogLeveler(leveler)`        |   `slog.Leveler`   | Specifies the `slog.Leveler` for logging. Explicitly setting the log level here takes precedence over the other options.                                                                                                                                                                                                       |
| `gslog.WithLogLevelFromEnvVar(envVar)` |      `string`      | Specifies the log level for logging comes from tne environmental variable specified by the key.                                                                                                                                                                                                                                |
| `gslog.WithDefaultLogLeveler()`        |   `slog.Leveler`   | Specifies the default `slog.Leveler` for logging.                                                                                                                                                                                                                                                                              |
| `gslog.WithSourceAdded()`              |                    | Causes the handler to compute the source code position of the log statement and add a `slog.SourceKey` attribute to the output.                                                                                                                                                                                                |
| `gslog.WithLabels()`                   |                    | Adds any labels found in the context to the `logging.Entry`'s `Labels` field.                                                                                                                                                                                                                                                  |
| `gslog.WithReplaceAttr(mapper)`        | `gslog.AttrMapper` | Specifies an attribute mapper used to rewrite each non-group attribute before it is logged.                                                                                                                                                                                                                                    |
| `otel.WithOtelBaggage()`               |                    | Directs that the `slog.Handler` to include [OpenTelemetry baggage](https://opentelemetry.io/docs/concepts/signals/baggage/).  The `baggage.Baggage` is obtained from the context, if available, and added as attributes.                                                                                                       |
| `otel.WithOtelTracing()`               |                    | Directs that the `slog.Handler` to include [OpenTelemetry tracing](https://opentelemetry.io/docs/concepts/signals/traces/).  Tracing information is obtained from the `trace.SpanContext` stored in the context, if provided.                                                                                                  |
| `k8s.WithPodinfoLabels(root)`          |      `string`      | Directs that the `slog.Handler` to include labels from the [Kubernetes Downward API](https://kubernetes.io/docs/concepts/workloads/pods/downward-api/) podinfo `labels` file. The labels file is expected to be found in the directory specified by root and MUST be named "labels", per the Kubernetes Downward API for Pods. |
