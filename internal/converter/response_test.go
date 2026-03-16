package converter

import (
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestParseToolResultContentPreservesStructuredPayload(t *testing.T) {
	content := map[string]any{
		"type": "image",
		"url":  "https://example.com/a.png",
	}

	got := ParseToolResultContent(content)
	if !strings.Contains(got, `"type":"image"`) {
		t.Fatalf("expected JSON payload, got %q", got)
	}
}

func TestParseToolResultContentTextBlocks(t *testing.T) {
	content := []any{
		map[string]any{"type": "text", "text": "hello"},
		map[string]any{"type": "text", "text": "world"},
	}

	got := ParseToolResultContent(content)
	if got != "hello\nworld" {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestFromEinoResponseIncludesStopSequenceWhenResolvable(t *testing.T) {
	resp := &schema.Message{
		Role:    schema.Assistant,
		Content: "done",
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: "stop_sequence",
			Usage: &schema.TokenUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
			},
		},
	}

	out := FromEinoResponse(resp, "model-id", []string{"<END>"})
	if out.StopReason != "stop_sequence" {
		t.Fatalf("stop reason mismatch: %q", out.StopReason)
	}
	if out.StopSequence == nil || *out.StopSequence != "<END>" {
		t.Fatalf("stop sequence mismatch: %#v", out.StopSequence)
	}
	if out.Usage.InputTokens != 10 || out.Usage.OutputTokens != 20 {
		t.Fatalf("usage mismatch: %#v", out.Usage)
	}
}
