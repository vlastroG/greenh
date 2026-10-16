package sim

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vlastrog/greenh/core"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "sim.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadConfigSpecExample(t *testing.T) {
	cfg, err := LoadConfig("../deploy/simulator.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Backend != "http://waterer:8080" || cfg.Interval.Duration != 30*time.Second ||
		cfg.Speed != 60 || cfg.Backfill.Duration != 48*time.Hour {
		t.Fatalf("unexpected top-level config: %+v", cfg)
	}
	if len(cfg.Nodes) != 1 || cfg.Nodes[0].ID != "windowsill-1" || len(cfg.Nodes[0].Channels) != 3 {
		t.Fatalf("unexpected nodes: %+v", cfg.Nodes)
	}
	c := cfg.Nodes[0].Channels[1]
	if c.Category != core.CategorySucculent || c.PotML != 500 || c.StartMoisture != 20 || c.DryRaw != 530 || c.WetRaw != 265 {
		t.Fatalf("unexpected ch1: %+v", c)
	}
}

func TestLoadConfigDefaultsAndDerivedCalibration(t *testing.T) {
	p := writeConfig(t, `
backend: http://localhost:8080
nodes:
  - id: n1
    channels:
      - { ch: 2, category: herb, pot_ml: 800, start_moisture: 50 }
failures:
  dropout: { node: n1, after: 1m, for: 2m }
`)
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Interval.Duration != 30*time.Second || cfg.Speed != 1 || cfg.Backfill.Duration != 0 {
		t.Fatalf("defaults wrong: %+v", cfg)
	}
	c := cfg.Nodes[0].Channels[0]
	if c.DryRaw != 520+(2*17)%40 || c.WetRaw != 260+(2*11)%30 {
		t.Fatalf("derived calibration wrong: %+v", c)
	}
	if d := cfg.Failures.Dropout; d == nil || d.After.Duration != time.Minute || d.For.Duration != 2*time.Minute {
		t.Fatalf("dropout wrong: %+v", d)
	}
}

func TestLoadConfigInvalid(t *testing.T) {
	cases := map[string]string{
		"unknown category": "nodes: [{id: n, channels: [{ch: 0, category: cactus, pot_ml: 100}]}]",
		"duplicate ch":     "nodes: [{id: n, channels: [{ch: 0, category: herb, pot_ml: 100}, {ch: 0, category: herb, pot_ml: 100}]}]",
		"empty nodes":      "nodes: []",
		"bad pot":          "nodes: [{id: n, channels: [{ch: 0, category: herb, pot_ml: 0}]}]",
		"bad backend":      "backend: waterer\nnodes: [{id: n, channels: [{ch: 0, category: herb, pot_ml: 100}]}]",
	}
	for name, body := range cases {
		if !strings.Contains(body, "backend:") {
			body = "backend: http://x:1\n" + body
		}
		if _, err := LoadConfig(writeConfig(t, body)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
