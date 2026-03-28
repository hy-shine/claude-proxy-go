package token

import (
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/pkoukk/tiktoken-go"
)

// Counter provides token counting functionality.
type Counter struct {
	defaultEncoding string
}

// NewCounter creates a new token counter.
func NewCounter() *Counter {
	return &Counter{
		defaultEncoding: "cl100k_base", // GPT-4/GPT-3.5-turbo encoding
	}
}

// CountMessages counts tokens for a list of messages.
// This is an approximation based on OpenAI's token counting approach.
func (c *Counter) CountMessages(messages []*schema.Message, model string) int {
	tkm, err := tiktoken.EncodingForModel(model)
	if err != nil {
		// Fallback to default encoding
		tkm, _ = tiktoken.GetEncoding(c.defaultEncoding)
	}

	// Tokens per message varies by model
	// See: https://github.com/openai/openai-cookbook/blob/main/examples/How_to_count_tokens_with_tiktoken.ipynb
	tokensPerMessage := 3 // For most modern models
	tokensPerName := 1

	total := 0
	for _, msg := range messages {
		total += tokensPerMessage
		total += len(tkm.Encode(string(msg.Role), nil, nil))
		total += len(tkm.Encode(msg.Content, nil, nil))

		// Count name/tool-related fields if present
		if msg.Name != "" {
			total += len(tkm.Encode(msg.Name, nil, nil))
			total += tokensPerName
		}

		// Count tool calls
		for _, tc := range msg.ToolCalls {
			total += len(tkm.Encode(tc.ID, nil, nil))
			total += len(tkm.Encode(tc.Type, nil, nil))
			total += len(tkm.Encode(tc.Function.Name, nil, nil))
			total += len(tkm.Encode(tc.Function.Arguments, nil, nil))
		}
	}

	// Every reply is primed with <|start|>assistant<|message|>
	total += 3

	return total
}

// CountText counts tokens in a plain text string.
func (c *Counter) CountText(text, model string) int {
	tkm, err := tiktoken.EncodingForModel(model)
	if err != nil {
		tkm, _ = tiktoken.GetEncoding(c.defaultEncoding)
	}
	return len(tkm.Encode(text, nil, nil))
}

// CountSystemPrompt counts tokens in a system prompt.
func (c *Counter) CountSystemPrompt(system any, model string) int {
	if system == nil {
		return 0
	}

	var text string
	switch v := system.(type) {
	case string:
		text = v
	case []any:
		var sb strings.Builder
		for _, item := range v {
			if block, ok := item.(map[string]any); ok {
				if block["type"] == "text" {
					if t, ok := block["text"].(string); ok {
						sb.WriteString(t)
						sb.WriteString("\n\n")
					}
				}
			}
		}
		text = strings.TrimSpace(sb.String())
	default:
		return 0
	}

	return c.CountText(text, model)
}

// CountTools counts tokens for tool definitions.
func (c *Counter) CountTools(tools []any, model string) int {
	if len(tools) == 0 {
		return 0
	}

	tkm, err := tiktoken.EncodingForModel(model)
	if err != nil {
		tkm, _ = tiktoken.GetEncoding(c.defaultEncoding)
	}

	total := 0
	for _, tool := range tools {
		// Rough estimation: each tool contributes its name + description + schema
		if t, ok := tool.(map[string]any); ok {
			if name, ok := t["name"].(string); ok {
				total += len(tkm.Encode(name, nil, nil))
			}
			if desc, ok := t["description"].(string); ok {
				total += len(tkm.Encode(desc, nil, nil))
			}
			if schema, ok := t["input_schema"].(map[string]any); ok {
				total += countSchemaTokens(schema, tkm)
			}
		}
	}

	return total
}

func countSchemaTokens(schema map[string]any, tkm *tiktoken.Tiktoken) int {
	// Simple estimation based on schema complexity
	total := 0

	if props, ok := schema["properties"].(map[string]any); ok {
		for name, prop := range props {
			total += len(tkm.Encode(name, nil, nil))
			if propMap, ok := prop.(map[string]any); ok {
				if t, ok := propMap["type"].(string); ok {
					total += len(tkm.Encode(t, nil, nil))
				}
				if desc, ok := propMap["description"].(string); ok {
					total += len(tkm.Encode(desc, nil, nil))
				}
			}
		}
	}

	if req, ok := schema["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				total += len(tkm.Encode(s, nil, nil))
			}
		}
	}

	return total
}
