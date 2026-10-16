package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig(env(map[string]string{"ADVISOR_API_KEY": "k"}))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		Addr: ":8080", DBPath: "/data/waterer.db",
		OfflineAfter: 30 * time.Minute, AdvisorTimeout: 30 * time.Second,
		Provider: "openaicompat", BaseURL: "https://api.deepseek.com",
		Model: "deepseek-chat", APIKey: "k",
	}
	if cfg.Addr != want.Addr || cfg.DBPath != want.DBPath ||
		cfg.OfflineAfter != want.OfflineAfter || cfg.AdvisorTimeout != want.AdvisorTimeout ||
		cfg.Provider != want.Provider || cfg.BaseURL != want.BaseURL ||
		cfg.Model != want.Model || cfg.APIKey != want.APIKey {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
	if len(cfg.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", cfg.Warnings)
	}
}

func TestLoadConfigNoKeyFallsBackToRules(t *testing.T) {
	cfg, err := LoadConfig(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "rules" {
		t.Fatalf("provider = %q, want rules", cfg.Provider)
	}
	if len(cfg.Warnings) == 0 {
		t.Fatal("want a warning")
	}
}

func TestLoadConfigInvalid(t *testing.T) {
	_, err := LoadConfig(env(map[string]string{"OFFLINE_AFTER": "soon"}))
	if err == nil || !strings.Contains(err.Error(), "OFFLINE_AFTER") {
		t.Fatalf("err = %v, want one naming OFFLINE_AFTER", err)
	}
	_, err = LoadConfig(env(map[string]string{"ADVISOR_PROVIDER": "gpt"}))
	if err == nil || !strings.Contains(err.Error(), "ADVISOR_PROVIDER") {
		t.Fatalf("err = %v, want one naming ADVISOR_PROVIDER", err)
	}
}

func TestConfigLogValueHidesKey(t *testing.T) {
	cfg, err := LoadConfig(env(map[string]string{"ADVISOR_API_KEY": "sk-secret-123"}))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("config", "config", cfg)
	out := buf.String()
	if strings.Contains(out, "sk-secret-123") {
		t.Fatalf("key leaked: %s", out)
	}
	if !strings.Contains(out, `"set"`) {
		t.Fatalf("want key rendered as set: %s", out)
	}
}
