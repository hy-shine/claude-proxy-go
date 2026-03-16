package stream

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/1rgs/claude-code-proxy-go/internal/converter"
	"github.com/cloudwego/eino/schema"
)

type MessageStream interface {
	Recv() (*schema.Message, error)
	Close()
}

type SSEHandler struct {
	model         string
	stopSequences []string
}

func NewSSEHandler(model string, stopSequences []string) *SSEHandler {
	return &SSEHandler{
		model:         model,
		stopSequences: stopSequences,
	}
}

func (h *SSEHandler) StreamToClient(stream MessageStream, w http.ResponseWriter) error {
	writer := io.Writer(w)
	flusher, _ := w.(http.Flusher)

	messageID := generateMessageID()
	inputTokens := 0
	outputTokens := 0
	toolUsed := false
	finishReason := ""
	textBlockOpen := true
	activeTextIndex := 0
	nextContentIndex := 1

	if err := h.sendEvent(writer, flusher, "message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":          messageID,
			"type":        "message",
			"role":        "assistant",
			"model":       h.model,
			"content":     []any{},
			"stop_reason": nil,
			"usage":       map[string]int{"input_tokens": 0, "output_tokens": 0},
		},
	}); err != nil {
		return err
	}

	if err := h.sendEvent(writer, flusher, "content_block_start", map[string]any{
		"type":          "content_block_start",
		"index":         0,
		"content_block": map[string]any{"type": "text", "text": ""},
	}); err != nil {
		return err
	}

	if err := h.sendEvent(writer, flusher, "ping", map[string]any{"type": "ping"}); err != nil {
		return err
	}

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if sendErr := h.sendErrorEvent(writer, flusher, err); sendErr != nil {
				return sendErr
			}
			return nil
		}

		if chunk.ResponseMeta != nil && chunk.ResponseMeta.Usage != nil {
			if chunk.ResponseMeta.Usage.PromptTokens > 0 {
				inputTokens = chunk.ResponseMeta.Usage.PromptTokens
			}
			if chunk.ResponseMeta.Usage.CompletionTokens > 0 {
				outputTokens = chunk.ResponseMeta.Usage.CompletionTokens
			}
		}
		if chunk.ResponseMeta != nil && chunk.ResponseMeta.FinishReason != "" {
			finishReason = chunk.ResponseMeta.FinishReason
		}

		if chunk.Content != "" {
			if !textBlockOpen {
				activeTextIndex = nextContentIndex
				nextContentIndex++
				if err := h.sendEvent(writer, flusher, "content_block_start", map[string]any{
					"type":          "content_block_start",
					"index":         activeTextIndex,
					"content_block": map[string]any{"type": "text", "text": ""},
				}); err != nil {
					return err
				}
				textBlockOpen = true
			}

			if err := h.sendEvent(writer, flusher, "content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": activeTextIndex,
				"delta": map[string]any{"type": "text_delta", "text": chunk.Content},
			}); err != nil {
				return err
			}
		}

		if len(chunk.ToolCalls) > 0 {
			toolUsed = true
			if textBlockOpen {
				if err := h.sendEvent(writer, flusher, "content_block_stop", map[string]any{
					"type":  "content_block_stop",
					"index": activeTextIndex,
				}); err != nil {
					return err
				}
				textBlockOpen = false
			}

			for _, tc := range chunk.ToolCalls {
				if err := h.sendEvent(writer, flusher, "content_block_start", map[string]any{
					"type":  "content_block_start",
					"index": nextContentIndex,
					"content_block": map[string]any{
						"type":  "tool_use",
						"id":    tc.ID,
						"name":  tc.Function.Name,
						"input": map[string]any{},
					},
				}); err != nil {
					return err
				}

				if err := h.sendEvent(writer, flusher, "content_block_delta", map[string]any{
					"type":  "content_block_delta",
					"index": nextContentIndex,
					"delta": map[string]any{
						"type":         "input_json_delta",
						"partial_json": tc.Function.Arguments,
					},
				}); err != nil {
					return err
				}

				if err := h.sendEvent(writer, flusher, "content_block_stop", map[string]any{
					"type":  "content_block_stop",
					"index": nextContentIndex,
				}); err != nil {
					return err
				}
				nextContentIndex++
			}
		}
	}

	if textBlockOpen {
		if err := h.sendEvent(writer, flusher, "content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": activeTextIndex,
		}); err != nil {
			return err
		}
	}

	stopReason := converter.MapStopReason(finishReason, toolUsed)
	stopSequence := converter.ResolveStopSequence(finishReason, h.stopSequences)
	stopSequenceValue := any(nil)
	if stopSequence != nil {
		stopSequenceValue = *stopSequence
	}

	if err := h.sendEvent(writer, flusher, "message_delta", map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   stopReason,
			"stop_sequence": stopSequenceValue,
		},
		"usage": map[string]int{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
		},
	}); err != nil {
		return err
	}

	if err := h.sendEvent(writer, flusher, "message_stop", map[string]any{"type": "message_stop"}); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(writer, "data: [DONE]\n\n"); err != nil {
		return err
	}
	if flusher != nil {
		flusher.Flush()
	}

	return nil
}

func (h *SSEHandler) sendErrorEvent(w io.Writer, flusher http.Flusher, sourceErr error) error {
	return h.sendEvent(w, flusher, "error", map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    "api_error",
			"message": sourceErr.Error(),
		},
	})
}

func (h *SSEHandler) sendEvent(w io.Writer, flusher http.Flusher, eventType string, data map[string]any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "event: %s\n", eventType); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", jsonData); err != nil {
		return err
	}
	if flusher != nil {
		flusher.Flush()
	}
	return nil
}

func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}
