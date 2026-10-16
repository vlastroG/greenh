package sim

import (
	"math"
	"math/rand/v2"
)

// Sensor is a capacitive soil probe: drier soil gives a higher raw value.
type Sensor struct {
	DryRaw, WetRaw int
	rng            *rand.Rand
}

func NewSensor(dryRaw, wetRaw int, seed int64) *Sensor {
	return &Sensor{DryRaw: dryRaw, WetRaw: wetRaw, rng: rand.New(rand.NewPCG(uint64(seed), 0))}
}

// Raw converts moisture percent to a noisy ADC value clamped to 0..4095.
func (s *Sensor) Raw(pct float64) int {
	span := float64(s.DryRaw - s.WetRaw)
	raw := float64(s.DryRaw) - pct/100*span + (s.rng.Float64()*2-1)*0.015*span
	return int(math.Min(4095, math.Max(0, math.Round(raw))))
}
