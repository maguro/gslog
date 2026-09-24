GOLANGCI_LINT_VERSION := v2.13.2
GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint

EXAMPLES_DOC := docs/examples.md
EXAMPLE_PACKAGES := stdout gcp otel

.PHONY: all build test coverage lint fmt check-fmt docs check-docs tidy clean check ci

all: build check

build:
	go build ./...

test:
	go test -v -race ./...

coverage:
	go test -v -covermode=count -coverpkg=./... -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

$(GOLANGCI_LINT):
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run

fmt: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) fmt

check-fmt: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) fmt --diff

docs:
	go run ./internal/cmd/exampledoc -o $(EXAMPLES_DOC) $(EXAMPLE_PACKAGES)

check-docs:
	go run ./internal/cmd/exampledoc -check -o $(EXAMPLES_DOC) $(EXAMPLE_PACKAGES)

tidy:
	go mod tidy

clean:
	rm -f coverage.out
	@test -x $(GOLANGCI_LINT) && $(GOLANGCI_LINT) cache clean || true
	go clean -testcache

check: test lint check-fmt check-docs

ci: check coverage
