FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o claude-proxy-go ./cmd/server

# Final stage
FROM alpine:latest

WORKDIR /app

# Install certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary
COPY --from=builder /app/claude-proxy-go .
COPY --from=builder /app/configs ./configs

EXPOSE 8082

CMD ["./claude-proxy-go", "-f", "configs/config.json"]
