package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/1rgs/claude-code-proxy-go/internal/config"
	"github.com/1rgs/claude-code-proxy-go/internal/converter"
	"github.com/1rgs/claude-code-proxy-go/internal/logger"
	"github.com/1rgs/claude-code-proxy-go/internal/stream"
	"github.com/1rgs/claude-code-proxy-go/internal/types"
	"github.com/1rgs/claude-code-proxy-go/pkg/eino"
)

type Handler struct {
	cfg    *config.Config
	client *eino.Client
}

func NewHandler(cfg *config.Config) (*Handler, error) {
	client, err := eino.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	return &Handler{
		cfg:    cfg,
		client: client,
	}, nil
}

func (h *Handler) HandleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req types.MessagesRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		logger.Warnf("Request decode failed: path=%s remote=%s error=%v", r.URL.Path, r.RemoteAddr, err)
		h.sendError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	if err := validateMessagesRequest(&req); err != nil {
		logger.Warnf("Request validation failed: model=%s path=%s error=%v", req.Model, r.URL.Path, err)
		h.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	logger.Infof("Request accepted: model=%s stream=%v remote=%s", req.Model, req.Stream != nil && *req.Stream, r.RemoteAddr)

	if req.Stream != nil && *req.Stream {
		h.handleStream(w, r, &req)
		return
	}

	h.handleNonStream(w, r, &req)
}

func (h *Handler) handleNonStream(w http.ResponseWriter, r *http.Request, req *types.MessagesRequest) {
	messages, opts, err := converter.ToEinoRequest(req)
	if err != nil {
		logger.Warnf("Request conversion failed: model=%s stream=false error=%v", req.Model, err)
		h.sendError(w, http.StatusBadRequest, "Unsupported request: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(h.cfg.Timeout.RequestSeconds)*time.Second)
	defer cancel()

	resp, err := h.client.Generate(ctx, req.Model, messages, opts)
	if err != nil {
		h.sendModelError(w, "Generation failed", err)
		return
	}

	anthropicResp := converter.FromEinoResponse(resp, req.Model, req.StopSequences)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(anthropicResp); err != nil {
		logger.Errorf("Response encoding failed: model=%s error=%v", req.Model, err)
		h.sendError(w, http.StatusInternalServerError, "Failed to encode response")
		return
	}
	logger.Debugf("Request completed: model=%s stream=false", req.Model)
}

func (h *Handler) handleStream(w http.ResponseWriter, r *http.Request, req *types.MessagesRequest) {
	messages, opts, err := converter.ToEinoRequest(req)
	if err != nil {
		logger.Warnf("Request conversion failed: model=%s stream=true error=%v", req.Model, err)
		h.sendError(w, http.StatusBadRequest, "Unsupported request: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(h.cfg.Timeout.StreamSeconds)*time.Second)
	defer cancel()

	streamResp, err := h.client.Stream(ctx, req.Model, messages, opts)
	if err != nil {
		h.sendModelError(w, "Stream failed", err)
		return
	}
	defer streamResp.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sseHandler := stream.NewSSEHandler(req.Model, req.StopSequences)
	if err := sseHandler.StreamToClient(streamResp, w); err != nil {
		logger.Warnf("Stream error: model=%s error=%v", req.Model, err)
		return
	}
	logger.Debugf("Request completed: model=%s stream=true", req.Model)
}

func (h *Handler) sendModelError(w http.ResponseWriter, prefix string, err error) {
	var clientErr *eino.ClientError
	if errors.As(err, &clientErr) {
		logger.Warnf("%s: status=%d message=%s", prefix, clientErr.StatusCode, clientErr.Message)
		h.sendError(w, clientErr.StatusCode, clientErr.Message)
		return
	}
	logger.Errorf("%s: %v", prefix, err)
	h.sendError(w, http.StatusInternalServerError, prefix+": "+err.Error())
}

func (h *Handler) sendError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(types.ErrorResponse{
		Type:    "error",
		Message: message,
	})
}

func (h *Handler) HandleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Claude API compatible proxy",
	})
}

func decodeJSONStrict(r io.ReadCloser, out any) error {
	defer r.Close()
	dec := json.NewDecoder(r)
	if err := dec.Decode(out); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("multiple JSON values are not allowed")
	}
	return nil
}

func validateMessagesRequest(req *types.MessagesRequest) error {
	if req.Type != "" && req.Type != "message" {
		return errors.New(`type must be "message" when provided`)
	}
	if req.Model == "" {
		return errors.New("model is required and must be a model_id")
	}
	if req.MaxTokens <= 0 {
		return errors.New("max_tokens must be > 0")
	}
	if len(req.Messages) == 0 {
		return errors.New("messages cannot be empty")
	}
	return nil
}
