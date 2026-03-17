package converter

import "testing"

func TestMapStopReason(t *testing.T) {
	tests := []struct {
		name        string
		finish      string
		hasToolCall bool
		want        string
	}{
		{name: "tool calls", finish: "tool_calls", want: "tool_use"},
		{name: "tool use", finish: "tool_use", want: "tool_use"},
		{name: "length", finish: "length", want: "max_tokens"},
		{name: "max tokens", finish: "max_tokens", want: "max_tokens"},
		{name: "stop sequence", finish: "stop_sequence", want: "stop_sequence"},
		{name: "stop", finish: "stop", want: "end_turn"},
		{name: "empty with tool", finish: "", hasToolCall: true, want: "tool_use"},
		{name: "unknown with tool", finish: "weird_reason", hasToolCall: true, want: "tool_use"},
		{name: "unknown no tool", finish: "weird_reason", hasToolCall: false, want: "end_turn"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapStopReason(tt.finish, tt.hasToolCall)
			if got != tt.want {
				t.Fatalf("MapStopReason(%q, %v) = %q, want %q", tt.finish, tt.hasToolCall, got, tt.want)
			}
		})
	}
}

func TestResolveStopSequence(t *testing.T) {
	tests := []struct {
		name       string
		finish     string
		configured []string
		want       *string
	}{
		{
			name:       "single stop sequence with matching finish reason",
			finish:     "stop_sequence",
			configured: []string{"<END>"},
			want:       testStringPtr("<END>"),
		},
		{
			name:       "empty configured list",
			finish:     "stop_sequence",
			configured: nil,
			want:       nil,
		},
		{
			name:       "multiple configured list is ambiguous",
			finish:     "stop_sequence",
			configured: []string{"A", "B"},
			want:       nil,
		},
		{
			name:       "non stop_sequence finish reason",
			finish:     "stop",
			configured: []string{"<END>"},
			want:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveStopSequence(tt.finish, tt.configured)
			if !equalStringPtr(got, tt.want) {
				t.Fatalf("ResolveStopSequence(%q, %#v) = %#v, want %#v", tt.finish, tt.configured, got, tt.want)
			}
		})
	}
}

func testStringPtr(v string) *string {
	return &v
}

func equalStringPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
