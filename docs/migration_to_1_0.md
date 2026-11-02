# Migration to v1.0.0

This guide covers a program that uses v0.25.x or earlier. 

Two kinds of change need your attention. A renamed or removed declaration
stops the build and tells you where. A change to the log output does not
stop the build. Read [Changes to the log output](#changes-to-the-log-output)
even when the build passes.

## The core package is gone

v0.25.0 held the shared options, labels, and levels in `m4o.io/gslog/core`.
The root package `m4o.io/gslog` held an alias for each one. v1.0.0 declares
them in the root package and removes `core`.

Replace an import of `m4o.io/gslog/core` with `m4o.io/gslog`. Replace each
`core.` qualifier with `gslog.`. The table below gives the names that also
changed.

## Renamed declarations

| v0.25.0 | v1.0.0 |
|---|---|
| `gslog.ExtractLabels`, `core.ExtractLabels` | `gslog.LabelsFrom` |
| `gslog.WithLogLeveler`, `core.WithLogLeveler` | `gslog.WithLeveler` |
| `gslog.WithDefaultLogLeveler`, `core.WithDefaultLogLeveler` | `gslog.WithDefaultLeveler` |
| `gslog.NewGcpHandler` | `gcp.NewHandler` |
| `gslog.GcpHandler` | `gcp.Handler` |
| `gslog.Logger` | `gcp.Logger` |
| `gslog.LoggerFunc` | `gcp.LoggerFunc` |
| `otel.OtelBaggageKey` | `otel.BaggagePrefix` |
| `otel.WithOtelBaggage` | `otel.WithBaggage` |
| `otel.WithOtelTracing` | `otel.WithTracing` |
| `detect.WithOtelTracing` | `detect.WithTracing` |

`gslog.Option`, `gslog.AttrMapper`, `gslog.LabelPair`, `gslog.MessageKey`,
and the four level constants keep their names. An option function that you
pass to a handler needs no edit. A function that matches the `AttrMapper`
signature needs no edit.

## Removed declarations

| Removed | What to use |
|---|---|
| `gslog.Options` | Nothing. The options type is internal. Pass the `With...` options to the handler constructor. |
| `core.LabelPair.IsIgnored` | Nothing. A `LabelPair` holds no ignore state. |
| `gcp.Log`, `gcp.LogSync` | `gcp.Logger`, which declares `Log`, `LogSync`, and `Flush`. |

## Changes to the log output

These changes can break a log query, an alert, or a test. Each of those
reads the text of an entry.

- A time attribute keeps its nanoseconds. v0.25.0 truncated the value to
  milliseconds. The layout is `time.RFC3339Nano`. That layout removes the
  trailing zeros of the fraction, so the number of digits varies.
- In the stdout handler, a group takes the key of an earlier attribute at
  the same level, and the handler discards that attribute. A JSON object
  cannot hold two members with one key. The gcp handler already discarded
  the attribute.
- The gcp handler omits a group when all of its members are empty.
- The handlers replace a byte that is not valid UTF-8 with U+FFFD before
  the value reaches a protobuf string.
- The stdout handler discards a top-level attribute with the key `time`,
  `timestampSeconds`, or `timestampNanos`. The Google Cloud logging agent
  reads these three keys as the timestamp.

## Changes to the error and panic behavior

- `gslog.Label` panics when the key is empty.
- `otel.WithTracing` panics when the project ID is empty.
- `gcp.Handler.Handle` returns the error of `LogSync`. The handler also
  writes the error to `os.Stderr`.
- A repeated option replaces the earlier option of the same kind. v0.25.0
  applied both options.
- `gslog.WithLogLevelFromEnvVar` logs a warning for a value that it cannot
  parse, and the option then has no effect. v0.25.0 used LevelInfo for such
  a value and logged nothing.
- When the context already holds 64 labels, `gslog.WithLabels` replaces a
  label that has the key of a label in the context. For a new key,
  `WithLabels` drops the label and logs an error with the default
  `slog.Logger`.

## New in v1.0.0

- `(*stdout.Handler).WithLeveler` returns a copy of the handler that uses
  the supplied leveler. The gcp handler already had this method.
- `gslog.WithLogLevelFromEnvVar` reads the Cloud Logging level names
  `WARNING`, `NOTICE`, `CRITICAL`, `ALERT`, and `EMERGENCY`, and it ignores
  the case of a name. It also reads an offset such as `INFO+2`. v0.25.0
  read an integer, or `DEBUG`, `INFO`, `WARN`, or `ERROR` in upper case.
- The root package declares `LevelNotice`, `LevelCritical`, `LevelAlert`,
  and `LevelEmergency` with their values.
