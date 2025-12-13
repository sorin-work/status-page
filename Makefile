BIN := bin
GO ?= go

.PHONY: all build test race lint fmt vet run clean

all: lint test build

build:
	$(GO) build -trimpath -o $(BIN)/ ./cmd/...

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
