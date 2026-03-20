package eino

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/hy-shine/claude-proxy-go/internal/config"
	"github.com/hy-shine/claude-proxy-go/internal/converter"
	"github.com/hy-shine/claude-proxy-go/internal/logger"
)

var statusCodePattern = regexp.MustCompile(`(?i)\bstatus(?:\s*code)?[:=]?\s*(\d{3})\b`)

type StreamReader interface {
	Recv() (*schema.Message, error)
	Close()
}

type ClientError struct {
	StatusCode int
	Message    string
	Err        error
}

func (e *ClientError) Error() string {
	if e.Err == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func (e *ClientError) Unwrap() error {
	return e.Err
}

type Client struct {
	cfg    *config.Config
	models map[string]model.ToolCallingChatModel
}

func NewClient(cfg *config.Config) (*Client, error) {
	client := &Client{
		cfg:    cfg,
		models: make(map[string]model.ToolCallingChatModel),
	}

	models := cfg.EnabledModels()
	if len(models) == 0 {
		return nil, fmt.Errorf("no enabled models configured")
	}

	for modelID, modelCfg := range models {
		chatModel, err := buildProviderModel(context.Background(), modelCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize model %s: %w", modelID, err)
		}
		client.models[modelID] = chatModel
	}

	return client, nil
}

func buildProviderModel(ctx context.Context, cfg config.ResolvedModel) (model.ToolCallingChatModel, error) {
	if cfg.APIType != "openai" {
		return nil, fmt.Errorf("unsupported api_type: %s", cfg.APIType)
	}

	openaiCfg := converter.GetOpenAIConfig(cfg.APIKey, cfg.BaseURL, cfg.Name)
	if cfg.Proxy != "" {
		httpClient, err := buildHTTPClientWithProxy(cfg.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy for model %s: %w", cfg.ModelID, err)
		}
		openaiCfg.HTTPClient = httpClient
	}
	return openai.NewChatModel(ctx, openaiCfg)
}

func buildHTTPClientWithProxy(proxyAddress string) (*http.Client, error) {
	normalized := strings.TrimSpace(proxyAddress)
	if normalized == "" {
		return nil, fmt.Errorf("proxy address is empty")
	}
	if strings.HasPrefix(strings.ToLower(normalized), "socks://") {
		normalized = "socks5://" + normalized[len("socks://"):]
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proxy: %w", err)
	}
	if parsed.Scheme == "" {
		return nil, fmt.Errorf("proxy scheme is required")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("proxy host is required")
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyURL(parsed)

	return &http.Client{Transport: transport}, nil
}

func (c *Client) Generate(ctx context.Context, modelID string, messages []*schema.Message, opts *converter.ChatOptions) (*schema.Message, error) {
	chatModel, chatOpts, err := c.prepareCall(modelID, opts)
	if err != nil {
		return nil, err
	}

	return retry(ctx, c.cfg, func() (*schema.Message, error) {
		return chatModel.Generate(ctx, messages, chatOpts...)
	})
}

func (c *Client) Stream(ctx context.Context, modelID string, messages []*schema.Message, opts *converter.ChatOptions) (StreamReader, error) {
	chatModel, chatOpts, err := c.prepareCall(modelID, opts)
	if err != nil {
		return nil, err
	}

	stream, err := retry(ctx, c.cfg, func() (*schema.StreamReader[*schema.Message], error) {
		return chatModel.Stream(ctx, messages, chatOpts...)
	})

	if err != nil {
		return nil, err
	}
	return &streamReaderWrapper{stream: stream}, nil
}

func (c *Client) prepareCall(modelID string, opts *converter.ChatOptions) (model.ToolCallingChatModel, []model.Option, error) {
	target, err := c.cfg.ResolveModel(modelID)
	if err != nil {
		return nil, nil, &ClientError{
			StatusCode: http.StatusBadRequest,
			Message:    err.Error(),
			Err:        err,
		}
	}

	chatModel, ok := c.models[target.ModelID]
	if !ok {
		return nil, nil, &ClientError{
			StatusCode: http.StatusInternalServerError,
			Message:    fmt.Sprintf("model not initialized: %s", modelID),
		}
	}

	if opts != nil && opts.MaxTokens != nil && target.MaxTokens > 0 && *opts.MaxTokens > target.MaxTokens {
		return nil, nil, &ClientError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("max_tokens exceeds configured limit (%d) for model %s", target.MaxTokens, modelID),
		}
	}

	chatOpts, err := convertOptions(opts)
	if err != nil {
		return nil, nil, &ClientError{
			StatusCode: http.StatusBadRequest,
			Message:    err.Error(),
			Err:        err,
		}
	}
	logger.Debugf("Model resolved: model_id=%s provider=%s api_type=%s upstream=%s", target.ModelID, target.Provider, target.APIType, target.Name)
	return chatModel, chatOpts, nil
}

func convertOptions(opts *converter.ChatOptions) ([]model.Option, error) {
	if opts == nil {
		return nil, nil
	}

	var chatOpts []model.Option

	if opts.Temperature != nil {
		temp := float32(*opts.Temperature)
		chatOpts = append(chatOpts, model.WithTemperature(temp))
	}
	if opts.MaxTokens != nil {
		maxTok := *opts.MaxTokens
		chatOpts = append(chatOpts, model.WithMaxTokens(maxTok))
	}
	if opts.TopP != nil {
		topP := float32(*opts.TopP)
		chatOpts = append(chatOpts, model.WithTopP(topP))
	}
	// OpenAI does not expose top_k directly; we map it to top_p only when top_p
	// is not explicitly provided.
	if opts.TopP == nil && opts.TopK != nil {
		mappedTopP := mapTopKToTopP(*opts.TopK)
		chatOpts = append(chatOpts, model.WithTopP(mappedTopP))
	}
	if len(opts.Stop) > 0 {
		chatOpts = append(chatOpts, model.WithStop(opts.Stop))
	}
	if len(opts.Tools) > 0 {
		chatOpts = append(chatOpts, model.WithTools(opts.Tools))
	}
	if opts.ToolChoice != nil {
		chatOpts = append(chatOpts, model.WithToolChoice(*opts.ToolChoice, opts.AllowedToolNames...))
	}

	if opts.Thinking != nil && opts.Thinking.Enabled {
		chatOpts = append(chatOpts, openai.WithReasoningEffort(mapThinkingBudgetToOpenAIEffort(opts.Thinking.BudgetTokens)))
	}
	return chatOpts, nil
}

func mapTopKToTopP(topK int) float32 {
	switch {
	case topK <= 0:
		return 1.0
	case topK <= 20:
		return 0.2
	case topK <= 40:
		return 0.35
	case topK <= 100:
		return 0.6
	case topK <= 200:
		return 0.8
	default:
		return 0.95
	}
}

func mapThinkingBudgetToOpenAIEffort(budgetTokens int) openai.ReasoningEffortLevel {
	switch {
	case budgetTokens <= 2048:
		return openai.ReasoningEffortLevelLow
	case budgetTokens <= 8192:
		return openai.ReasoningEffortLevelMedium
	default:
		return openai.ReasoningEffortLevelHigh
	}
}

type streamReaderWrapper struct {
	stream *schema.StreamReader[*schema.Message]
}

func (s *streamReaderWrapper) Recv() (*schema.Message, error) {
	return s.stream.Recv()
}

func (s *streamReaderWrapper) Close() {
	s.stream.Close()
}

func retry[T any](ctx context.Context, cfg *config.Config, fn func() (T, error)) (T, error) {
	var zero T

	maxRetries := cfg.Retry.MaxRetries
	initialDelay := time.Duration(cfg.Retry.InitialBackoffMS) * time.Millisecond
	maxDelay := time.Duration(cfg.Retry.MaxBackoffMS) * time.Millisecond

	attempt := 0
	start := time.Now()
	for {
		result, err := fn()
		if err == nil {
			return result, nil
		}

		elapsed := time.Since(start)

		if attempt >= maxRetries || !isRetryableError(err) {
			if errors.Is(err, context.Canceled) {
				logger.Warnf("Upstream request canceled: elapsed=%v attempt=%d/%d error=%v", elapsed.Round(time.Millisecond), attempt+1, maxRetries+1, err)
			} else if errors.Is(err, context.DeadlineExceeded) {
				logger.Warnf("Upstream request timed out: elapsed=%v attempt=%d/%d error=%v", elapsed.Round(time.Millisecond), attempt+1, maxRetries+1, err)
			} else {
				logger.Warnf("Upstream request failed: elapsed=%v attempt=%d/%d error=%v", elapsed.Round(time.Millisecond), attempt+1, maxRetries+1, err)
			}
			return zero, err
		}
		logger.Warnf("Upstream request failed, retrying: elapsed=%v attempt=%d/%d error=%v", elapsed.Round(time.Millisecond), attempt+1, maxRetries+1, err)

		delay := initialDelay
		if attempt > 0 {
			delay = maxDelay
		}
		if delay > maxDelay {
			delay = maxDelay
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return zero, ctx.Err()
		case <-timer.C:
		}

		attempt++
	}
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	var clientErr *ClientError
	if errors.As(err, &clientErr) {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	statusCode := extractStatusCode(err)
	return statusCode == http.StatusTooManyRequests || statusCode >= 500
}

func extractStatusCode(err error) int {
	var openAIErr *openai.APIError
	if errors.As(err, &openAIErr) {
		return openAIErr.HTTPStatusCode
	}

	match := statusCodePattern.FindStringSubmatch(err.Error())
	if len(match) < 2 {
		return 0
	}
	code, convErr := strconv.Atoi(match[1])
	if convErr != nil {
		return 0
	}
	return code
}
