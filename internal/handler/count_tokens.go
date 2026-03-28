package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/hy-shine/claude-proxy-go/internal/converter"
	"github.com/hy-shine/claude-proxy-go/internal/logger"
	"github.com/hy-shine/claude-proxy-go/internal/token"
	"github.com/hy-shine/claude-proxy-go/internal/types"
)

// CountTokensRequest matches Anthropic's count_tokens API format.
type CountTokensRequest struct {
	Model    string          `json:"model"`
	Messages []types.Message `json:"messages"`
	System   any             `json:"system,omitempty"`
	Tools    []types.Tool    `json:"tools,omitempty"`
}

// CountTokensResponse matches Anthropic's count_tokens response.
type CountTokensResponse struct {
	InputTokens int `json:"input_tokens"`
}

// HandleCountTokens handles POST /v1/messages/count_tokens requests.
func (h *Handler) HandleCountTokens(w http.ResponseWriter, r *http.Request) {
	reqID := requestIDFromHeader(r)
	started := time.Now()
	defer func() {
		logger.Infof("Count tokens finished: req_id=%s latency_ms=%d", reqID, time.Since(started).Milliseconds())
	}()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CountTokensRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		logger.Infof("Count tokens decode failed: req_id=%s error=%v", reqID, err)
		h.sendError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	if req.Model == "" {
		h.sendError(w, http.StatusBadRequest, "model is required")
		return
	}

	if len(req.Messages) == 0 {
		h.sendError(w, http.StatusBadRequest, "messages cannot be empty")
		return
	}

	// Resolve the model to get upstream model name
	resolved, err := h.cfg.ResolveModel(req.Model)
	if err != nil {
		h.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check if there's multimodal content (images, documents)
	hasMultimodal := detectMultimodalContent(req.Messages)

	var inputTokens int

	if hasMultimodal {
		// For multimodal content, use upstream API for accurate counting
		inputTokens, err = h.countTokensViaUpstream(r.Context(), req, resolved.Name)
		if err != nil {
			logger.Warnf("Upstream token count failed, falling back to local: req_id=%s error=%v", reqID, err)
			// Fallback to local estimation
			inputTokens = h.countTokensLocally(req, resolved.Name)
		}
	} else {
		// For text-only content, use local estimation
		inputTokens = h.countTokensLocally(req, resolved.Name)
	}

	logger.Infof("Token count result: req_id=%s model=%s upstream=%s tokens=%d multimodal=%v",
		reqID, req.Model, resolved.Name, inputTokens, hasMultimodal)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CountTokensResponse{
		InputTokens: inputTokens,
	})
}

func (h *Handler) countTokensLocally(req CountTokensRequest, model string) int {
	counter := token.NewCounter()

	// Convert messages to eino format
	messagesReq := &types.MessagesRequest{
		Messages: req.Messages,
		System:   req.System,
	}
	messages, _, err := converter.ToEinoRequest(messagesReq)
	if err != nil {
		// Fallback: just count message content directly
		return counter.CountText(extractAllText(req.Messages), model)
	}

	total := counter.CountMessages(messages, model)
	total += counter.CountSystemPrompt(req.System, model)

	// Add tokens for tools
	if len(req.Tools) > 0 {
		toolsAny := make([]any, len(req.Tools))
		for i, t := range req.Tools {
			toolsAny[i] = map[string]any{
				"name":         t.Name,
				"description":  t.Description,
				"input_schema": t.InputSchema,
			}
		}
		total += counter.CountTools(toolsAny, model)
	}

	return total
}

func (h *Handler) countTokensViaUpstream(ctx context.Context, req CountTokensRequest, model string) (int, error) {
	// Create a minimal request to get usage info
	messagesReq := &types.MessagesRequest{
		Model:    req.Model,
		Messages: req.Messages,
		System:   req.System,
		Tools:    req.Tools,
		MaxTokens: func() int { return 1 }(), // Minimal output
	}

	messages, opts, err := converter.ToEinoRequest(messagesReq)
	if err != nil {
		return 0, err
	}

	// Set minimal options
	if opts == nil {
		opts = &converter.ChatOptions{}
	}
	opts.MaxTokens = intPtr(1)
	opts.Temperature = floatPtr(0)

	// Make the request
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := h.client.Generate(ctx, req.Model, messages, opts)
	if err != nil {
		return 0, err
	}

	// Extract input tokens from response
	if resp.ResponseMeta != nil && resp.ResponseMeta.Usage != nil {
		return resp.ResponseMeta.Usage.PromptTokens, nil
	}

	return 0, nil
}

func detectMultimodalContent(messages []types.Message) bool {
	for _, msg := range messages {
		if blocks, ok := msg.Content.([]any); ok {
			for _, b := range blocks {
				if block, ok := b.(map[string]any); ok {
					t, _ := block["type"].(string)
					if t == "image" || t == "document" {
						return true
					}
				}
			}
		}
	}
	return false
}

func extractAllText(messages []types.Message) string {
	var text string
	for _, msg := range messages {
		text += msg.GetContentAsString() + " "
	}
	return text
}

func intPtr(i int) *int       { return &i }
func floatPtr(f float64) *float64 { return &f }

// Ensure schema.Message is imported
var _ = func() *schema.Message { return nil }
