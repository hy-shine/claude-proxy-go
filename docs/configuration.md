# 配置设计文档

## 项目概述

claude-code-proxy-go 是一个 API 代理，将 Anthropic 风格的 `/v1/messages` 请求转换为 OpenAI 兼容的提供者调用。

---

## 现有配置结构

### 完整配置示例

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
      "models": {
        "oai_haiku": {
          "name": "gpt-4.1-mini",
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

---

## 字段详解

### ServerConfig

| 字段 | 类型 | 默认值 | 用途 |
|------|------|--------|------|
| `host` | string | `"0.0.0.0"` | 监听地址 |
| `port` | int | `8082` | 监听端口 |

### LogConfig

| 字段 | 类型 | 默认值 | 用途 |
|------|------|--------|------|
| `level` | string | `"info"` | 日志级别 (debug/info/warn/error) |

### RetryConfig

| 字段 | 类型 | 默认值 | 用途 |
|------|------|--------|------|
| `maxRetries` | int | `2` | 请求失败时的最大重试次数 |
| `initialBackoffMs` | int | `200` | 首次重试等待时间 (ms) |
| `maxBackoffMs` | int | `800` | 最大等待时间 (ms) |

**重试间隔计算公式**：
```
间隔 = min(initialBackoff * 2^attempt, maxBackoff)
```

| 重试次数 | 等待时间 (默认配置) |
|----------|-------------------|
| 第1次 | 200ms |
| 第2次 | 400ms |
| 第3次 | 800ms (达到上限) |

### TimeoutConfig

| 字段 | 类型 | 默认值 | 用途 |
|------|------|--------|------|
| `requestTimeout` | int | `60` | 非流式请求超时 (秒) |
| `streamTimeout` | int | `300` | 流式响应超时 (秒) |

**说明**：
- `requestTimeout`: 非流式请求的整体超时
- `streamTimeout`: 流式响应的整体超时，流式超时远大于普通请求，因为大模型生成大量内容需要时间

### ProviderConfig

| 字段 | 类型 | 用途 |
|------|------|------|
| `apiType` | string | API 类型 (当前仅支持 `openai`) |
| `apiKey` | string | 上游 API 密钥 |
| `baseUrl` | string | 上游 API 地址 (默认 `https://api.openai.com/v1`) |
| `models` | map | 模型映射配置 |

### ModelConfig

| 字段 | 类型 | 用途 |
|------|------|------|
| `name` | string | 上游模型名称 (必需) |
| `enabled` | bool | 是否启用 (默认 true) |
| `maxTokens` | int | 输出 token 上限限制 |

---

## maxTokens 字段说明

### 作用机制

`maxTokens` 有两个用途：

1. **上限限制 (Guard)**
   - 位置: `pkg/eino/client.go:127-131`
   - 如果用户请求的 `max_tokens` 超过配置值，返回 400 错误
   - 错误信息: `"max_tokens exceeds configured limit (%d) for model %s"`

2. **请求传递**
   - 位置: `pkg/eino/client.go:157-159`
   - 将用户的 `max_tokens` 值传递给上游 provider

### 触发条件

只有同时满足以下条件才会触发检查：
1. 用户请求中指定了 `max_tokens`
2. 配置中 `maxTokens > 0`

### maxTokens vs Context Window

| 概念 | 含义 | 示例 |
|------|------|------|
| **context window** | 模型能接收的最大输入+输出总长度 | 128K tokens |
| **maxTokens** | 单次请求中**生成**的最大输出 token 数 | 4096 tokens |

**结论**: Context window 是模型的固有属性，不应作为配置项。上游 provider 会自动处理超出 context window 的请求。

---

## 建议添加的配置

### 高优先级

| 配置 | 用途 | 理由 |
|------|------|------|
| `maxInputTokens` | 限制输入 token | 保护上游，控制成本 |
| `rateLimit` | 请求速率限制 | 防止滥用，保护后端 |
| `cors.allowedOrigins` | 跨域配置 | 前端调用需要 |

### 中优先级

| 配置 | 用途 | 理由 |
|------|------|------|
| `log.format` | 日志格式 (json/text) | 生产环境通常要 json |
| `log.output` | 日志输出 (stdout/file) | 方便日志收集 |
| `apiKey` | 代理自身认证 | 防止未授权访问 |
| `maxTools` | 最大工具调用数 | 防止工具滥用 |

### 模型级配置建议

```json
{
  "models": {
    "oai_sonnet": {
      "name": "gpt-4.1",
      "maxTokens": 16384,
      "maxInputTokens": 100000,
      "temperature": 0.7,
      "topP": 0.9,
      "stopSequences": ["END"],
      "thinking": {
        "enabled": true,
        "budgetTokens": 1024
      }
    }
  }
}
```

| 配置 | 用途 | 场景 |
|------|------|------|
| `maxInputTokens` | 限制输入 token 数 | 成本控制、保护上游 |
| `temperature` | 默认采样温度 | 不同模型有最佳实践温度 |
| `topP` | 默认 nucleus 采样 | 与 temperature 二选一 |
| `stopSequences` | 默认停止序列 | 特定模型的输出格式控制 |
| `thinking.enabled` | 默认启用 thinking | 对话模型默认开启推理 |
| `thinking.budgetTokens` | 默认推理预算 | 控制推理 token 消耗 |

---

## 调优建议

### 场景配置推荐

| 场景 | requestTimeout | streamTimeout | maxRetries | maxBackoffMs |
|------|----------------|---------------|------------|--------------|
| 快速调试 | 30 | 60 | 1 | 500 |
| 大模型输出 | 60 | 600 | 2 | 1000 |
| 高并发 | 60 | 300 | 3 | 2000 |
| 低延迟需求 | 30 | 120 | 1 | 300 |

---

## 当前支持的请求参数

| Anthropic 参数 | 代理支持 | 模型级默认 |
|----------------|---------|-----------|
| `max_tokens` | ✅ | ✅ `maxTokens` |
| `temperature` | ✅ | ❌ |
| `top_p` | ✅ | ❌ |
| `top_k` | ✅ | ❌ |
| `stop_sequences` | ✅ | ❌ |
| `thinking.budget_tokens` | ✅ | ❌ |
| `thinking.enabled` | ✅ | ❌ |
| `tools` | ✅ | ❌ |