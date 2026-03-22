package logger

import (
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
		hasError bool
	}{
		{"debug", LevelDebug, false},
		{"DEBUG", LevelDebug, false},
		{"  debug  ", LevelDebug, false},
		{"info", LevelInfo, false},
		{"INFO", LevelInfo, false},
		{"", LevelInfo, false}, // default
		{"warn", LevelWarn, false},
		{"warning", LevelWarn, false},
		{"WARN", LevelWarn, false},
		{"error", LevelError, false},
		{"ERROR", LevelError, false},
		{"invalid", LevelInfo, true}, // unsupported, returns error
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseLevel(tt.input)
			if tt.hasError {
				if err == nil {
					t.Errorf("ParseLevel(%q) expected error, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("ParseLevel(%q) unexpected error: %v", tt.input, err)
				}
				if got != tt.expected {
					t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.expected)
				}
			}
		})
	}
}

func TestInitDefaultConfig(t *testing.T) {
	// Test with empty config (should use defaults)
	cfg := Config{
		Level:  "info",
		Format: "",
	}

	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	_ = Sync()
}

func TestInitJSONFormat(t *testing.T) {
	cfg := Config{
		Level:  "debug",
		Format: "json",
	}

	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	_ = Sync()
}

func TestUnsupportedLevel(t *testing.T) {
	cfg := Config{
		Level:  "invalid",
		Format: "text",
	}

	err := Init(cfg)
	if err == nil {
		t.Error("Init should fail with unsupported level")
	}
}
