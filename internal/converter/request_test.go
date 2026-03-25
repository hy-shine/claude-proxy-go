package converter

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/hy-shine/claude-proxy-go/internal/types"
)

func TestToEinoRequestKeepsThinkingAndTopK(t *testing.T) {
	topK := 10
	req := &types.MessagesRequest{
		Model:     "m1",
		MaxTokens: 32,
		Messages:  []types.Message{{Role: "user", Content: "hello"}},
		TopK:      &topK,
		Thinking:  &types.ThinkingConfig{Enabled: true, BudgetTokens: 128},
	}

	_, opts, err := ToEinoRequest(req)
	if err != nil {
		t.Fatalf("ToEinoRequest() error = %v", err)
	}
	if opts.TopK == nil || *opts.TopK != 10 {
		t.Fatalf("TopK mismatch: %#v", opts.TopK)
	}
	if opts.Thinking == nil || !opts.Thinking.Enabled || opts.Thinking.BudgetTokens != 128 {
		t.Fatalf("Thinking mismatch: %#v", opts.Thinking)
	}
}

func TestToEinoRequestAcceptsThinkingTypeEnabled(t *testing.T) {
	req := &types.MessagesRequest{
		Model:     "m1",
		MaxTokens: 32,
		Messages:  []types.Message{{Role: "user", Content: "hello"}},
		Thinking:  &types.ThinkingConfig{Type: "enabled", BudgetTokens: 2048},
	}

	_, opts, err := ToEinoRequest(req)
	if err != nil {
		t.Fatalf("ToEinoRequest() error = %v", err)
	}
	if opts.Thinking == nil {
		t.Fatalf("Thinking mismatch: %#v", opts.Thinking)
	}
	if !opts.Thinking.Enabled {
		t.Fatalf("expected thinking enabled, got %#v", opts.Thinking)
	}
	if opts.Thinking.BudgetTokens != 2048 {
		t.Fatalf("budget mismatch: %#v", opts.Thinking)
	}
}

func TestToEinoRequestAcceptsThinkingTypeDisabled(t *testing.T) {
	req := &types.MessagesRequest{
		Model:     "m1",
		MaxTokens: 32,
		Messages:  []types.Message{{Role: "user", Content: "hello"}},
		Thinking:  &types.ThinkingConfig{Type: "disabled"},
	}

	_, opts, err := ToEinoRequest(req)
	if err != nil {
		t.Fatalf("ToEinoRequest() error = %v", err)
	}
	if opts.Thinking == nil {
		t.Fatalf("Thinking mismatch: %#v", opts.Thinking)
	}
	if opts.Thinking.Enabled {
		t.Fatalf("expected thinking disabled, got %#v", opts.Thinking)
	}
}

func TestToEinoRequestAcceptsThinkingTypeAdaptive(t *testing.T) {
	req := &types.MessagesRequest{
		Model:     "m1",
		MaxTokens: 32,
		Messages:  []types.Message{{Role: "user", Content: "hello"}},
		Thinking:  &types.ThinkingConfig{Type: "adaptive"},
	}

	_, opts, err := ToEinoRequest(req)
	if err != nil {
		t.Fatalf("ToEinoRequest() error = %v", err)
	}
	if opts.Thinking == nil {
		t.Fatalf("Thinking mismatch: %#v", opts.Thinking)
	}
	if opts.Thinking.Type != "adaptive" {
		t.Fatalf("expected thinking type=adaptive, got %#v", opts.Thinking)
	}
}

