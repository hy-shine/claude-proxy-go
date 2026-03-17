# Claude Code Proxy Go

[![Go Version](https://img.shields.io/github/go-mod/go-version/hy-shine/claude-proxy-go)](https://github.com/hy-shine/claude-proxy-go)
[![Build Status](https://img.shields.io/github/actions/workflow/status/hy-shine/claude-proxy-go/ci.yml)](https://github.com/hy-shine/claude-proxy-go/actions)
[![Docker Image](https://img.shields.io/docker/image-size/hy-shine/claude-proxy-go/latest)](https://hub.docker.com/r/hy-shine/claude-proxy-go)
[![License](https://img.shields.io/github/license/hy-shine/claude-proxy-go)](LICENSE)

**[简体中文](README_CN.md)** | **[繁體中文](README_TW.md)**

---

## Overview

Claude Proxy Go is a high-performance API proxy that translates **Anthropic-compatible** `/v1/messages` requests into **OpenAI-compatible** provider calls. It enables you to use OpenAI-compatible models (OpenAI, NVIDIA, OpenRouter, etc.) through the Anthropic API interface.

## Features

- **Protocol Translation**: Seamless conversion between Anthropic and OpenAI API formats
- **Multi-Provider Support**: Works with any OpenAI-compatible provider (OpenAI, NVIDIA, OpenRouter, etc.)
- **Streaming Support**: Full Server-Sent Events (SSE) streaming with Anthropic event sequence
- **Tool Calling**: Support for tool definitions, tool_choice (auto/any/tool), and tool result handling
- **Advanced Parameter Mapping**:
  - `thinking.budget_tokens` → `reasoning_effort` (low/medium/high)
  - `top_k` → `top_p` mapping when `top_p` is not explicitly set
- **Retry Mechanism**: Automatic retry with exponential backoff for rate limits (429) and server errors (5xx)
- **Proxy Support**: HTTP/HTTPS/SOCKS5 proxy for upstream API calls
- **Production-Ready**: Strict request validation, comprehensive error handling, configurable timeouts

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Claude Code Proxy                            │
│                                                                 │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────┐  │
│  │   Handler    │───▶│  Converter   │───▶│   Eino Client    │  │
│  │    (HTTP)    │    │ (Anthropic   │    │    (OpenAI       │  │
│  │              │    │  → OpenAI)   │    │    Protocol)     │  │
│  └──────────────┘    └──────────────┘    └──────────────────┘  │
│         │                   │                     │            │
│         │           ┌───────┴───────┐             │            │
│         └──────────▶│     SSE       │◀────────────┘            │
│                     │  (Streaming)  │                         │
│                     └───────────────┘                         │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
            ┌─────────────────────────┐
            │    Upstream Provider    │
            │   (OpenAI/NVIDIA/       │
            │   OpenRouter/etc.)      │
            └─────────────────────────┘
```

## Quick Start

### Using Go Install

```bash
# Install directly from GitHub
go install github.com/hy-shine/claude-proxy-go/cmd/server@latest

# Run with config
claude-proxy-go -f configs/config.json
```

### Using Binary

```bash
# Build
make build

# Run with default config
make run

# Or run with custom config
./bin/claude-proxy-go -f configs/config.json
```

### Using Docker

```bash
# Pull and run
docker run -d -p 8082:8082 -v $(pwd)/configs:/app/configs 1rgs/claude-proxy-go:latest

# Or build from source
docker build -t claude-proxy-go .
docker run -d -p 8082:8082 -v $(pwd)/configs:/app/configs claude-proxy-go
```

## Configuration

Create `configs/config.json`:

```json
{
  "server": { "host": "0.0.0.0", "port": 8082 },
  "providers": {
    "openai": {
      "apiType": "openai",
      "apiKey": "sk-your-openai-key",
      "models": {
        "claude-sonnet": { "name": "gpt-4.1" }
      }
    }
  }
}
```

See [CONFIG.md](CONFIG.md) for full configuration options.

## Development

```bash
# Run tests
make test

# Run with coverage
make cover

# Lint code
make lint

# Format code
make fmt
```

## License

MIT License - see [LICENSE](LICENSE) file for details.
