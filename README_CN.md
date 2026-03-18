# Claude Proxy Go

[![Go Version](https://img.shields.io/github/go-mod/go-version/hy-shine/claude-proxy-go)](https://github.com/hy-shine/claude-proxy-go)
[![License](https://img.shields.io/github/license/hy-shine/claude-proxy-go)](LICENSE)

**[English](README.md)** | **[繁體中文](README_TW.md)**

---

## 概述

Claude Proxy Go 是一个高性能 API 代理，将 **Anthropic 兼容** 的 `/v1/messages` 请求转换为 **OpenAI 兼容** 的 provider 调用。它允许您通过 Anthropic API 接口使用 OpenAI 兼容的模型（OpenAI、NVIDIA、OpenRouter 等）。

## 功能特性

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

## 架构

```
┌─────────────────────────────────────────────────────────────────┐
│                    Claude Proxy Go                              │
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

## 快速开始

### 使用 Go Install

```bash
# 直接从 GitHub 安装
go install github.com/hy-shine/claude-proxy-go/cmd/server@latest

# 使用配置运行
claude-proxy-go -f configs/config.json
```

### 使用二进制

```bash
# 构建
make build

# 使用默认配置运行
make run

# 或使用自定义配置运行
./bin/claude-proxy-go -f configs/config.json
```

## 配置

创建 `configs/config.json`：

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

完整配置选项见 [CONFIG.md](CONFIG.md)。

## 开发

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

## 许可证

MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。