func TestToEinoRequestRejectsInvalidThinkingAndTopK(t *testing.T) {
	negTopK := -1

	tests := []struct {
		name string
		req  *types.MessagesRequest
		want string
	}{
		{
			name: "negative top_k",
			req: &types.MessagesRequest{
				Model:     "m1",
				MaxTokens: 32,
				Messages:  []types.Message{{Role: "user", Content: "hello"}},
				TopK:      &negTopK,
			},
			want: "top_k must be >= 0",
		},
		{
			name: "enabled thinking requires positive budget",
			req: &types.MessagesRequest{
				Model:     "m1",
				MaxTokens: 32,
				Messages:  []types.Message{{Role: "user", Content: "hello"}},
				Thinking:  &types.ThinkingConfig{Type: "enabled", BudgetTokens: 0},
			},
			want: "thinking.budget_tokens must be > 0 when thinking.type is enabled",
		},
		{
			name: "invalid thinking type",
			req: &types.MessagesRequest{
				Model:     "m1",
				MaxTokens: 32,
				Messages:  []types.Message{{Role: "user", Content: "hello"}},
				Thinking:  &types.ThinkingConfig{Type: "invalid", BudgetTokens: 512},
			},
			want: "thinking.type must be enabled, disabled, or adaptive",
		},
		{
			name: "thinking enabled conflicts with disabled type",
			req: &types.MessagesRequest{
				Model:     "m1",
				MaxTokens: 32,
				Messages:  []types.Message{{Role: "user", Content: "hello"}},
				Thinking:  &types.ThinkingConfig{Enabled: true, Type: "disabled", BudgetTokens: 512},
			},
			want: "thinking.enabled conflicts with thinking.type=disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ToEinoRequest(tt.req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q in error, got %v", tt.want, err)
			}
		})
	}
}

