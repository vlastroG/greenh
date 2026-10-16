package sim

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/vlastrog/greenh/core"
)

// Duration is a time.Duration that unmarshals from YAML strings like "30s".
type Duration struct{ time.Duration }

func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	var s string
	if err := n.Decode(&s); err != nil {
		return err
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = v
	return nil
}

type Config struct {
	Backend  string       `yaml:"backend"`
	Interval Duration     `yaml:"interval"`
	Speed    float64      `yaml:"speed"`
	Backfill Duration     `yaml:"backfill"`
	Seed     int64        `yaml:"seed"`
	Nodes    []NodeConfig `yaml:"nodes"`
	Failures Failures     `yaml:"failures"`
}

type NodeConfig struct {
	ID       string          `yaml:"id"`
	Channels []ChannelConfig `yaml:"channels"`
}

type ChannelConfig struct {
	Ch            int           `yaml:"ch"`
	Category      core.Category `yaml:"category"`
	PotML         int           `yaml:"pot_ml"`
	StartMoisture float64       `yaml:"start_moisture"`
	DryRaw        int           `yaml:"dry_raw"`
	WetRaw        int           `yaml:"wet_raw"`
}

type Failures struct {
	Dropout    *DropoutFailure    `yaml:"dropout"`
	Stuck      *StuckFailure      `yaml:"stuck"`
	OutOfRange *OutOfRangeFailure `yaml:"out_of_range"`
}

// DropoutFailure silences a node for For, starting After the simulator starts (real time).
type DropoutFailure struct {
	Node  string   `yaml:"node"`
	After Duration `yaml:"after"`
	For   Duration `yaml:"for"`
}

type StuckFailure struct {
	Node string `yaml:"node"`
	Ch   int    `yaml:"ch"`
}

// OutOfRangeFailure makes every Nth reading of a channel carry a garbage raw value.
type OutOfRangeFailure struct {
	Node  string `yaml:"node"`
	Ch    int    `yaml:"ch"`
	Every int    `yaml:"every"`
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{Interval: Duration{30 * time.Second}, Speed: 1}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := cfg.normalize(); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// normalize validates the config and fills derived calibration values.
func (c *Config) normalize() error {
	u, err := url.Parse(c.Backend)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("invalid backend URL %q", c.Backend)
	}
	if c.Interval.Duration <= 0 {
		return fmt.Errorf("interval must be positive")
	}
	if c.Speed <= 0 {
		return fmt.Errorf("speed must be positive")
	}
	if c.Backfill.Duration < 0 {
		return fmt.Errorf("backfill must not be negative")
	}
	if len(c.Nodes) == 0 {
		return fmt.Errorf("at least one node is required")
	}
	ids := map[string]bool{}
	for i := range c.Nodes {
		n := &c.Nodes[i]
		if n.ID == "" || ids[n.ID] {
			return fmt.Errorf("node id %q is empty or duplicated", n.ID)
		}
		ids[n.ID] = true
		if len(n.Channels) == 0 {
			return fmt.Errorf("node %s has no channels", n.ID)
		}
		seen := map[int]bool{}
		for j := range n.Channels {
			ch := &n.Channels[j]
			if seen[ch.Ch] {
				return fmt.Errorf("node %s: duplicate channel %d", n.ID, ch.Ch)
			}
			seen[ch.Ch] = true
			if !ch.Category.Valid() {
				return fmt.Errorf("node %s ch %d: unknown category %q", n.ID, ch.Ch, ch.Category)
			}
			if ch.PotML <= 0 {
				return fmt.Errorf("node %s ch %d: pot_ml must be positive", n.ID, ch.Ch)
			}
			if ch.DryRaw == 0 {
				ch.DryRaw = 520 + (ch.Ch*17)%40
			}
			if ch.WetRaw == 0 {
				ch.WetRaw = 260 + (ch.Ch*11)%30
			}
		}
	}
	if f := c.Failures.OutOfRange; f != nil && f.Every <= 0 {
		return fmt.Errorf("failures.out_of_range.every must be positive")
	}
	return nil
}
