# Configuration Guide

This document describes all configuration options for Claude Proxy Go.

## Configuration File

Default location: `configs/config.json`

## Complete Configuration Example

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
        },
        "claude-opus": {
          "name": "o1",
          "enabled": true,
          "maxTokens": 32768
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

## Configuration Options

### Server Settings

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `server.host` | string | Server binding host | `0.0.0.0` |
| `server.port` | int | Server binding port | `8082` |

### Log Settings

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `log.level` | string | Log level: `debug`, `info`, `warn`, `error` | `info` |

### Provider Settings

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `providers` | object | Map of provider configurations | required |
| `providers.<name>.apiType` | string | API protocol type (currently only `openai` supported) | `openai` |
| `providers.<name>.apiKey` | string | API key for the provider | required |
| `providers.<name>.baseUrl` | string | Provider API base URL | `https://api.openai.com/v1` |
| `providers.<name>.proxy` | string | Proxy URL for upstream API calls (http/https/socks5) | `""` |
| `providers.<name>.customHeaders` | object | Custom HTTP headers to add to all requests | `{}` |
| `providers.<name>.models` | object | Model configurations for this provider | required |

### Model Settings

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `models.<model_id>.name` | string | Upstream model name to use | required |
| `models.<model_id>.enabled` | bool | Whether this model is active | `true` |
| `models.<model_id>.maxTokens` | int | Maximum tokens for this model | `16384` |

**Important**: `model_id` must be globally unique across all providers. Duplicate `model_id` values will cause a configuration error.

**Model Resolution**: Client sends `model` field as a `model_id`, which is resolved to the provider's actual model name.

Example:
```json
{
  "models": {
    "claude-sonnet": {
      "name": "gpt-4.1"
    }
  }
}
```
Client sends: `"model": "claude-sonnet"` → Provider receives: `"model": "gpt-4.1"`

### Retry Settings

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `retry.maxRetries` | int | Maximum retry attempts | `2` |
| `retry.initialBackoffMs` | int | Initial backoff duration (milliseconds) | `200` |
| `retry.maxBackoffMs` | int | Maximum backoff duration (milliseconds) | `800` |

Retry is triggered for:
- HTTP 429 (Rate Limited)
- HTTP 5xx (Server Error)

Backoff uses exponential strategy: `min(initialBackoffMs * 2^attempt, maxBackoffMs)`

### Timeout Settings

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `timeout.requestTimeout` | int | Non-streaming request timeout (seconds) | `60` |
| `timeout.streamTimeout` | int | Streaming request timeout (seconds) | `300` |

## Multi-Provider Example

```json
{
  "providers": {
    "openai": {
      "apiType": "openai",
      "apiKey": "sk-openai-key",
      "models": {
        "claude-sonnet": { "name": "gpt-4.1" }
      }
    },
    "nvidia": {
      "apiType": "openai",
      "apiKey": "nvapi-xxx",
      "baseUrl": "https://integrate.api.nvidia.com/v1",
      "models": {
        "claude-opus": { "name": "meta/llama-3.1-405b-instruct" }
      }
    },
    "openrouter": {
      "apiType": "openai",
      "apiKey": "sk-or-xxx",
      "baseUrl": "https://openrouter.ai/api/v1",
      "models": {
        "claude-haiku": { "name": "anthropic/claude-3-haiku" }
      }
    }
  }
}
```

## Proxy Configuration

Supports HTTP, HTTPS, and SOCKS5 proxies for upstream API calls:

```json
{
  "providers": {
    "openai": {
      "apiKey": "sk-xxx",
      "proxy": "http://127.0.0.1:7890"
    }
  }
}
```

Proxy URL formats:
- HTTP: `http://host:port`
- HTTPS: `https://host:port`
- SOCKS5: `socks5://host:port`

## Custom Headers

You can add custom HTTP headers to all requests sent to a provider:

```json
{
  "providers": {
    "llama": {
      "apiType": "openai",
      "apiKey": "your-api-key",
      "baseUrl": "https://your-endpoint/v1",
      "customHeaders": {
        "X-Api-Version": "2024-01-01",
        "X-Request-Source": "claude-proxy"
      },
      "models": {
        "my-model": { "name": "llama-3.1-8b" }
      }
    }
  }
}
```

Custom headers are useful for:
- API version headers (e.g., `X-Api-Version`)
- Authentication headers (e.g., `X-Api-Key` for additional auth)
- Request tracking headers (e.g., `X-Request-Id`)
- Custom metadata headers