func TestToEinoRequestToolChoiceMapping(t *testing.T) {
	req := &types.MessagesRequest{
		Model:     "m1",
		MaxTokens: 64,
		Messages:  []types.Message{{Role: "user", Content: "hello"}},
		Tools: []types.Tool{
			{
				Name:        "calc",
				Description: "calculator",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"x":{"type":"number","description":"x"}},"required":["x"]}`),
			},
		},
		ToolChoice: &types.ToolChoice{Type: "tool", Name: "calc"},
	}

	msgs, opts, err := ToEinoRequest(req)
	if err != nil {
		t.Fatalf("ToEinoRequest() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("messages length mismatch: %d", len(msgs))
	}
	if opts.ToolChoice == nil || *opts.ToolChoice != schema.ToolChoiceForced {
		t.Fatalf("unexpected tool choice: %#v", opts.ToolChoice)
	}
	if len(opts.AllowedToolNames) != 1 || opts.AllowedToolNames[0] != "calc" {
		t.Fatalf("allowed tool names mismatch: %#v", opts.AllowedToolNames)
	}
	if len(opts.Tools) != 1 || opts.Tools[0].Name != "calc" {
		t.Fatalf("tools mismatch: %#v", opts.Tools)
	}
}

func TestToEinoRequestConvertsDocumentBlock(t *testing.T) {
	req := &types.MessagesRequest{
		Model:     "m1",
		MaxTokens: 16,
		Messages: []types.Message{
			{
				Role: "user",
				Content: []any{
					map[string]any{
						"type":    "document",
						"title":   "Design Spec",
						"context": "Architecture",
						"source": map[string]any{
							"type": "text",
							"text": "This is the document body.",
						},
					},
				},
			},
		},
	}

	msgs, _, err := ToEinoRequest(req)
	if err != nil {
		t.Fatalf("ToEinoRequest() error = %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("messages length = %d, want 1", len(msgs))
	}
	if msgs[0].Role != schema.User {
		t.Fatalf("role mismatch: %s", msgs[0].Role)
	}
	if !strings.Contains(msgs[0].Content, "Document title: Design Spec") {
		t.Fatalf("missing title in converted content: %q", msgs[0].Content)
	}
	if !strings.Contains(msgs[0].Content, "This is the document body.") {
		t.Fatalf("missing document text in converted content: %q", msgs[0].Content)
	}
}

func TestToEinoRequestConvertsDocumentBase64BlockToReference(t *testing.T) {
	req := &types.MessagesRequest{
		Model:     "m1",
		MaxTokens: 16,
		Messages: []types.Message{
			{
				Role: "user",
				Content: []any{
					map[string]any{
						"type": "document",
						"source": map[string]any{
							"type":       "base64",
							"media_type": "application/pdf",
							"data":       "cGRmLWRhdGE=",
						},
					},
				},
			},
		},
	}

	msgs, _, err := ToEinoRequest(req)
	if err != nil {
		t.Fatalf("ToEinoRequest() error = %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("messages length = %d, want 1", len(msgs))
	}
	if !strings.Contains(msgs[0].Content, "Document attachment") {
		t.Fatalf("missing attachment marker: %q", msgs[0].Content)
	}
	if !strings.Contains(msgs[0].Content, "media_type=application/pdf") {
		t.Fatalf("missing media_type marker: %q", msgs[0].Content)
	}
}

func TestToEinoRequestRejectsDocumentWithoutTextOrSource(t *testing.T) {
	req := &types.MessagesRequest{
		Model:     "m1",
		MaxTokens: 16,
		Messages: []types.Message{
			{
				Role: "user",
				Content: []any{
					map[string]any{"type": "document"},
				},
			},
		},
	}

	_, _, err := ToEinoRequest(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "document block requires text or source object") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToEinoRequestConvertsUserImageBlocksToMultiContent(t *testing.T) {
	req := &types.MessagesRequest{
		Model:     "m1",
		MaxTokens: 32,
		Messages: []types.Message{
			{
				Role: "user",
				Content: []any{
					map[string]any{"type": "text", "text": "Describe this image"},
					map[string]any{
						"type": "image",
						"source": map[string]any{
							"type":       "base64",
							"media_type": "image/png",
							"data":       "ZmFrZQ==",
						},
					},
				},
			},
		},
	}

	msgs, _, err := ToEinoRequest(req)
	if err != nil {
		t.Fatalf("ToEinoRequest() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("messages length = %d, want 1", len(msgs))
	}

	msg := msgs[0]
	if msg.Role != schema.User {
		t.Fatalf("role mismatch: %s", msg.Role)
	}
	if msg.Content != "" {
		t.Fatalf("expected empty string content for multimodal message, got %q", msg.Content)
	}
	if len(msg.UserInputMultiContent) != 2 {
		t.Fatalf("multi content length mismatch: %d", len(msg.UserInputMultiContent))
	}
	if msg.UserInputMultiContent[0].Type != schema.ChatMessagePartTypeText || msg.UserInputMultiContent[0].Text != "Describe this image" {
		t.Fatalf("text part mismatch: %#v", msg.UserInputMultiContent[0])
	}
	imagePart := msg.UserInputMultiContent[1]
	if imagePart.Type != schema.ChatMessagePartTypeImageURL || imagePart.Image == nil {
		t.Fatalf("image part mismatch: %#v", imagePart)
	}
	if imagePart.Image.Base64Data == nil || *imagePart.Image.Base64Data != "ZmFrZQ==" {
		t.Fatalf("image base64 mismatch: %#v", imagePart.Image)
	}
	if imagePart.Image.MIMEType != "image/png" {
		t.Fatalf("image mime mismatch: %q", imagePart.Image.MIMEType)
	}
}

func TestToEinoRequestRejectsImageBlockWithInvalidSource(t *testing.T) {
	req := &types.MessagesRequest{
		Model:     "m1",
		MaxTokens: 16,
		Messages: []types.Message{
			{
				Role: "user",
				Content: []any{
					map[string]any{
						"type": "image",
						"source": map[string]any{
							"type": "base64",
							"data": "ZmFrZQ==",
						},
					},
				},
			},
		},
	}

	_, _, err := ToEinoRequest(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "image base64 source requires media_type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToEinoRequestConvertsToolResultToToolMessage(t *testing.T) {
	req := &types.MessagesRequest{
		Model:     "m1",
		MaxTokens: 16,
		Messages: []types.Message{
			{
				Role: "user",
				Content: []any{
					map[string]any{"type": "text", "text": "Hello"},
					map[string]any{
						"type":        "tool_result",
						"tool_use_id": "call_1",
						"content": []any{
							map[string]any{"type": "text", "text": "World"},
						},
					},
					map[string]any{"type": "text", "text": "!"},
				},
			},
		},
	}

	msgs, _, err := ToEinoRequest(req)
	if err != nil {
		t.Fatalf("ToEinoRequest() error = %v", err)
	}

	if len(msgs) != 3 {
		t.Fatalf("messages length = %d, want 3", len(msgs))
	}

	if msgs[0].Role != schema.User || msgs[0].Content != "Hello" {
		t.Fatalf("msg[0] mismatch: %#v", msgs[0])
	}
	if msgs[1].Role != schema.Tool || msgs[1].ToolCallID != "call_1" || msgs[1].Content != "World" {
		t.Fatalf("msg[1] mismatch: %#v", msgs[1])
	}
	if msgs[2].Role != schema.User || msgs[2].Content != "!" {
		t.Fatalf("msg[2] mismatch: %#v", msgs[2])
	}
}

func TestToEinoRequestRejectsToolChoiceWithoutTools(t *testing.T) {
	req := &types.MessagesRequest{
		Model:      "m1",
		MaxTokens:  16,
		Messages:   []types.Message{{Role: "user", Content: "hello"}},
		ToolChoice: &types.ToolChoice{Type: "auto"},
	}

	_, _, err := ToEinoRequest(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "tool_choice requires tools") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToEinoRequestPreservesToolSchemaTypes(t *testing.T) {
	req := &types.MessagesRequest{
		Model:     "m1",
		MaxTokens: 32,
		Messages:  []types.Message{{Role: "user", Content: "hello"}},
		Tools: []types.Tool{
			{
				Name: "typed_tool",
				InputSchema: json.RawMessage(`{
					"type":"object",
					"properties":{
						"count":{"type":"integer","description":"counter"},
						"ratio":{"type":"number"},
						"ok":{"type":"boolean"},
						"tags":{"type":"array","items":{"type":"string"}},
						"meta":{
							"type":"object",
							"properties":{"x":{"type":"string"}},
							"required":["x"]
						}
					},
					"required":["count","meta"]
				}`),
			},
		},
	}

	_, opts, err := ToEinoRequest(req)
	if err != nil {
		t.Fatalf("ToEinoRequest() error = %v", err)
	}

	if len(opts.Tools) != 1 {
		t.Fatalf("tools length mismatch: %d", len(opts.Tools))
	}

	s, err := opts.Tools[0].ParamsOneOf.ToJSONSchema()
	if err != nil {
		t.Fatalf("ToJSONSchema() error = %v", err)
	}
	if s == nil {
		t.Fatal("expected schema, got nil")
	}

	if s.Type != "object" {
		t.Fatalf("schema type = %q, want object", s.Type)
	}

	count, ok := s.Properties.Get("count")
	if !ok || count == nil || count.Type != "integer" {
		t.Fatalf("count type mismatch: %#v", count)
	}

	ratio, ok := s.Properties.Get("ratio")
	if !ok || ratio == nil || ratio.Type != "number" {
		t.Fatalf("ratio type mismatch: %#v", ratio)
	}

	okProp, ok := s.Properties.Get("ok")
	if !ok || okProp == nil || okProp.Type != "boolean" {
		t.Fatalf("ok type mismatch: %#v", okProp)
	}

	tags, ok := s.Properties.Get("tags")
	if !ok || tags == nil || tags.Type != "array" || tags.Items == nil || tags.Items.Type != "string" {
		t.Fatalf("tags schema mismatch: %#v", tags)
	}

	meta, ok := s.Properties.Get("meta")
	if !ok || meta == nil || meta.Type != "object" {
		t.Fatalf("meta schema mismatch: %#v", meta)
	}
	x, ok := meta.Properties.Get("x")
	if !ok || x == nil || x.Type != "string" {
		t.Fatalf("meta.x schema mismatch: %#v", x)
	}
	if !contains(meta.Required, "x") {
		t.Fatalf("meta required missing x: %#v", meta.Required)
	}
	if !contains(s.Required, "count") || !contains(s.Required, "meta") {
		t.Fatalf("root required mismatch: %#v", s.Required)
	}
}

func TestToEinoRequestConvertsAssistantToolUseBlock(t *testing.T) {
	req := &types.MessagesRequest{
		Model:     "m1",
		MaxTokens: 32,
		Messages: []types.Message{
			{
				Role: "assistant",
				Content: []any{
					map[string]any{"type": "text", "text": "Let me call a tool."},
					map[string]any{
						"type":  "tool_use",
						"id":    "tool_1",
						"name":  "search",
						"input": map[string]any{"q": "golang"},
					},
				},
			},
		},
	}

	msgs, _, err := ToEinoRequest(req)
	if err != nil {
		t.Fatalf("ToEinoRequest() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("messages length = %d, want 1", len(msgs))
	}

	msg := msgs[0]
	if msg.Role != schema.Assistant {
		t.Fatalf("role mismatch: %s", msg.Role)
	}
	if msg.Content != "Let me call a tool." {
		t.Fatalf("content mismatch: %q", msg.Content)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("tool_calls length mismatch: %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].ID != "tool_1" || msg.ToolCalls[0].Function.Name != "search" {
		t.Fatalf("tool call mismatch: %#v", msg.ToolCalls[0])
	}
	if msg.ToolCalls[0].Function.Arguments != `{"q":"golang"}` {
		t.Fatalf("tool arguments mismatch: %q", msg.ToolCalls[0].Function.Arguments)
	}
}

func TestToEinoRequestInfersAssistantToolUseNameFromID(t *testing.T) {
	req := &types.MessagesRequest{
		Model:     "m1",
		MaxTokens: 32,
		Messages: []types.Message{
			{
				Role: "assistant",
				Content: []any{
					map[string]any{
						"type":  "tool_use",
						"id":    "tool_1",
						"input": map[string]any{"q": "golang"},
					},
				},
			},
		},
	}

	msgs, _, err := ToEinoRequest(req)
	if err != nil {
		t.Fatalf("ToEinoRequest() error = %v", err)
	}
	if len(msgs) != 1 || len(msgs[0].ToolCalls) != 1 {
		t.Fatalf("unexpected tool calls: %#v", msgs)
	}
	if got := msgs[0].ToolCalls[0].Function.Name; got != "tool_1" {
		t.Fatalf("inferred tool name = %q, want %q", got, "tool_1")
	}
}

func TestToEinoRequestInfersAssistantToolUseNameFromSingleConfiguredTool(t *testing.T) {
	req := &types.MessagesRequest{
		Model:     "m1",
		MaxTokens: 32,
		Messages: []types.Message{
			{
				Role: "assistant",
				Content: []any{
					map[string]any{
						"type":  "tool_use",
						"id":    "call_abc",
						"input": map[string]any{"pattern": "**/*"},
					},
				},
			},
		},
		Tools: []types.Tool{
			{
				Name:        "glob_search",
				Description: "search files",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string"}}}`),
			},
		},
	}

	msgs, _, err := ToEinoRequest(req)
	if err != nil {
		t.Fatalf("ToEinoRequest() error = %v", err)
	}
	if len(msgs) != 1 || len(msgs[0].ToolCalls) != 1 {
		t.Fatalf("unexpected tool calls: %#v", msgs)
	}
	if got := msgs[0].ToolCalls[0].Function.Name; got != "glob_search" {
		t.Fatalf("inferred tool name = %q, want %q", got, "glob_search")
	}
}

