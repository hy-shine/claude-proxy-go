# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-03-27

### Added
- **Thinking/Reasoning Support**: Full support for Claude's extended thinking feature
  - Map `thinking.budget_tokens` to OpenAI `reasoning_effort` (low/medium/high)
  - Convert upstream `reasoning_content` to Claude thinking blocks
  - Support `thinking.type=adaptive` with `effort` parameter
  - Preserve thinking blocks in message history
- **Enhanced Logging**: Replaced default logger with zap for better performance
  - Structured request/response logging
  - Configurable log levels
- **CI/CD**: GitHub Actions workflow for automated binary releases

### Changed
- Improved CLI flags and removed Docker documentation
- Renamed project to "Claude Proxy Go"
- Health check endpoint renamed from `/healthz` to `/health`

### Fixed
- Removed backoff validation bug and dead code in config
- Improved error logging with more context
- Correctly map `thinking_tokens` in streaming responses
- Move `effort` to `output_config` and add display field

## [0.1.0] - 2026-03-17

### Added
- Initial release
- Protocol translation between Anthropic and OpenAI API formats
- Multi-provider support (OpenAI, NVIDIA, OpenRouter)
- SSE streaming with Anthropic event sequence
- Tool calling support (auto/any/tool choice types)
- Retry mechanism with exponential backoff
- HTTP/HTTPS/SOCKS5 proxy support
- Strict request validation and error handling

[Unreleased]: https://github.com/hy-shine/claude-proxy-go/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/hy-shine/claude-proxy-go/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/hy-shine/claude-proxy-go/releases/tag/v0.1.0
