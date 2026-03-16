# Build
build:
	CGO_ENABLED=0 go build -o bin/server ./cmd/server

# Dev (run with dev config)
dev:
	go run ./cmd/server -f configs/config.dev.json

# Run
run:
	go run ./cmd/server

# Run with config
run-config:
	go run ./cmd/server -f configs/config.json

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
	docker build -t claude-code-proxy-go:latest .

# Docker run
docker-run:
	docker run -p 8082:8082 -v $(PWD)/configs:/app/configs claude-code-proxy-go:latest

# Clean
clean:
	rm -rf bin/ coverage.out coverage.html

# All (build, test, vet)
all: build test vet

.PHONY: build run run-config test cover lint fmt vet docker-build docker-run clean all
