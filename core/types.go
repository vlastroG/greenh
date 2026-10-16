package core

import (
	"encoding/json"
	"time"
)

type Category string

const (
	CategorySucculent Category = "succulent"
	CategoryFoliage   Category = "foliage"
	CategoryFlowering Category = "flowering"
	CategoryHerb      Category = "herb"
)

type Status string

const (
	StatusOK               Status = "ok"
	StatusSensorOffline    Status = "sensor_offline"
	StatusNeedsCalibration Status = "needs_calibration"
)

type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

type Trigger string

const (
	TriggerScheduled Trigger = "scheduled"
	TriggerManual    Trigger = "manual"
)

type Ambient struct{ TempC, HumidityPct, LightLux *float64 }

type ChannelReading struct{ Ch, MoistureRaw int }

type Reading struct {
	NodeID   string
	TS       time.Time
	Ambient  Ambient
	Channels []ChannelReading
}

// RawPoint is one channel's sample joined with its node's ambient.
type RawPoint struct {
	TS          time.Time
	MoistureRaw int
	Ambient     Ambient
}

type Rules struct{ ThresholdPct, AmountFraction float64 }

type Pot struct {
	ID                  int64
	Name, Species       string
	Category            Category
	PotML               int
	SoilType, Placement string
	NodeID              *string
	Channel             *int
	DryRaw, WetRaw      *int
	Rules               Rules
	CheckInterval       time.Duration
	CreatedAt           time.Time
}

// Calibrated reports whether both calibration points are set.
func (p Pot) Calibrated() bool { return p.DryRaw != nil && p.WetRaw != nil }

// MoisturePct converts a raw sample; ok is false when the pot is not calibrated.
func (p Pot) MoisturePct(raw int) (float64, bool) {
	if !p.Calibrated() {
		return 0, false
	}
	return MoisturePct(raw, *p.DryRaw, *p.WetRaw), true
}

type Watering struct {
	ID, PotID     int64
	TS, CreatedAt time.Time
	AmountML      int
	Source        string
}

// WateringLink is a watering with the pot's current node link.
type WateringLink struct {
	Watering
	NodeID  *string
	Channel *int
}

type Recommendation struct {
	NeedsWater bool       `json:"needs_water"`
	AmountML   int        `json:"amount_ml"`
	Confidence Confidence `json:"confidence"`
	Reason     string     `json:"reason"`
}

type ProviderInfo struct{ Provider, Model, PromptVersion string }

type Advice struct {
	ID, PotID      int64
	CreatedAt      time.Time
	Trigger        Trigger
	Status         Status
	Recommendation *Recommendation // nil unless Status==ok
	Rules          *Recommendation // nil unless Status==ok
	ProviderInfo
	Fallback       bool
	FallbackReason string
	Disagree       bool
	Summary        json.RawMessage // exact input summary, nil unless Status==ok
}

type NodeChannel struct {
	Ch      int
	LastRaw *int
	LastAt  *time.Time
	PotID   *int64
}

type Node struct {
	ID, Name string
	LastSeen time.Time
	Channels []NodeChannel
}
