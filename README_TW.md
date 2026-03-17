# Claude Code Proxy Go

[![Go Version](https://img.shields.io/github/go-mod/go-version/hy-shine/claude-proxy-go)](https://github.com/hy-shine/claude-proxy-go)
[![Build Status](https://img.shields.io/github/actions/workflow/status/hy-shine/claude-proxy-go/ci.yml)](https://github.com/hy-shine/claude-proxy-go/actions)
[![Docker Image](https://img.shields.io/docker/image-size/hy-shine/claude-proxy-go/latest)](https://hub.docker.com/r/hy-shine/claude-proxy-go)
[![License](https://img.shields.io/github/license/hy-shine/claude-proxy-go)](LICENSE)

**[English](README.md)** | **[简体中文](README_CN.md)**

---

## 概述

Claude Proxy Go 是一個高效能 API 代理，將 **Anthropic 相容** 的 `/v1/messages` 請求轉換為 **OpenAI 相容** 的 provider 呼叫。它允許您透過 Anthropic API 介面使用 OpenAI 相容的模型（OpenAI、NVIDIA、OpenRouter 等）。

## 功能特性

- **協定轉換**：Anthropic 和 OpenAI API 格式之間的無縫轉換
- **多 Provider 支援**：支援任何 OpenAI 相容的 provider（OpenAI、NVIDIA、OpenRouter 等）
- **串流支援**：完整的 Server-Sent Events (SSE) 串流傳輸，採用 Anthropic 事件序列
- **工具呼叫**：支援工具定義、tool_choice (auto/any/tool) 和工具結果處理
- **進階參數映射**：
  - `thinking.budget_tokens` → `reasoning_effort` (low/medium/high)
  - `top_k` → `top_p` 映射（當 `top_p` 未明確設定時）
- **重試機制**：針對限流（429）和伺服器錯誤（5xx）的自動指數退避重試
- **代理支援**：支援 HTTP/HTTPS/SOCKS5 代理存取上游 API
- **生產就緒**：嚴格的請求驗證、全面的錯誤處理、可設定的逾時

## 架構

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

## 快速開始

### 使用 Go Install

```bash
# 直接從 GitHub 安裝
go install github.com/hy-shine/claude-proxy-go/cmd/server@latest

# 使用設定執行
claude-proxy-go -f configs/config.json
```

### 使用二進位檔案

```bash
# 建置
make build

# 使用預設設定執行
make run

# 或使用自訂設定執行
./bin/claude-proxy-go -f configs/config.json
```

### 使用 Docker

```bash
# 拉取並執行
docker run -d -p 8082:8082 -v $(pwd)/configs:/app/configs 1rgs/claude-proxy-go:latest

# 或從原始碼建置
docker build -t claude-proxy-go .
docker run -d -p 8082:8082 -v $(pwd)/configs:/app/configs claude-proxy-go
```

## 設定

建立 `configs/config.json`：

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

完整設定選項見 [CONFIG.md](CONFIG.md)。

## 開發

```bash
# 執行測試
make test

# 執行覆蓋率測試
make cover

# 程式碼檢查
make lint

# 程式碼格式化
make fmt
```

## 授權條款

MIT 授權條款 - 詳見 [LICENSE](LICENSE) 檔案。
