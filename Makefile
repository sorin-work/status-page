BIN := bin
GO ?= go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/sorin-work/status-page/internal/version.Version=$(VERSION)

.PHONY: all build test race lint fmt vet run clean

all: lint test build

build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)/ ./cmd/...

test:
	$(GO) test ./...

race:
	$(GO) test -race ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

vet:
	$(GO) vet ./...

run:
	$(GO) run ./cmd/statuspage

clean:
	rm -rf $(BIN)
