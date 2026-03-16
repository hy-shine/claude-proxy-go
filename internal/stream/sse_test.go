package stream

import (
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

type fakeStream struct {
	chunks []*schema.Message
	index  int
}

func (f *fakeStream) Recv() (*schema.Message, error) {
	if f.index >= len(f.chunks) {
		return nil, io.EOF
	}
	chunk := f.chunks[f.index]
	f.index++
	return chunk, nil
}

func (f *fakeStream) Close() {}

type errorStream struct {
	chunks    []*schema.Message
	index     int
	failAfter int
	err       error
}

func (s *errorStream) Recv() (*schema.Message, error) {
	if s.index < len(s.chunks) {
		chunk := s.chunks[s.index]
		s.index++
		return chunk, nil
	}
	if s.failAfter >= 0 && s.index >= s.failAfter {
		s.index++
		if s.err == nil {
			s.err = errors.New("stream failed")
		}
		return nil, s.err
	}
	return nil, io.EOF
}

func (s *errorStream) Close() {}

func TestStreamToClientTextOnly(t *testing.T) {
	h := NewSSEHandler("m1", nil)
	rec := httptest.NewRecorder()

	stream := &fakeStream{
		chunks: []*schema.Message{
			{
				Content: "Hello",
				ResponseMeta: &schema.ResponseMeta{
					Usage: &schema.TokenUsage{
						PromptTokens:     7,
						CompletionTokens: 3,
					},
				},
			},
		},
	}

	if err := h.StreamToClient(stream, rec); err != nil {
		t.Fatalf("StreamToClient() error = %v", err)
	}

	events := parseSSEEvents(t, rec.Body.String())
	if len(events) < 7 {
		t.Fatalf("expected >= 7 events, got %d", len(events))
	}

	wantOrder := []string{
		"message_start",
		"content_block_start",
		"ping",
		"content_block_delta",
		"content_block_stop",
		"message_delta",
		"message_stop",
	}
	for i, want := range wantOrder {
		if events[i].Event != want {
			t.Fatalf("event[%d] = %s, want %s", i, events[i].Event, want)
		}
	}

	if delta := events[5].Data["delta"].(map[string]any)["stop_reason"]; delta != "end_turn" {
		t.Fatalf("stop_reason = %v, want end_turn", delta)
	}
	usage := events[5].Data["usage"].(map[string]any)
	if usage["input_tokens"].(float64) != 7 || usage["output_tokens"].(float64) != 3 {
		t.Fatalf("usage mismatch: %#v", usage)
	}
}

func TestStreamToClientToolUse(t *testing.T) {
	h := NewSSEHandler("m1", nil)
	rec := httptest.NewRecorder()

	stream := &fakeStream{
		chunks: []*schema.Message{
			{
				ToolCalls: []schema.ToolCall{
					{
						ID:   "tool_1",
						Type: "function",
						Function: schema.FunctionCall{
							Name:      "calc",
							Arguments: `{"x":1}`,
						},
					},
				},
				ResponseMeta: &schema.ResponseMeta{
					Usage: &schema.TokenUsage{
						PromptTokens:     9,
						CompletionTokens: 4,
					},
				},
			},
		},
	}

	if err := h.StreamToClient(stream, rec); err != nil {
		t.Fatalf("StreamToClient() error = %v", err)
	}

	events := parseSSEEvents(t, rec.Body.String())
	names := eventNames(events)

	expectedContainsInOrder := []string{
		"message_start",
		"content_block_start",
		"ping",
		"content_block_stop",
		"content_block_start",
		"content_block_delta",
		"content_block_stop",
		"message_delta",
		"message_stop",
	}
	assertContainsInOrder(t, names, expectedContainsInOrder)

	lastDelta := findLastEvent(t, events, "message_delta")
	if got := lastDelta.Data["delta"].(map[string]any)["stop_reason"]; got != "tool_use" {
		t.Fatalf("stop_reason = %v, want tool_use", got)
	}
}

func TestStreamToClientToolThenText(t *testing.T) {
	h := NewSSEHandler("m1", nil)
	rec := httptest.NewRecorder()

	stream := &fakeStream{
		chunks: []*schema.Message{
			{
				ToolCalls: []schema.ToolCall{
					{
						ID:   "tool_1",
						Type: "function",
						Function: schema.FunctionCall{
							Name:      "calc",
							Arguments: `{"x":1}`,
						},
					},
				},
			},
			{
				Content: "final text",
			},
		},
	}

	if err := h.StreamToClient(stream, rec); err != nil {
		t.Fatalf("StreamToClient() error = %v", err)
	}

	events := parseSSEEvents(t, rec.Body.String())
	names := eventNames(events)
	assertContainsInOrder(t, names, []string{
		"content_block_stop",  // close initial text index 0
		"content_block_start", // tool index 1
		"content_block_delta", // tool delta
		"content_block_stop",  // tool stop
		"content_block_start", // reopen text index 2
		"content_block_delta", // text delta index 2
		"content_block_stop",  // final text close index 2
	})

	var (
		seenTextStart bool
		seenTextDelta bool
		seenTextStop  bool
	)
	for _, ev := range events {
		if ev.Event == "content_block_start" {
			cb, _ := ev.Data["content_block"].(map[string]any)
			if cb["type"] == "text" && int(ev.Data["index"].(float64)) == 2 {
				seenTextStart = true
			}
		}
		if ev.Event == "content_block_delta" {
			delta, _ := ev.Data["delta"].(map[string]any)
			if delta["type"] == "text_delta" && int(ev.Data["index"].(float64)) == 2 {
				seenTextDelta = true
			}
		}
		if ev.Event == "content_block_stop" && int(ev.Data["index"].(float64)) == 2 {
			seenTextStop = true
		}
	}

	if !seenTextStart || !seenTextDelta || !seenTextStop {
		t.Fatalf("expected reopened text block on index 2, got events=%v", names)
	}
}

func TestStreamToClientMapsLengthToMaxTokens(t *testing.T) {
	h := NewSSEHandler("m1", nil)
	rec := httptest.NewRecorder()

	stream := &fakeStream{
		chunks: []*schema.Message{
			{
				Content: "partial",
				ResponseMeta: &schema.ResponseMeta{
					FinishReason: "length",
					Usage: &schema.TokenUsage{
						PromptTokens:     10,
						CompletionTokens: 20,
					},
				},
			},
		},
	}

	if err := h.StreamToClient(stream, rec); err != nil {
		t.Fatalf("StreamToClient() error = %v", err)
	}

	events := parseSSEEvents(t, rec.Body.String())
	lastDelta := findLastEvent(t, events, "message_delta")
	if got := lastDelta.Data["delta"].(map[string]any)["stop_reason"]; got != "max_tokens" {
		t.Fatalf("stop_reason = %v, want max_tokens", got)
	}
}

func TestStreamToClientIncludesStopSequenceWhenResolvable(t *testing.T) {
	h := NewSSEHandler("m1", []string{"<END>"})
	rec := httptest.NewRecorder()

	stream := &fakeStream{
		chunks: []*schema.Message{
			{
				Content: "partial",
				ResponseMeta: &schema.ResponseMeta{
					FinishReason: "stop_sequence",
					Usage: &schema.TokenUsage{
						PromptTokens:     5,
						CompletionTokens: 6,
					},
				},
			},
		},
	}

	if err := h.StreamToClient(stream, rec); err != nil {
		t.Fatalf("StreamToClient() error = %v", err)
	}

	events := parseSSEEvents(t, rec.Body.String())
	lastDelta := findLastEvent(t, events, "message_delta")
	delta := lastDelta.Data["delta"].(map[string]any)
	if got := delta["stop_reason"]; got != "stop_sequence" {
		t.Fatalf("stop_reason = %v, want stop_sequence", got)
	}
	if got := delta["stop_sequence"]; got != "<END>" {
		t.Fatalf("stop_sequence = %v, want <END>", got)
	}
}

func TestStreamToClientEmitsErrorEventOnRecvError(t *testing.T) {
	h := NewSSEHandler("m1", nil)
	rec := httptest.NewRecorder()

	stream := &errorStream{
		chunks:    nil,
		failAfter: 0,
		err:       errors.New("upstream disconnected"),
	}

	if err := h.StreamToClient(stream, rec); err != nil {
		t.Fatalf("StreamToClient() error = %v", err)
	}

	events := parseSSEEvents(t, rec.Body.String())
	names := eventNames(events)
	if !containsEvent(names, "error") {
		t.Fatalf("expected error event, got %v", names)
	}
	if containsEvent(names, "message_stop") {
		t.Fatalf("unexpected message_stop in error stream: %v", names)
	}
	errEvent := findLastEvent(t, events, "error")
	errBody := errEvent.Data["error"].(map[string]any)
	if errBody["type"] != "api_error" {
		t.Fatalf("unexpected error type: %v", errBody["type"])
	}
	if !strings.Contains(errBody["message"].(string), "upstream disconnected") {
		t.Fatalf("unexpected error message: %v", errBody["message"])
	}
}

type sseEvent struct {
	Event string
	Data  map[string]any
}

func parseSSEEvents(t *testing.T, payload string) []sseEvent {
	t.Helper()

	segments := strings.Split(payload, "\n\n")
	events := make([]sseEvent, 0, len(segments))

	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" || seg == "data: [DONE]" {
			continue
		}

		var ev sseEvent
		for _, line := range strings.Split(seg, "\n") {
			if strings.HasPrefix(line, "event: ") {
				ev.Event = strings.TrimPrefix(line, "event: ")
				continue
			}
			if strings.HasPrefix(line, "data: ") {
				raw := strings.TrimPrefix(line, "data: ")
				var data map[string]any
				if err := json.Unmarshal([]byte(raw), &data); err != nil {
					t.Fatalf("failed to parse event data %q: %v", raw, err)
				}
				ev.Data = data
			}
		}
		if ev.Event != "" {
			events = append(events, ev)
		}
	}
	return events
}

func eventNames(events []sseEvent) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, e.Event)
	}
	return out
}

func assertContainsInOrder(t *testing.T, got []string, want []string) {
	t.Helper()

	pos := 0
	for _, g := range got {
		if pos < len(want) && g == want[pos] {
			pos++
		}
	}
	if pos != len(want) {
		t.Fatalf("order mismatch, got=%v wantSubsequence=%v", got, want)
	}
}

func findLastEvent(t *testing.T, events []sseEvent, name string) sseEvent {
	t.Helper()
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Event == name {
			return events[i]
		}
	}
	t.Fatalf("event %q not found", name)
	return sseEvent{}
}

func containsEvent(events []string, target string) bool {
	for _, e := range events {
		if e == target {
			return true
		}
	}
	return false
}
