package converter

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/1rgs/claude-code-proxy-go/internal/types"
	"github.com/cloudwego/eino/schema"
)

func FromEinoResponse(resp *schema.Message, originalModel string, requestedStopSequences []string) *types.MessagesResponse {
	content := convertContent(resp.Content, resp.ToolCalls)

	stopReason := "end_turn"
	var stopSequence *string
	if resp != nil {
		finishReason := ""
		if resp.ResponseMeta != nil {
			finishReason = resp.ResponseMeta.FinishReason
		}
		stopReason = MapStopReason(finishReason, len(resp.ToolCalls) > 0)
		stopSequence = ResolveStopSequence(finishReason, requestedStopSequences)
	}

	return &types.MessagesResponse{
		ID:           generateMessageID(),
		Type:         "message",
		Role:         "assistant",
		Content:      content,
		Model:        originalModel,
		StopReason:   stopReason,
		StopSequence: stopSequence,
		Usage: types.Usage{
			InputTokens:  getUsage(resp, "input"),
			OutputTokens: getUsage(resp, "output"),
		},
	}
}

func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

func convertContent(text string, toolCalls []schema.ToolCall) []types.ContentBlock {
	var content []types.ContentBlock

	if text != "" {
		content = append(content, types.ContentBlock{
			Type: "text",
			Text: text,
		})
	}

	for _, tc := range toolCalls {
		block := types.ContentBlock{
			Type: "tool_use",
			ID:   tc.ID,
			Name: tc.Function.Name,
		}

		block.Input = parseToolArguments(tc.Function.Arguments)

		content = append(content, block)
	}

	if len(content) == 0 {
		content = append(content, types.ContentBlock{
			Type: "text",
			Text: "",
		})
	}

	return content
}

func parseToolArguments(raw string) any {
	if raw == "" {
		return map[string]any{}
	}

	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return raw
	}
	return parsed
}

func getUsage(resp *schema.Message, direction string) int {
	if resp.ResponseMeta == nil || resp.ResponseMeta.Usage == nil {
		return 0
	}

	usage := resp.ResponseMeta.Usage

	switch direction {
	case "input":
		return usage.PromptTokens
	case "output":
		return usage.CompletionTokens
	default:
		return 0
	}
}

func ParseToolResultContent(content any) string {
	if content == nil {
		return ""
	}

	switch v := content.(type) {
	case string:
		return v
	case []any:
		var sb strings.Builder
		containsOnlyText := true
		for _, item := range v {
			if block, ok := item.(map[string]any); ok {
				if block["type"] == "text" {
					if text, ok := block["text"].(string); ok {
						sb.WriteString(text)
						sb.WriteString("\n")
						continue
					}
				}
			}
			containsOnlyText = false
		}
		parsedText := strings.TrimSpace(sb.String())
		if parsedText != "" && containsOnlyText {
			return parsedText
		}
		if parsedText != "" && !containsOnlyText {
			// Prefer preserving the full payload when mixed block types exist.
			return marshalCompactJSON(v)
		}
		return marshalCompactJSON(v)
	case map[string]any:
		if block, ok := v["type"].(string); ok && block == "text" {
			if text, ok := v["text"].(string); ok {
				return text
			}
		}
		return marshalCompactJSON(v)
	default:
		return marshalCompactJSON(v)
	}
}

func marshalCompactJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}
