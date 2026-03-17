package eino

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/hy-shine/claude-code-proxy-go/internal/config"
	"github.com/hy-shine/claude-code-proxy-go/internal/converter"
	"github.com/hy-shine/claude-code-proxy-go/internal/types"
)

type fakeChatModel struct{}

func (f *fakeChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage("ok", nil), nil
}

func (f *fakeChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	reader, writer := schema.Pipe[*schema.Message](1)
	writer.Close()
	return reader, nil
}

func (f *fakeChatModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return f, nil
}

func TestPrepareCallMapsTopKToTopPForOpenAI(t *testing.T) {
	cfg := mustBuildConfig(t, "m1")
	c := &Client{
		cfg:    cfg,
		models: map[string]model.ToolCallingChatModel{"m1": &fakeChatModel{}},
	}

	topK := 40
	_, opts, err := c.prepareCall("m1", &converter.ChatOptions{
		TopK: &topK,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	common := model.GetCommonOptions(nil, opts...)
	if common.TopP == nil {
		t.Fatalf("expected mapped top_p, got nil")
	}
	if got := *common.TopP; got != 0.35 {
		t.Fatalf("mapped top_p = %v, want 0.35", got)
	}
}

func TestConvertOptionsIgnoresTopKWhenTopPProvided(t *testing.T) {
	topK := 100
	topP := 0.77
	opts, err := convertOptions(&converter.ChatOptions{
		TopK: &topK,
		TopP: &topP,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	common := model.GetCommonOptions(nil, opts...)
	if common.TopP == nil {
		t.Fatalf("expected top_p, got nil")
	}
	if got := *common.TopP; got != 0.77 {
		t.Fatalf("top_p = %v, want 0.77", got)
	}
}

func TestConvertOptionsSupportsThinkingForOpenAI(t *testing.T) {
	got, err := convertOptions(&converter.ChatOptions{
		Thinking: &types.ThinkingConfig{Enabled: true, BudgetTokens: 1024},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("options length = %d, want 1", len(got))
	}
}

func TestMapThinkingBudgetToOpenAIEffort(t *testing.T) {
	tests := []struct {
		name   string
		budget int
		want   openai.ReasoningEffortLevel
	}{
		{name: "low tier", budget: 1024, want: openai.ReasoningEffortLevelLow},
		{name: "medium tier", budget: 4096, want: openai.ReasoningEffortLevelMedium},
		{name: "high tier", budget: 20000, want: openai.ReasoningEffortLevelHigh},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapThinkingBudgetToOpenAIEffort(tt.budget)
			if got != tt.want {
				t.Fatalf("mapThinkingBudgetToOpenAIEffort(%d) = %q, want %q", tt.budget, got, tt.want)
			}
		})
	}
}

func TestMapTopKToTopP(t *testing.T) {
	tests := []struct {
		name string
		topK int
		want float32
	}{
		{name: "zero", topK: 0, want: 1.0},
		{name: "small", topK: 20, want: 0.2},
		{name: "medium", topK: 80, want: 0.6},
		{name: "large", topK: 300, want: 0.95},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapTopKToTopP(tt.topK)
			if got != tt.want {
				t.Fatalf("mapTopKToTopP(%d) = %v, want %v", tt.topK, got, tt.want)
			}
		})
	}
}

func TestBuildHTTPClientWithProxySupportsHTTP(t *testing.T) {
	client, err := buildHTTPClientWithProxy("http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("buildHTTPClientWithProxy() error = %v", err)
	}
	if client == nil {
		t.Fatal("expected client, got nil")
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport == nil {
		t.Fatalf("unexpected transport: %#v", client.Transport)
	}
	proxyURL, err := transport.Proxy(&http.Request{URL: &url.URL{Scheme: "https", Host: "example.com"}})
	if err != nil {
		t.Fatalf("proxy func error = %v", err)
	}
	if proxyURL == nil || proxyURL.String() != "http://127.0.0.1:8080" {
		t.Fatalf("proxy mismatch: %#v", proxyURL)
	}
}

func TestBuildHTTPClientWithProxySupportsSocksAlias(t *testing.T) {
	client, err := buildHTTPClientWithProxy("socks://127.0.0.1:1080")
	if err != nil {
		t.Fatalf("buildHTTPClientWithProxy() error = %v", err)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport == nil {
		t.Fatalf("unexpected transport: %#v", client.Transport)
	}
	proxyURL, err := transport.Proxy(&http.Request{URL: &url.URL{Scheme: "https", Host: "example.com"}})
	if err != nil {
		t.Fatalf("proxy func error = %v", err)
	}
	if proxyURL == nil || proxyURL.Scheme != "socks5" {
		t.Fatalf("proxy mismatch: %#v", proxyURL)
	}
}

func TestBuildHTTPClientWithProxyRejectsInvalidProxy(t *testing.T) {
	_, err := buildHTTPClientWithProxy("://bad")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func mustBuildConfig(t *testing.T, modelID string) *config.Config {
	t.Helper()

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"openai": {
				Models: map[string]config.ModelConfig{
					modelID: {Name: modelID},
				},
			},
		},
	}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	return cfg
}
