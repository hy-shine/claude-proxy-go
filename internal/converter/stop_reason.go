package converter

import "strings"

// MapStopReason converts provider finish reasons into Anthropic-compatible stop_reason values.
func MapStopReason(finishReason string, hasToolCalls bool) string {
	switch strings.ToLower(strings.TrimSpace(finishReason)) {
	case "tool_use", "tool_calls":
		return "tool_use"
	case "length", "max_tokens", "max_output_tokens", "max_completion_tokens":
		return "max_tokens"
	case "stop_sequence":
		return "stop_sequence"
	case "", "stop", "end_turn":
		// continue to fallback logic
	default:
		// unknown values should still preserve tool_use when tool calls are present.
	}

	if hasToolCalls {
		return "tool_use"
	}
	return "end_turn"
}

// ResolveStopSequence returns the stop_sequence value only when finish reason
// indicates stop_sequence and the match can be determined safely.
func ResolveStopSequence(finishReason string, configured []string) *string {
	if strings.ToLower(strings.TrimSpace(finishReason)) != "stop_sequence" {
		return nil
	}
	if len(configured) != 1 {
		return nil
	}
	seq := configured[0]
	if seq == "" {
		return nil
	}
	return &seq
}
