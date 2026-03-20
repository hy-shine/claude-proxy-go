package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/hy-shine/claude-proxy-go/internal/config"
	"github.com/hy-shine/claude-proxy-go/internal/converter"
	"github.com/hy-shine/claude-proxy-go/internal/logger"
	"github.com/hy-shine/claude-proxy-go/internal/stream"
	"github.com/hy-shine/claude-proxy-go/internal/types"
	"github.com/hy-shine/claude-proxy-go/pkg/eino"
)

type Handler struct {
	cfg    *config.Config
	client *eino.Client
}

var requestSeq uint64

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
	reqID := requestIDFromHeader(r)
	rw := newResponseLogger(w)
	started := time.Now()
	defer func() {
		logger.Infof("Request finished: req_id=%s path=%s method=%s status=%d latency_ms=%d bytes=%d",
			reqID, r.URL.Path, r.Method, rw.StatusCode(), time.Since(started).Milliseconds(), rw.BytesWritten())
	}()

	if r.Method != http.MethodPost {
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req types.MessagesRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		logger.Infof("Request decode failed: req_id=%s path=%s remote=%s error=%v", reqID, r.URL.Path, r.RemoteAddr, err)
		h.sendError(rw, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	if err := validateMessagesRequest(&req); err != nil {
		logger.Infof("Request validation failed: req_id=%s model=%s path=%s error=%v", reqID, req.Model, r.URL.Path, err)
		h.sendError(rw, http.StatusBadRequest, err.Error())
		return
	}

	logger.Infof("Request accepted: req_id=%s model=%s stream=%v remote=%s", reqID, req.Model, req.Stream != nil && *req.Stream, r.RemoteAddr)

	if req.Stream != nil && *req.Stream {
		h.handleStream(rw, r, &req, reqID)
		return
	}

	h.handleNonStream(rw, r, &req, reqID)
}

func (h *Handler) handleNonStream(w http.ResponseWriter, r *http.Request, req *types.MessagesRequest, reqID string) {
	messages, opts, err := converter.ToEinoRequest(req)
	if err != nil {
		logger.Infof("Request conversion failed: req_id=%s model=%s stream=false error=%v", reqID, req.Model, err)
		h.sendError(w, http.StatusBadRequest, "Unsupported request: "+err.Error())
		return
	}

	timeout := time.Duration(h.cfg.Timeout.RequestTimeout) * time.Second
	logger.Debugf("Request timeout configured: req_id=%s model=%s timeout=%v", reqID, req.Model, timeout)

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// Track context cancellation timing
	start := time.Now()
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			elapsed := time.Since(start)
			switch ctx.Err() {
			case context.Canceled:
				logger.Warnf("Request canceled by client: req_id=%s model=%s elapsed=%v", reqID, req.Model, elapsed.Round(time.Millisecond))
			case context.DeadlineExceeded:
				logger.Warnf("Request timed out: req_id=%s model=%s elapsed=%v timeout=%v", reqID, req.Model, elapsed.Round(time.Millisecond), timeout)
			default:
				logger.Warnf("Request context done: req_id=%s model=%s elapsed=%v err=%v", reqID, req.Model, elapsed.Round(time.Millisecond), ctx.Err())
			}
		case <-done:
		}
	}()
	defer close(done)

	resp, err := h.client.Generate(ctx, req.Model, messages, opts)
	if err != nil {
		h.sendModelError(w, reqID, "Generation failed", err)
		return
	}

	anthropicResp := converter.FromEinoResponse(resp, req.Model, req.StopSequences)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(anthropicResp); err != nil {
		logger.Errorf("Response encoding failed: req_id=%s model=%s error=%v", reqID, req.Model, err)
		h.sendError(w, http.StatusInternalServerError, "Failed to encode response")
		return
	}
	logger.Debugf("Request completed: req_id=%s model=%s stream=false", reqID, req.Model)
}

func (h *Handler) handleStream(w http.ResponseWriter, r *http.Request, req *types.MessagesRequest, reqID string) {
	messages, opts, err := converter.ToEinoRequest(req)
	if err != nil {
		logger.Infof("Request conversion failed: req_id=%s model=%s stream=true error=%v", reqID, req.Model, err)
		h.sendError(w, http.StatusBadRequest, "Unsupported request: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(h.cfg.Timeout.StreamTimeout)*time.Second)
	defer cancel()

	streamResp, err := h.client.Stream(ctx, req.Model, messages, opts)
	if err != nil {
		h.sendModelError(w, reqID, "Stream failed", err)
		return
	}
	defer streamResp.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sseHandler := stream.NewSSEHandler(req.Model, req.StopSequences)
	if err := sseHandler.StreamToClient(streamResp, w); err != nil {
		logger.Warnf("Stream error: req_id=%s model=%s error=%v", reqID, req.Model, err)
		return
	}
	logger.Debugf("Request completed: req_id=%s model=%s stream=true", reqID, req.Model)
}

func (h *Handler) sendModelError(w http.ResponseWriter, reqID, prefix string, err error) {
	if errors.Is(err, context.Canceled) {
		logger.Infof("%s: req_id=%s canceled by client", prefix, reqID)
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		logger.Warnf("%s: req_id=%s timed out", prefix, reqID)
		h.sendError(w, http.StatusGatewayTimeout, "upstream request timeout")
		return
	}

	var clientErr *eino.ClientError
	if errors.As(err, &clientErr) {
		if clientErr.StatusCode >= http.StatusInternalServerError {
			logger.Warnf("%s: req_id=%s status=%d message=%s", prefix, reqID, clientErr.StatusCode, clientErr.Message)
		} else {
			logger.Infof("%s: req_id=%s status=%d message=%s", prefix, reqID, clientErr.StatusCode, clientErr.Message)
		}
		h.sendError(w, clientErr.StatusCode, clientErr.Message)
		return
	}
	logger.Errorf("%s: req_id=%s error=%v", prefix, reqID, err)
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

func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
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

func requestIDFromHeader(r *http.Request) string {
	reqID := r.Header.Get("X-Request-Id")
	if reqID == "" {
		reqID = r.Header.Get("X-Request-ID")
	}
	if reqID != "" {
		return reqID
	}
	seq := atomic.AddUint64(&requestSeq, 1)
	return fmt.Sprintf("req_%d_%d", time.Now().UnixNano(), seq)
}

type responseLogger struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func newResponseLogger(w http.ResponseWriter) *responseLogger {
	return &responseLogger{
		ResponseWriter: w,
		status:         http.StatusOK,
	}
}

func (l *responseLogger) WriteHeader(code int) {
	if !l.wroteHeader {
		l.status = code
		l.wroteHeader = true
	}
	l.ResponseWriter.WriteHeader(code)
}

func (l *responseLogger) Write(p []byte) (int, error) {
	if !l.wroteHeader {
		l.WriteHeader(http.StatusOK)
	}
	n, err := l.ResponseWriter.Write(p)
	l.bytes += n
	return n, err
}

func (l *responseLogger) StatusCode() int {
	return l.status
}

func (l *responseLogger) BytesWritten() int {
	return l.bytes
}

func (l *responseLogger) Flush() {
	if flusher, ok := l.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
