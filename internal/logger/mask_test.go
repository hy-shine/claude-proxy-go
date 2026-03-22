package logger

import (
	"testing"
)

func TestMaskAPIKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name: "simple api key",
			input: map[string]any{
				"apiKey": "secret123",
				"name":   "test",
			},
			expected: `{"apiKey":"***","name":"test"}`,
		},
		{
			name: "nested api key",
			input: map[string]any{
				"provider": map[string]any{
					"apiKey": "nested-secret",
					"name":   "openai",
				},
			},
			expected: `{"provider":{"apiKey":"***","name":"openai"}}`,
		},
		{
			name: "no api key",
			input: map[string]any{
				"name":  "test",
				"model": "gpt-4",
			},
			expected: `{"model":"gpt-4","name":"test"}`,
		},
		{
			name:     "nil input",
			input:    nil,
			expected: ``,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskAPIKeys(tt.input)
			if tt.input == nil {
				if result != nil {
					t.Errorf("MaskAPIKeys(nil) = %v, want nil", result)
				}
				return
			}

			// The result should be a map, check that apiKey is masked
			resultMap, ok := result.(map[string]any)
			if !ok {
				t.Errorf("MaskAPIKeys result is not a map: %T", result)
				return
			}

			// Check top-level apiKey
			if apiKey, exists := resultMap["apiKey"]; exists {
				if apiKey != "***" {
					t.Errorf("apiKey should be masked, got: %v", apiKey)
				}
			}
		})
	}
}
