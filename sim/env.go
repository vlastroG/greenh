// Package sim models the physical world the simulated node observes.
package sim

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/vlastrog/greenh/core"
)

// Env produces ambient conditions for a simulated time.
type Env struct {
	seed int64
	rng  *rand.Rand
}

func NewEnv(seed int64) *Env {
	return &Env{seed: seed, rng: rand.New(rand.NewPCG(uint64(seed), 0))}
}

// At returns ambient conditions at t, using the UTC hour of day.
func (e *Env) At(t time.Time) core.Ambient {
	t = t.UTC()
	h := float64(t.Hour()) + float64(t.Minute())/60 + float64(t.Second())/3600

	light := 5000 * math.Max(0, math.Sin(math.Pi*(h-6)/12)) * e.cloud(t.Unix()/3600)
	temp := 22 + 3*math.Sin(math.Pi*(h-9)/12) + (e.rng.Float64()*2-1)*0.3
	hum := 50 - 4*(temp-22) + (e.rng.Float64()*2-1)*1
	hum = math.Min(100, math.Max(0, hum))

	return core.Ambient{TempC: &temp, HumidityPct: &hum, LightLux: &light}
}

// cloud is derived from the hour index so it is stable within the hour.
func (e *Env) cloud(hourIdx int64) float64 {
	r := rand.New(rand.NewPCG(uint64(e.seed), uint64(hourIdx)))
	if r.Float64() < 0.1 {
		return 0.3 + 0.4*r.Float64()
	}
	return 1
}