func TestToEinoRequestSkipsThinkingBlocksInAssistantHistory(t *testing.T) {
	req := &types.MessagesRequest{
		Model:     "m1",
		MaxTokens: 100,
		Messages: []types.Message{
			{
				Role: "user",
				Content: []any{
					map[string]any{"type": "text", "text": "What is 2+2?"},
				},
			},
			{
				Role: "assistant",
				Content: []any{
					map[string]any{"type": "thinking", "thinking": "Let me calculate..."},
					map[string]any{"type": "text", "text": "4"},
				},
			},
			{
				Role: "user",
				Content: []any{
					map[string]any{"type": "text", "text": "What about 3+3?"},
				},
			},
		},
	}

	msgs, _, err := ToEinoRequest(req)
	if err != nil {
		t.Fatalf("ToEinoRequest() error = %v", err)
	}
	// Should have: user, assistant (thinking skipped), user
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d: %#v", len(msgs), msgs)
	}
	if msgs[1].Role != schema.Assistant {
		t.Fatalf("msg[1] role = %q, want assistant", msgs[1].Role)
	}
	if msgs[1].Content != "4" {
		t.Fatalf("msg[1] content = %q, want %q", msgs[1].Content, "4")
	}
}

func TestToEinoRequestSkipsRedactedThinkingBlocks(t *testing.T) {
	req := &types.MessagesRequest{
		Model:     "m1",
		MaxTokens: 100,
		Messages: []types.Message{
			{
				Role: "assistant",
				Content: []any{
					map[string]any{"type": "redacted_thinking", "data": "encrypted..."},
					map[string]any{"type": "text", "text": "Here is my answer."},
				},
			},
		},
	}

	msgs, _, err := ToEinoRequest(req)
	if err != nil {
		t.Fatalf("ToEinoRequest() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != "Here is my answer." {
		t.Fatalf("content = %q, want %q", msgs[0].Content, "Here is my answer.")
	}
}

func contains(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
