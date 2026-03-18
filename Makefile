VERSION := $(shell \
    TAG=$$(git describe --tags --exact-match 2>/dev/null); \
    if [ -n "$$TAG" ]; then \
        echo "$$TAG"; \
    else \
        echo "$$(git rev-parse --short HEAD)-$$(date -u +%Y-%m-%d)"; \
    fi)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w \
	-X github.com/hy-shine/claude-proxy-go/internal/version.Version=$(VERSION) \
	-X github.com/hy-shine/claude-proxy-go/internal/version.Commit=$(COMMIT) \
	-X github.com/hy-shine/claude-proxy-go/internal/version.BuildTime=$(BUILD_TIME)

# Build
build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -trimpath -o bin/claude-proxy-go ./cmd/server

# Dev (run with dev config)
dev:
	go run ./cmd/server -f configs/config.dev.json

# Run
run:
	go run ./cmd/server

# Test
test:
	go test -v ./...

# Test coverage
cover:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Lint
lint:
	golangci-lint run ./...

# Format
fmt:
	go fmt ./...

# Vet
vet:
	go vet ./...

# Docker build
docker-build:
	docker build -t claude-proxy-go:latest .

# Docker run
docker-run:
	docker run -p 8082:8082 -v $(PWD)/configs:/app/configs claude-proxy-go:latest

# Clean
clean:
	rm -rf bin/ coverage.out coverage.html

# All (build, test, vet)
all: build test vet

.PHONY: build run test cover lint fmt vet docker-build docker-run clean all
