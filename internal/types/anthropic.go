package types

import (
	"encoding/json"
	"fmt"
	"strings"
)

type MessagesRequest struct {
	Type          string          `json:"type,omitempty"`
	Model         string          `json:"model" validate:"required"`
	MaxTokens     int             `json:"max_tokens" validate:"required"`
	Messages      []Message       `json:"messages" validate:"required"`
	System        any             `json:"system,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Stream        *bool           `json:"stream,omitempty"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	TopK          *int            `json:"top_k,omitempty"`
	Metadata      map[string]any  `json:"metadata,omitempty"`
	Tools         []Tool          `json:"tools,omitempty"`
	ToolChoice    *ToolChoice     `json:"tool_choice,omitempty"`
	Thinking      *ThinkingConfig `json:"thinking,omitempty"`
	OutputConfig  *OutputConfig   `json:"output_config,omitempty"`

	OriginalModel string `json:"-"`
}

type ThinkingConfig struct {
	Type         string `json:"type,omitempty"`
	Enabled      bool   `json:"enabled"`
	BudgetTokens int    `json:"budget_tokens"`
	Display      string `json:"display,omitempty"`
}

type OutputConfig struct {
	Effort string `json:"effort,omitempty"`
}

type Message struct {
	Role    string `json:"role" validate:"required"`
	Content any    `json:"content" validate:"required"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`

	Source *ContentBlockSource `json:"source,omitempty"`

	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`

	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   any    `json:"content,omitempty"`

	Thinking  string `json:"thinking,omitempty"`
	Signature string `json:"signature,omitempty"`
}

type ContentBlockSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type Tool struct {
	Type        string          `json:"type,omitempty"`
	Name        string          `json:"name" validate:"required"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema" validate:"required"`
}

type ToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	ThinkingTokens           int `json:"thinking_tokens,omitempty"`
}

type MessagesResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason,omitempty"`
	StopSequence *string        `json:"stop_sequence,omitempty"`
	Usage        Usage          `json:"usage"`
}

type ErrorResponse struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func CleanModelName(model string) string {
	clean := model
	if strings.HasPrefix(clean, "anthropic/") {
		clean = clean[10:]
	} else if strings.HasPrefix(clean, "openai/") {
		clean = clean[7:]
	} else if strings.HasPrefix(clean, "gemini/") {
		clean = clean[7:]
	}
	return clean
}

func (m *Message) GetContentAsString() string {
	if m == nil {
		return ""
	}

	switch v := m.Content.(type) {
	case string:
		return v
	case []any:
		var sb strings.Builder
		for _, item := range v {
			switch block := item.(type) {
			case map[string]any:
				if block["type"] == "text" {
					if text, ok := block["text"].(string); ok {
						sb.WriteString(text)
					}
				}
			}
		}
		return sb.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}
