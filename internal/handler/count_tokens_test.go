package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hy-shine/claude-proxy-go/internal/config"
	"github.com/hy-shine/claude-proxy-go/internal/types"
)

func TestHandleCountTokens(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"openai": {
				APIKey: "test-key",
				Models: map[string]config.ModelConfig{
					"test-model": {Name: "gpt-4"},
				},
			},
		},
	}
	cfg.ApplyDefaults()

	h, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	tests := []struct {
		name       string
		body       CountTokensRequest
		wantStatus int
	}{
		{
			name: "simple text messages",
			body: CountTokensRequest{
				Model: "test-model",
				Messages: []types.Message{
					{Role: "user", Content: "Hello, world!"},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "messages with system",
			body: CountTokensRequest{
				Model:  "test-model",
				System: "You are a helpful assistant.",
				Messages: []types.Message{
					{Role: "user", Content: "Hi!"},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "missing model",
			body: CountTokensRequest{
				Messages: []types.Message{
					{Role: "user", Content: "Hello"},
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "empty messages",
			body: CountTokensRequest{
				Model:    "test-model",
				Messages: []types.Message{},
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			h.HandleCountTokens(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("HandleCountTokens() status = %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				var resp CountTokensResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Errorf("Failed to parse response: %v", err)
				}
				if resp.InputTokens <= 0 {
					t.Errorf("Expected positive token count, got %d", resp.InputTokens)
				}
			}
		})
	}
}

func TestHandleCountTokensMethodNotAllowed(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"openai": {
				APIKey: "test-key",
				Models: map[string]config.ModelConfig{
					"test-model": {Name: "gpt-4"},
				},
			},
		},
	}
	cfg.ApplyDefaults()

	h, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/messages/count_tokens", nil)
	w := httptest.NewRecorder()
	h.HandleCountTokens(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("HandleCountTokens() status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestDetectMultimodalContent(t *testing.T) {
	tests := []struct {
		name     string
		messages []types.Message
		want     bool
	}{
		{
			name: "text only",
			messages: []types.Message{
				{Role: "user", Content: "Hello"},
			},
			want: false,
		},
		{
			name: "with image",
			messages: []types.Message{
				{Role: "user", Content: []any{
					map[string]any{"type": "text", "text": "Look at this"},
					map[string]any{"type": "image", "source": map[string]any{"type": "base64"}},
				}},
			},
			want: true,
		},
		{
			name: "with document",
			messages: []types.Message{
				{Role: "user", Content: []any{
					map[string]any{"type": "document", "text": "PDF content"},
				}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectMultimodalContent(tt.messages)
			if got != tt.want {
				t.Errorf("detectMultimodalContent() = %v, want %v", got, tt.want)
			}
		})
	}
}
