package main

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	providerOpenAICompat = "openaicompat"
	providerRules        = "rules"
)

type Config struct {
	Addr, DBPath                     string
	OfflineAfter, AdvisorTimeout     time.Duration
	Provider, BaseURL, Model, APIKey string
	Warnings                         []string
}

// LoadConfig reads settings through getenv. An openaicompat provider without
// a key is downgraded to rules, with a warning.
func LoadConfig(rawGetenv func(string) string) (Config, error) {
	// trims stray spaces and CRs copied into .env, which would break the auth header
	getenv := func(key string) string { return strings.TrimSpace(rawGetenv(key)) }
	str := func(key, def string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return def
	}
	cfg := Config{
		Addr:     str("WATERER_ADDR", ":8080"),
		DBPath:   str("WATERER_DB", "/data/waterer.db"),
		Provider: str("ADVISOR_PROVIDER", providerOpenAICompat),
		BaseURL:  str("ADVISOR_BASE_URL", "https://api.deepseek.com"),
		Model:    str("ADVISOR_MODEL", "deepseek-chat"),
		APIKey:   getenv("ADVISOR_API_KEY"),
	}
	var err error
	if cfg.OfflineAfter, err = duration(getenv, "OFFLINE_AFTER", 30*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.AdvisorTimeout, err = duration(getenv, "ADVISOR_TIMEOUT", 30*time.Second); err != nil {
		return Config{}, err
	}
	switch cfg.Provider {
	case providerRules:
	case providerOpenAICompat:
		if cfg.APIKey == "" {
			cfg.Provider = providerRules
			cfg.Warnings = append(cfg.Warnings, "ADVISOR_API_KEY is not set, using the rules advisor")
		}
	default:
		return Config{}, fmt.Errorf("ADVISOR_PROVIDER: unknown provider %q (want %s or %s)",
			cfg.Provider, providerOpenAICompat, providerRules)
	}
	return cfg, nil
}

func duration(getenv func(string) string, key string, def time.Duration) (time.Duration, error) {
	v := getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s: %q is not a positive duration", key, v)
	}
	return d, nil
}

// LogValue renders the config without exposing the API key.
func (c Config) LogValue() slog.Value {
	key := "unset"
	if c.APIKey != "" {
		key = "set"
	}
	return slog.GroupValue(
		slog.String("addr", c.Addr),
		slog.String("db", c.DBPath),
		slog.String("offline_after", c.OfflineAfter.String()),
		slog.String("advisor_timeout", c.AdvisorTimeout.String()),
		slog.String("provider", c.Provider),
		slog.String("base_url", c.BaseURL),
		slog.String("model", c.Model),
		slog.String("api_key", key),
	)
}
