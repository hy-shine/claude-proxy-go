package handler

import (
	"io"
	"strings"
	"testing"

	"github.com/hy-shine/claude-proxy-go/internal/types"
)

func TestValidateMessagesRequestAcceptsOptionalTypeMessage(t *testing.T) {
	req := &types.MessagesRequest{
		Type:      "message",
		Model:     "m1",
		MaxTokens: 128,
		Messages:  []types.Message{{Role: "user", Content: "hello"}},
	}

	if err := validateMessagesRequest(req); err != nil {
		t.Fatalf("validateMessagesRequest() error = %v", err)
	}
}

func TestValidateMessagesRequestRejectsUnexpectedType(t *testing.T) {
	req := &types.MessagesRequest{
		Type:      "request",
		Model:     "m1",
		MaxTokens: 128,
		Messages:  []types.Message{{Role: "user", Content: "hello"}},
	}

	err := validateMessagesRequest(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `type must be "message"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDecodeJSONStrictAllowsToolTypeField(t *testing.T) {
	body := io.NopCloser(strings.NewReader(`{
		"type":"message",
		"model":"m1",
		"max_tokens":128,
		"messages":[{"role":"user","content":"hello"}],
		"tools":[{"type":"custom","name":"t1","input_schema":{"type":"object","properties":{}}}]
	}`))

	var req types.MessagesRequest
	if err := decodeJSONStrict(body, &req); err != nil {
		t.Fatalf("decodeJSONStrict() error = %v", err)
	}

	if len(req.Tools) != 1 || req.Tools[0].Type != "custom" {
		t.Fatalf("unexpected tools decode result: %+v", req.Tools)
	}
}

func TestDecodeJSONStrictAllowsUnknownTopLevelField(t *testing.T) {
	body := io.NopCloser(strings.NewReader(`{
		"type":"message",
		"model":"m1",
		"max_tokens":128,
		"messages":[{"role":"user","content":"hello"}],
		"x_unknown":"kept-compatible"
	}`))

	var req types.MessagesRequest
	if err := decodeJSONStrict(body, &req); err != nil {
		t.Fatalf("decodeJSONStrict() error = %v", err)
	}
}

func TestDecodeJSONStrictRejectsMultipleJSONValues(t *testing.T) {
	body := io.NopCloser(strings.NewReader(`{"model":"m1","max_tokens":128,"messages":[{"role":"user","content":"hello"}]} {"x":1}`))

	var req types.MessagesRequest
	err := decodeJSONStrict(body, &req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "multiple JSON values are not allowed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
