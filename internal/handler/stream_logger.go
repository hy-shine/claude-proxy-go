package handler

import (
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/hy-shine/claude-proxy-go/internal/logger"
	"github.com/hy-shine/claude-proxy-go/pkg/eino"
)

type streamLogger struct {
	inner   eino.StreamReader
	reqID   string
	model   string
	started time.Time
	chunks  []string
	tokens  int
}

func newStreamLogger(inner eino.StreamReader, reqID, model string) *streamLogger {
	return &streamLogger{
		inner:   inner,
		reqID:   reqID,
		model:   model,
		started: time.Now(),
		chunks:  make([]string, 0),
		tokens:  0,
	}
}

func (s *streamLogger) Recv() (*schema.Message, error) {
	msg, err := s.inner.Recv()
	if err != nil {
		return msg, err
	}
	if msg != nil && msg.Content != "" {
		s.chunks = append(s.chunks, msg.Content)
	}
	s.tokens++
	return msg, nil
}

func (s *streamLogger) Close() {
	s.inner.Close()

	// Log stream summary
	content := strings.Join(s.chunks, "")
	if len(content) > 500 {
		content = content[:500] + "...(truncated)"
	}

	logger.Debugw("Stream response summary",
		"req_id", s.reqID,
		"model", s.model,
		"duration_ms", time.Since(s.started).Milliseconds(),
		"total_chunks", len(s.chunks),
		"total_tokens", s.tokens,
		"content_preview", content,
	)
}
