# Claude Code Proxy Go

[![Go Version](https://img.shields.io/github/go-mod/go-version/hy-shine/claude-proxy-go)](https://github.com/hy-shine/claude-proxy-go)
[![Build Status](https://img.shields.io/github/actions/workflow/status/hy-shine/claude-proxy-go/ci.yml)](https://github.com/hy-shine/claude-proxy-go/actions)
[![Docker Image](https://img.shields.io/docker/image-size/hy-shine/claude-proxy-go/latest)](https://hub.docker.com/r/hy-shine/claude-proxy-go)
[![License](https://img.shields.io/github/license/hy-shine/claude-proxy-go)](LICENSE)

[English](#english) | [中文](#中文)

---

## English

### Overview

Claude Code Proxy Go is a high-performance API proxy that translates **Anthropic-compatible** `/v1/messages` requests into **OpenAI-compatible** provider calls. It enables you to use OpenAI-compatible models (OpenAI, NVIDIA, OpenRouter, etc.) through the Anthropic API interface.

### Features

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

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Claude Code Proxy                        │
│                                                                 │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────┐ │
│  │   Handler    │───▶│  Converter   │───▶│  Eino Client     │ │
│  │  (HTTP)      │    │ (Anthropic   │    │  (OpenAI         │ │
│  │              │    │   → OpenAI)  │    │   Protocol)      │ │
│  └──────────────┘    └──────────────┘    └──────────────────┘ │
│         │                                           │          │
│         │           ┌──────────────┐                │          │
│         └──────────▶│     SSE      │◀───────────────┘          │
│                     │  (Streaming) │                           │
│                     └──────────────┘                           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                ┌─────────────────────────┐
                │  Upstream Provider      │
                │  (OpenAI/NVIDIA/        │
                │   OpenRouter/etc.)      │
                └─────────────────────────┘
```

### Quick Start

#### Using Docker

```bash
# Pull and run
docker run -d -p 8082:8082 -v $(pwd)/configs:/app/configs 1rgs/claude-code-proxy-go:latest

# Or build from source
docker build -t claude-code-proxy-go .
docker run -d -p 8082:8082 -v $(pwd)/configs:/app/configs claude-code-proxy-go
```

#### Using Binary

```bash
# Build
make build

# Run with default config
make run

# Or run with custom config
./bin/server -f configs/config.json
```

#### Using Go

```bash
go run ./cmd/server -f configs/config.json
```

### Configuration

Create or edit `configs/config.json`:

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8082
  },
  "log": {
    "level": "info"
  },
  "providers": {
    "openai": {
      "apiType": "openai",
      "apiKey": "sk-your-openai-key",
      "baseUrl": "",
      "proxy": "",
      "models": {
        "claude-sonnet": {
          "name": "gpt-4.1",
          "enabled": true,
          "maxTokens": 16384
        }
      }
    }
  },
  "retry": {
    "maxRetries": 2,
    "initialBackoffMs": 200,
    "maxBackoffMs": 800
  },
  "timeout": {
    "requestTimeout": 60,
    "streamTimeout": 300
  }
}
```

#### Configuration Options

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `server.host` | string | Server binding host | `0.0.0.0` |
| `server.port` | int | Server binding port | `8082` |
| `log.level` | string | Log level (debug/info/warn/error) | `info` |
| `providers` | object | Provider configurations | required |
| `providers.<name>.apiType` | string | Protocol type (currently only `openai`) | `openai` |
| `providers.<name>.apiKey` | string | API key for the provider | required |
| `providers.<name>.baseUrl` | string | Provider API base URL | `https://api.openai.com/v1` |
| `providers.<name>.proxy` | string | Proxy URL (http/https/socks5) for upstream API calls | `""` |
| `providers.<name>.models` | object | Model configurations | required |
| `models.<model_id>.name` | string | Upstream model name | required |
| `models.<model_id>.enabled` | bool | Whether model is active | `true` |
| `models.<model_id>.maxTokens` | int | Max tokens for this model | 16384 |
| `retry.maxRetries` | int | Maximum retry attempts | 2 |
| `retry.initialBackoffMs` | int | Initial backoff in milliseconds | 200 |
| `retry.maxBackoffMs` | int | Maximum backoff in milliseconds | 800 |
| `timeout.requestTimeout` | int | Request timeout in seconds | 60 |
| `timeout.streamTimeout` | int | Streaming timeout in seconds | 300 |

### API Usage

#### Non-Streaming Request

```bash
curl -X POST http://localhost:8082/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "Hello, world!"}
    ]
  }'
```

#### Streaming Request

```bash
curl -X POST http://localhost:8082/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet",
    "max_tokens": 1024,
    "stream": true,
    "messages": [
      {"role": "user", "content": "Count to 5"}
    ]
  }'
```

#### With Tools

```bash
curl -X POST http://localhost:8082/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "What is the weather in Tokyo?"}
    ],
    "tools": [
      {
        "name": "get_weather",
        "description": "Get weather information for a location",
        "input_schema": {
          "type": "object",
          "properties": {
            "location": {"type": "string", "description": "City name"}
          },
          "required": ["location"]
        }
      }
    ]
  }'
```

### Development

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

### License

MIT License - see [LICENSE](LICENSE) file for details.

---

## 中文

### 概述

Claude Code Proxy Go 是一个高性能 API 代理，将 **Anthropic 兼容** 的 `/v1/messages` 请求转换为 **OpenAI 兼容** 的 provider 调用。它允许您通过 Anthropic API 接口使用 OpenAI 兼容的模型（OpenAI、NVIDIA、OpenRouter 等）。

### 功能特性

- **协议转换**：Anthropic 和 OpenAI API 格式之间的无缝转换
- **多 Provider 支持**：支持任何 OpenAI 兼容的 provider（OpenAI、NVIDIA、OpenRouter 等）
- **流式支持**：完整的 Server-Sent Events (SSE) 流式传输，采用 Anthropic 事件序列
- **工具调用**：支持工具定义、tool_choice (auto/any/tool) 和工具结果处理
- **高级参数映射**：
  - `thinking.budget_tokens` → `reasoning_effort` (low/medium/high)
  - `top_k` → `top_p` 映射（当 `top_p` 未显式设置时）
- **重试机制**：针对限流（429）和服务器错误（5xx）的自动指数退避重试
- **代理支持**：支持 HTTP/HTTPS/SOCKS5 代理访问上游 API
- **生产就绪**：严格的请求验证、全面的错误处理、可配置的超时

### 架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        Claude Code Proxy                        │
│                                                                 │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────┐ │
│  │   Handler    │───▶│  Converter   │───▶│  Eino Client     │ │
│  │  (HTTP)      │    │ (Anthropic   │    │  (OpenAI         │ │
│  │              │    │   → OpenAI)  │    │   Protocol)      │ │
│  └──────────────┘    └──────────────┘    └──────────────────┘ │
│         │                                           │          │
│         │           ┌──────────────┐                │          │
│         └──────────▶│     SSE      │◀───────────────┘          │
│                     │  (Streaming) │                           │
│                     └──────────────┘                           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                ┌─────────────────────────┐
                │  Upstream Provider      │
                │  (OpenAI/NVIDIA/        │
                │   OpenRouter/etc.)      │
                └─────────────────────────┘
```

### 快速开始

#### 使用 Docker

```bash
# 拉取并运行
docker run -d -p 8082:8082 -v $(pwd)/configs:/app/configs 1rgs/claude-code-proxy-go:latest

# 或从源码构建
docker build -t claude-code-proxy-go .
docker run -d -p 8082:8082 -v $(pwd)/configs:/app/configs claude-code-proxy-go
```

#### 使用二进制

```bash
# 构建
make build

# 使用默认配置运行
make run

# 或使用自定义配置运行
./bin/server -f configs/config.json
```

#### 使用 Go

```bash
go run ./cmd/server -f configs/config.json
```

### 配置

创建或编辑 `configs/config.json`：

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8082
  },
  "log": {
    "level": "info"
  },
  "providers": {
    "openai": {
      "apiType": "openai",
      "apiKey": "sk-your-openai-key",
      "baseUrl": "",
      "proxy": "",
      "models": {
        "claude-sonnet": {
          "name": "gpt-4.1",
          "enabled": true,
          "maxTokens": 16384
        }
      }
    }
  },
  "retry": {
    "maxRetries": 2,
    "initialBackoffMs": 200,
    "maxBackoffMs": 800
  },
  "timeout": {
    "requestTimeout": 60,
    "streamTimeout": 300
  }
}
```

#### 配置选项

| 字段 | 类型 | 描述 | 默认值 |
|-------|------|-------------|---------|
| `server.host` | string | 服务器绑定地址 | `0.0.0.0` |
| `server.port` | int | 服务器绑定端口 | `8082` |
| `log.level` | string | 日志级别 (debug/info/warn/error) | `info` |
| `providers` | object | Provider 配置 | 必填 |
| `providers.<name>.apiType` | string | 协议类型（当前仅支持 `openai`） | `openai` |
| `providers.<name>.apiKey` | string | Provider API 密钥 | 必填 |
| `providers.<name>.baseUrl` | string | Provider API 基础 URL | `https://api.openai.com/v1` |
| `providers.<name>.proxy` | string | 上游 API 调用代理 URL (http/https/socks5) | `""` |
| `providers.<name>.models` | object | 模型配置 | 必填 |
| `models.<model_id>.name` | string | 上游模型名称 | 必填 |
| `models.<model_id>.enabled` | bool | 模型是否启用 | `true` |
| `models.<model_id>.maxTokens` | int | 模型最大 token 数 | 16384 |
| `retry.maxRetries` | int | 最大重试次数 | 2 |
| `retry.initialBackoffMs` | int | 初始退避时间（毫秒） | 200 |
| `retry.maxBackoffMs` | int | 最大退避时间（毫秒） | 800 |
| `timeout.requestTimeout` | int | 请求超时（秒） | 60 |
| `timeout.streamTimeout` | int | 流式超时（秒） | 300 |

### API 使用示例

#### 非流式请求

```bash
curl -X POST http://localhost:8082/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "Hello, world!"}
    ]
  }'
```

#### 流式请求

```bash
curl -X POST http://localhost:8082/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet",
    "max_tokens": 1024,
    "stream": true,
    "messages": [
      {"role": "user", "content": "数到5"}
    ]
  }'
```

#### 使用工具

```bash
curl -X POST http://localhost:8082/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "东京的天气怎么样？"}
    ],
    "tools": [
      {
        "name": "get_weather",
        "description": "获取某个位置的天气信息",
        "input_schema": {
          "type": "object",
          "properties": {
            "location": {"type": "string", "description": "城市名称"}
          },
          "required": ["location"]
        }
      }
    ]
  }'
```

### 开发

```bash
# 运行测试
make test

# 运行覆盖率测试
make cover

# 代码检查
make lint

# 代码格式化
make fmt
```

### 许可证

MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。
