package eino

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/hy-shine/claude-proxy-go/internal/config"
)

func TestRetryRetriesOn429(t *testing.T) {
	cfg := &config.Config{
		Retry: config.RetryConfig{
			MaxRetries:       2,
			InitialBackoffMS: 1,
			MaxBackoffMS:     1,
		},
	}

	attempts := 0
	result, err := retry(context.Background(), cfg, func() (string, error) {
		attempts++
		if attempts < 3 {
			return "", &openai.APIError{HTTPStatusCode: 429, Message: "rate limit"}
		}
		return "ok", nil
	})

	if err != nil {
		t.Fatalf("retry() error = %v", err)
	}
	if result != "ok" {
		t.Fatalf("result = %q, want ok", result)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestRetryDoesNotRetryOnClientError(t *testing.T) {
	cfg := &config.Config{
		Retry: config.RetryConfig{
			MaxRetries:       2,
			InitialBackoffMS: 1,
			MaxBackoffMS:     1,
		},
	}

	attempts := 0
	_, err := retry(context.Background(), cfg, func() (string, error) {
		attempts++
		return "", &ClientError{
			StatusCode: 400,
			Message:    "bad request",
			Err:        errors.New("bad request"),
		}
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestExtractStatusCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "openai pointer error",
			err:  &openai.APIError{HTTPStatusCode: 503, Message: "unavailable"},
			want: 503,
		},
		{
			name: "regex fallback with status",
			err:  fmt.Errorf("upstream returned status: 502"),
			want: 502,
		},
		{
			name: "regex fallback with status code",
			err:  fmt.Errorf("request failed, status code=504"),
			want: 504,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractStatusCode(tt.err)
			if got != tt.want {
				t.Fatalf("extractStatusCode() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestIsRetryableErrorOnServerCodes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "retryable 429",
			err:  &openai.APIError{HTTPStatusCode: 429, Message: "rate limit"},
			want: true,
		},
		{
			name: "retryable 500",
			err:  fmt.Errorf("upstream status code: 500"),
			want: true,
		},
		{
			name: "non retryable 400",
			err:  &openai.APIError{HTTPStatusCode: 400, Message: "bad request"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableError(tt.err)
			if got != tt.want {
				t.Fatalf("isRetryableError() = %v, want %v", got, tt.want)
			}
		})
	}
}
