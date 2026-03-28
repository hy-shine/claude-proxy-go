package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Server    ServerConfig              `json:"server"`
	Log       LogConfig                 `json:"log"`
	Providers map[string]ProviderConfig `json:"providers"`
	Retry     RetryConfig               `json:"retry"`
	Timeout   TimeoutConfig             `json:"timeout"`

	modelIndex map[string]ResolvedModel
}

type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type LogConfig struct {
	Level  string `json:"level"`
	Format string `json:"format,omitempty"` // "text" (default) or "json"
}

type ProviderConfig struct {
	APIType       string                 `json:"apiType,omitempty"`
	APIKey        string                 `json:"apiKey"`
	BaseURL       string                 `json:"baseUrl"`
	Proxy         string                 `json:"proxy,omitempty"`
	CustomHeaders map[string]string      `json:"customHeaders,omitempty"`
	Models        map[string]ModelConfig `json:"models"`
}

type ModelConfig struct {
	Name      string `json:"name"`
	Enabled   *bool  `json:"enabled,omitempty"`
	MaxTokens int    `json:"maxTokens,omitempty"`
}

type RetryConfig struct {
	MaxRetries       int `json:"maxRetries"`
	InitialBackoffMS int `json:"initialBackoffMs"`
	MaxBackoffMS     int `json:"maxBackoffMs"`
}

type TimeoutConfig struct {
	RequestTimeout int `json:"requestTimeout"`
	StreamTimeout  int `json:"streamTimeout"`
}

type ResolvedModel struct {
	ModelID       string
	Provider      string
	APIType       string
	Name          string
	APIKey        string
	BaseURL       string
	Proxy         string
	MaxTokens     int
	CustomHeaders map[string]string
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("failed to read config file: " + err.Error())
	}

	var cfg Config
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&cfg); err != nil {
		return nil, errors.New("failed to parse config file: " + err.Error())
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return nil, errors.New("failed to parse config file: multiple JSON values are not allowed")
	}

	cfg.ApplyDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) ApplyDefaults() {
	if c.Providers == nil {
		c.Providers = make(map[string]ProviderConfig)
	}
	for providerID, provider := range c.Providers {
		provider.APIType = normalizeAPIType(provider.APIType)
		if provider.APIType == "" {
			provider.APIType = "openai"
		}
		provider.Proxy = normalizeProxy(provider.Proxy)
		if provider.APIType == "openai" && provider.BaseURL == "" {
			provider.BaseURL = "https://api.openai.com/v1"
		}
		if provider.Models == nil {
			provider.Models = make(map[string]ModelConfig)
		}
		c.Providers[providerID] = provider
	}

	if c.Retry.MaxRetries == 0 {
		c.Retry.MaxRetries = 2
	}
	if c.Retry.InitialBackoffMS == 0 {
		c.Retry.InitialBackoffMS = 200
	}
	if c.Retry.MaxBackoffMS == 0 {
		c.Retry.MaxBackoffMS = 800
	}
	if c.Timeout.RequestTimeout == 0 {
		c.Timeout.RequestTimeout = 60
	}
	if c.Timeout.StreamTimeout == 0 {
		c.Timeout.StreamTimeout = 300
	}

	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8082
	}

	if normalizeAPIType(c.Log.Level) == "" {
		c.Log.Level = "info"
	}
	if c.Log.Format == "" {
		c.Log.Format = "text"
	}
}

func (c *Config) Validate() error {
	if len(c.Providers) == 0 {
		return errors.New("providers is required and cannot be empty")
	}

	if c.Retry.MaxRetries < 0 {
		return errors.New("retry.maxRetries cannot be negative")
	}
	switch normalizeAPIType(c.Log.Level) {
	case "debug", "info", "warn", "warning", "error":
	default:
		return fmt.Errorf("unsupported log.level: %s", c.Log.Level)
	}
	switch normalizeAPIType(c.Log.Format) {
	case "", "text", "json":
	default:
		return fmt.Errorf("unsupported log.format: %s", c.Log.Format)
	}

	validAPITypes := map[string]bool{
		"openai": true,
	}

	seenModelIDs := make(map[string]string)
	modelIndex := make(map[string]ResolvedModel)
	enabledCount := 0

	for providerID, provider := range c.Providers {
		if strings.TrimSpace(providerID) == "" {
			return errors.New("providers contains empty provider name")
		}

		apiType := normalizeAPIType(provider.APIType)
		if apiType == "" {
			apiType = "openai"
		}
		if !validAPITypes[apiType] {
			return fmt.Errorf("unsupported api_type %q for provider %q", provider.APIType, providerID)
		}
		if err := validateProxy(provider.Proxy); err != nil {
			return fmt.Errorf("invalid proxy for provider %q: %w", providerID, err)
		}
		if len(provider.Models) == 0 {
			return fmt.Errorf("providers.%s.models cannot be empty", providerID)
		}

		for modelID, modelCfg := range provider.Models {
			if modelID == "" {
				return fmt.Errorf("providers.%s.models contains empty model_id", providerID)
			}
			if prevProvider, exists := seenModelIDs[modelID]; exists {
				return fmt.Errorf("model_id %q is duplicated in providers %q and %q", modelID, prevProvider, providerID)
			}
			seenModelIDs[modelID] = providerID

			if modelCfg.Name == "" {
				return fmt.Errorf("providers.%s.models.%s.name is required", providerID, modelID)
			}

			if modelCfg.isEnabled() {
				enabledCount++
				modelIndex[modelID] = ResolvedModel{
					ModelID:       modelID,
					Provider:      providerID,
					APIType:       apiType,
					Name:          modelCfg.Name,
					APIKey:        provider.APIKey,
					BaseURL:       provider.BaseURL,
					Proxy:         provider.Proxy,
					MaxTokens:     modelCfg.MaxTokens,
					CustomHeaders: provider.CustomHeaders,
				}
			}
		}
	}

	if enabledCount == 0 {
		return errors.New("at least one enabled model is required")
	}

	c.modelIndex = modelIndex
	return nil
}

func (m ModelConfig) isEnabled() bool {
	if m.Enabled == nil {
		return true
	}
	return *m.Enabled
}

func (c *Config) ResolveModel(modelID string) (ResolvedModel, error) {
	if modelID == "" {
		return ResolvedModel{}, errors.New("model is required")
	}
	resolved, ok := c.modelIndex[modelID]
	if !ok {
		return ResolvedModel{}, fmt.Errorf("unknown model id: %s", modelID)
	}
	return resolved, nil
}

func (c *Config) EnabledModels() map[string]ResolvedModel {
	result := make(map[string]ResolvedModel, len(c.modelIndex))
	for modelID, info := range c.modelIndex {
		result[modelID] = info
	}
	return result
}

func normalizeAPIType(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func normalizeProxy(v string) string {
	trimmed := strings.TrimSpace(v)
	if strings.HasPrefix(strings.ToLower(trimmed), "socks://") {
		return "socks5://" + trimmed[len("socks://"):]
	}
	return trimmed
}

func validateProxy(v string) error {
	if v == "" {
		return nil
	}
	parsed, err := url.Parse(v)
	if err != nil {
		return fmt.Errorf("failed to parse proxy url: %w", err)
	}
	if parsed.Scheme == "" {
		return errors.New("proxy url scheme is required")
	}
	if parsed.Host == "" {
		return errors.New("proxy url host is required")
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "socks", "socks5", "socks5h":
		return nil
	default:
		return fmt.Errorf("unsupported proxy scheme: %s", parsed.Scheme)
	}
}
