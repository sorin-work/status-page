BIN := bin
GO ?= go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/sorin-work/status-page/internal/version.Version=$(VERSION)

.PHONY: all build test race lint fmt vet run demo docker-up docker-down clean

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

# Runs the server and the demo target locally with demo data.
demo:
	$(GO) run ./cmd/demotarget & \
	SEED_DEMO=true ADMIN_PASSWORD=demo-password DATABASE_PATH=data/demo.db $(GO) run ./cmd/statuspage; \
	kill %1

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

clean:
	rm -rf $(BIN)
