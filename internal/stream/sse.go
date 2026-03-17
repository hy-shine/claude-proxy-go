package stream

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/hy-shine/claude-code-proxy-go/internal/converter"
	"github.com/hy-shine/claude-code-proxy-go/internal/logger"
)

type MessageStream interface {
	Recv() (*schema.Message, error)
	Close()
}

type SSEHandler struct {
	model         string
	stopSequences []string
}

type toolBlockState struct {
	contentIndex int
	id           string
	name         string
	pendingArgs  string
	open         bool
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
	toolBlocks := make(map[int]*toolBlockState)

	closeAllToolBlocks := func() error {
		positions := make([]int, 0, len(toolBlocks))
		for pos := range toolBlocks {
			positions = append(positions, pos)
		}
		sort.Ints(positions)
		for _, pos := range positions {
			state := toolBlocks[pos]
			if state == nil {
				delete(toolBlocks, pos)
				continue
			}
			if state.open {
				if err := h.sendEvent(writer, flusher, "content_block_stop", map[string]any{
					"type":  "content_block_stop",
					"index": state.contentIndex,
				}); err != nil {
					return err
				}
			} else if strings.TrimSpace(state.pendingArgs) != "" {
				logger.Warnf("Dropping unresolved tool call delta: model=%s pos=%d id=%s pending_len=%d", h.model, pos, state.id, len(state.pendingArgs))
			}
			delete(toolBlocks, pos)
		}
		return nil
	}

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
			if len(toolBlocks) > 0 {
				if err := closeAllToolBlocks(); err != nil {
					return err
				}
			}

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

			for pos, tc := range chunk.ToolCalls {
				state, ok := toolBlocks[pos]
				if !ok {
					state = &toolBlockState{}
					toolBlocks[pos] = state
				}
				if tc.ID != "" {
					state.id = tc.ID
				}
				if tc.Function.Name != "" {
					state.name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					if state.open {
						if err := h.sendEvent(writer, flusher, "content_block_delta", map[string]any{
							"type":  "content_block_delta",
							"index": state.contentIndex,
							"delta": map[string]any{
								"type":         "input_json_delta",
								"partial_json": tc.Function.Arguments,
							},
						}); err != nil {
							return err
						}
					} else {
						state.pendingArgs += tc.Function.Arguments
					}
				}

				if state.open || state.name == "" {
					continue
				}

				if state.id == "" {
					state.id = fmt.Sprintf("tool_%d", nextContentIndex)
				}
				state.contentIndex = nextContentIndex
				nextContentIndex++
				if err := h.sendEvent(writer, flusher, "content_block_start", map[string]any{
					"type":  "content_block_start",
					"index": state.contentIndex,
					"content_block": map[string]any{
						"type":  "tool_use",
						"id":    state.id,
						"name":  state.name,
						"input": map[string]any{},
					},
				}); err != nil {
					return err
				}

				state.open = true
				if state.pendingArgs != "" {
					if err := h.sendEvent(writer, flusher, "content_block_delta", map[string]any{
						"type":  "content_block_delta",
						"index": state.contentIndex,
						"delta": map[string]any{
							"type":         "input_json_delta",
							"partial_json": state.pendingArgs,
						},
					}); err != nil {
						return err
					}
					state.pendingArgs = ""
				}
			}
		}
	}

	if len(toolBlocks) > 0 {
		if err := closeAllToolBlocks(); err != nil {
			return err
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
