package token

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestCountMessages(t *testing.T) {
	counter := NewCounter()

	messages := []*schema.Message{
		schema.UserMessage("Hello, world!"),
		schema.AssistantMessage("Hi there!", nil),
		schema.UserMessage("How are you?"),
	}

	count := counter.CountMessages(messages, "gpt-4")
	if count <= 0 {
		t.Errorf("expected positive token count, got %d", count)
	}

	// Should have some tokens for message overhead
	if count < 10 {
		t.Errorf("expected at least 10 tokens for messages with overhead, got %d", count)
	}
}

func TestCountText(t *testing.T) {
	counter := NewCounter()

	tests := []struct {
		text     string
		minCount int
		maxCount int
	}{
		{"Hello", 1, 5},
		{"Hello, world!", 2, 10},
		{"This is a longer sentence with more words.", 5, 20},
	}

	for _, tt := range tests {
		count := counter.CountText(tt.text, "gpt-4")
		if count < tt.minCount || count > tt.maxCount {
			t.Errorf("CountText(%q) = %d, want between %d and %d", tt.text, count, tt.minCount, tt.maxCount)
		}
	}
}

func TestCountSystemPrompt(t *testing.T) {
	counter := NewCounter()

	// String system prompt
	count := counter.CountSystemPrompt("You are a helpful assistant.", "gpt-4")
	if count <= 0 {
		t.Errorf("expected positive token count for string system, got %d", count)
	}

	// List system prompt
	listSystem := []any{
		map[string]any{"type": "text", "text": "You are helpful."},
		map[string]any{"type": "text", "text": "Be concise."},
	}
	count = counter.CountSystemPrompt(listSystem, "gpt-4")
	if count <= 0 {
		t.Errorf("expected positive token count for list system, got %d", count)
	}

	// Nil system prompt
	count = counter.CountSystemPrompt(nil, "gpt-4")
	if count != 0 {
		t.Errorf("expected 0 for nil system, got %d", count)
	}
}

func TestCountTools(t *testing.T) {
	counter := NewCounter()

	tools := []any{
		map[string]any{
			"name":        "get_weather",
			"description": "Get the current weather",
			"input_schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"location": map[string]any{
						"type":        "string",
						"description": "City name",
					},
				},
				"required": []any{"location"},
			},
		},
	}

	count := counter.CountTools(tools, "gpt-4")
	if count <= 0 {
		t.Errorf("expected positive token count for tools, got %d", count)
	}

	// Empty tools
	count = counter.CountTools(nil, "gpt-4")
	if count != 0 {
		t.Errorf("expected 0 for empty tools, got %d", count)
	}
}

func TestCountMessagesWithToolCalls(t *testing.T) {
	counter := NewCounter()

	messages := []*schema.Message{
		{
			Role:    schema.Assistant,
			Content: "",
			ToolCalls: []schema.ToolCall{
				{
					ID:   "call_123",
					Type: "function",
					Function: schema.FunctionCall{
						Name:      "get_weather",
						Arguments: `{"location": "San Francisco"}`,
					},
				},
			},
		},
	}

	count := counter.CountMessages(messages, "gpt-4")
	if count <= 0 {
		t.Errorf("expected positive token count for tool calls, got %d", count)
	}
}
