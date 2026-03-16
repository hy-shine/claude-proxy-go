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
      "api_key": "sk-test",
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
		t.Fatalf("openai base_url default mismatch: %q", got)
	}

	model, err := cfg.ResolveModel("m1")
	if err != nil {
		t.Fatalf("ResolveModel() error = %v", err)
	}
	if model.Provider != "openai" || model.Name != "gpt-4.1-mini" {
		t.Fatalf("resolved model mismatch: %#v", model)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := writeTempConfig(t, `{
  "providers": {
    "openai": {
      "api_key": "sk-test",
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

func TestLoadRejectsNonOpenAIProvider(t *testing.T) {
	path := writeTempConfig(t, `{
  "providers": {
    "google": {
      "api_key": "sk-google",
      "models": { "m1": { "name": "gemini-2.5-pro" } }
    }
  }
}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported provider") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsWhenAllModelsDisabled(t *testing.T) {
	path := writeTempConfig(t, `{
  "providers": {
    "openai": {
      "api_key": "sk-openai",
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

func TestLoadRejectsUnsupportedProvider(t *testing.T) {
	path := writeTempConfig(t, `{
  "providers": {
    "azure_openai": {
      "api_key": "sk-azure",
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
	if !strings.Contains(err.Error(), "unsupported provider") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadAllowsEmptyAPIKey(t *testing.T) {
	path := writeTempConfig(t, `{
  "providers": {
    "openai": {
      "api_key": "",
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

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}
