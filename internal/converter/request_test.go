package converter

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/1rgs/claude-code-proxy-go/internal/types"
	"github.com/cloudwego/eino/schema"
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
				Thinking:  &types.ThinkingConfig{Enabled: true, BudgetTokens: 0},
			},
			want: "thinking.budget_tokens must be > 0",
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

func TestToEinoRequestRejectsUnsupportedContentBlock(t *testing.T) {
	req := &types.MessagesRequest{
		Model:     "m1",
		MaxTokens: 16,
		Messages: []types.Message{
			{
				Role: "user",
				Content: []any{
					map[string]any{"type": "document", "source": "x"},
				},
			},
		},
	}

	_, _, err := ToEinoRequest(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported content block type") {
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

func contains(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
