BINARY := splat-viewer
GOFLAGS ?=
GOLANGCI_LINT_VERSION ?= v2.12.2

.PHONY: build lint test clean

build:
	go build $(GOFLAGS) -o $(BINARY) .

lint: $(GOLANGCI_LINT)
	go vet ./...
	$(GOLANGCI_LINT) run ./...

test:
	go test ./... -count=1

clean:
	rm -f $(BINARY)

GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint
$(GOLANGCI_LINT):
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
