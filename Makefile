GOLANGCI_LINT_VERSION := v2.15.0
GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint
GOVULNCHECK_VERSION := v1.9.0

EXAMPLES_DOC := docs/examples.md

# Only tests use these paths. The coverage profile does not include them.
COVERAGE_EXCLUDE := /benchmark/|/testdata/|^m4o\.io/gslog/internal/testhelp/

.PHONY: all build test coverage lint fmt check-fmt vuln docs check-docs tidy clean check ci

all: build check

build:
	go build ./...

test:
	go test -v -race ./...

coverage:
	go test -v -covermode=count -coverpkg=./... -coverprofile=coverage.out ./...
	grep -vE '$(COVERAGE_EXCLUDE)' coverage.out > coverage.filtered.out
	mv coverage.filtered.out coverage.out
	go tool cover -func=coverage.out

$(GOLANGCI_LINT):
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run

fmt: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) fmt

check-fmt: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) fmt --diff

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

docs:
	go run ./internal/cmd/exampledoc -o $(EXAMPLES_DOC)

check-docs:
	go run ./internal/cmd/exampledoc -check -o $(EXAMPLES_DOC)

tidy:
	go mod tidy

clean:
	rm -f coverage.out
	@test -x $(GOLANGCI_LINT) && $(GOLANGCI_LINT) cache clean || true
	go clean -testcache

check: test lint check-fmt check-docs

ci: check coverage
