# gslog

![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.21-%23007d9c)
[![Documentation](https://godoc.org/github.com/maguro/gslog?status.svg)](http://godoc.org/github.com/maguro/gslog)
[![Go Report Card](https://goreportcard.com/badge/github.com/maguro/gslog)](https://goreportcard.com/report/github.com/maguro/gslog)
[![codecov](https://codecov.io/gh/maguro/gslog/graph/badge.svg?token=3FAJJ2SIZB)](https://codecov.io/gh/maguro/gslog)
[![License](https://img.shields.io/github/license/maguro/gslog)](./LICENSE)

A Google Cloud Logging [Handler](https://pkg.go.dev/log/slog#Handler) implementation for [slog](https://go.dev/blog/slog).

---

Critical level log records will be sent synchronously.


## 🚀 Install

```sh
go get m4o.io/gslog
```

**Compatibility**: go >= 1.21

No breaking changes will be made to exported APIs before v2.0.0.

## 💡 Usage

```go
package main

import (
	"log/slog"

	"github.com/jeffry-luqman/zlog"
)

func main() {
	logger := zlog.New()
	logger.Debug("Hello, World!")
	logger.Info("Hello, World!")
	logger.Warn("Hello, World!", slog.String("foo", "bar"), slog.Bool("baz", true))
	logger.Error("Hello, World!", slog.String("foo", "bar"))
}
```


## 🤝 Contributing

- Fork the [project](https://github.com/maguro/gslog)
- Fix [open issues](https://github.com/maguro/gslog/issues) or request new features

Don't hesitate ;)

## 👤 Contributors

![Contributors](https://contrib.rocks/image?repo=maguro/gslog)

## 📝 License

Copyright © 2024 The original author or authors.

This project is [AL2.0](./LICENSE) licensed.
