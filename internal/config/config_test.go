package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAppliesDefaultsAndResolvesModel(t *testing.T) {
	path := writeTempConfig(t, `{
  "providers": {
    "openai": {
      "apiKey": "sk-test",
      "models": {
        "m1": { "name": "gpt-4.1-mini" }
      }
    }
  }
}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got := cfg.Providers["openai"].BaseURL; got != "https://api.openai.com/v1" {
		t.Fatalf("openai baseUrl default mismatch: %q", got)
	}
	if got := cfg.Log.Level; got != "info" {
		t.Fatalf("log level default mismatch: %q", got)
	}

	model, err := cfg.ResolveModel("m1")
	if err != nil {
		t.Fatalf("ResolveModel() error = %v", err)
	}
	if model.Provider != "openai" || model.Name != "gpt-4.1-mini" {
		t.Fatalf("resolved model mismatch: %#v", model)
	}
	if model.APIType != "openai" {
		t.Fatalf("apiType default mismatch: %#v", model)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := writeTempConfig(t, `{
  "providers": {
    "openai": {
      "apiKey": "sk-test",
      "models": {
        "m1": { "name": "gpt-4.1-mini", "unknown_field": 1 }
      }
    }
  }
}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse config file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadAllowsArbitraryProviderNameWithDefaultOpenAIType(t *testing.T) {
	path := writeTempConfig(t, `{
  "providers": {
    "nvidia": {
      "apiKey": "sk-nvidia",
      "models": { "m1": { "name": "gpt-4.1-mini" } }
    }
  }
}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	model, err := cfg.ResolveModel("m1")
	if err != nil {
		t.Fatalf("ResolveModel() error = %v", err)
	}
	if model.Provider != "nvidia" || model.APIType != "openai" {
		t.Fatalf("resolved model mismatch: %#v", model)
	}
	if model.BaseURL != "https://api.openai.com/v1" {
		t.Fatalf("baseUrl default mismatch: %q", model.BaseURL)
	}
}

func TestLoadRejectsWhenAllModelsDisabled(t *testing.T) {
	path := writeTempConfig(t, `{
  "providers": {
    "openai": {
      "apiKey": "sk-openai",
      "models": {
        "m1": { "name": "gpt-4.1-mini", "enabled": false }
      }
    }
  }
}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "at least one enabled model is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsUnsupportedAPIType(t *testing.T) {
	path := writeTempConfig(t, `{
  "providers": {
    "azure_openai": {
      "apiType": "gemini",
      "apiKey": "sk-azure",
      "models": {
        "m1": { "name": "gpt-4.1-mini" }
      }
    }
  }
}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported api_type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadAllowsEmptyAPIKey(t *testing.T) {
	path := writeTempConfig(t, `{
  "providers": {
    "openai": {
      "apiKey": "",
      "models": {
        "m1": { "name": "gpt-4.1-mini" }
      }
    }
  }
}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	model, err := cfg.ResolveModel("m1")
	if err != nil {
		t.Fatalf("ResolveModel() error = %v", err)
	}
	if model.APIKey != "" {
		t.Fatalf("expected empty api key, got %q", model.APIKey)
	}
}

func TestLoadRejectsUnsupportedLogLevel(t *testing.T) {
	path := writeTempConfig(t, `{
  "log": { "level": "trace" },
  "providers": {
    "openai": {
      "apiKey": "sk-test",
      "models": {
        "m1": { "name": "gpt-4.1-mini" }
      }
    }
  }
}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported log.level") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadResolvesProviderProxy(t *testing.T) {
	path := writeTempConfig(t, `{
  "providers": {
    "openai": {
      "apiKey": "sk-test",
      "proxy": "socks://127.0.0.1:1080",
      "models": {
        "m1": { "name": "gpt-4.1-mini" }
      }
    }
  }
}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	model, err := cfg.ResolveModel("m1")
	if err != nil {
		t.Fatalf("ResolveModel() error = %v", err)
	}
	if model.Proxy != "socks5://127.0.0.1:1080" {
		t.Fatalf("proxy normalization mismatch: %q", model.Proxy)
	}
}

func TestLoadRejectsInvalidProviderProxy(t *testing.T) {
	path := writeTempConfig(t, `{
  "providers": {
    "openai": {
      "apiKey": "sk-test",
      "proxy": "ftp://127.0.0.1:21",
      "models": {
        "m1": { "name": "gpt-4.1-mini" }
      }
    }
  }
}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid proxy for provider") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}
