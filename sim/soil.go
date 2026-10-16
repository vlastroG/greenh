package sim

import (
	"math"
	"time"

	"github.com/vlastrog/greenh/core"
)

// baseDryPctPerDay is the drying rate at 22 °C in average daylight for a 1000 ml pot.
var baseDryPctPerDay = map[core.Category]float64{
	core.CategorySucculent: 4,
	core.CategoryFoliage:   8,
	core.CategoryFlowering: 10,
	core.CategoryHerb:      12,
}

const excessDrainTau = 20 * time.Minute

// Soil models moisture of one pot, including the transient excess after watering.
type Soil struct {
	moisture, excess float64
	category         core.Category
	potML            int
}

func NewSoil(c core.Category, potML int, startPct float64) *Soil {
	return &Soil{moisture: clamp(startPct), category: c, potML: potML}
}

// Step advances the soil by dt of simulated time.
func (s *Soil) Step(dt time.Duration, a core.Ambient) {
	temp, hum, light := deref(a.TempC, 22), deref(a.HumidityPct, 50), deref(a.LightLux, 0)

	rate := baseDryPctPerDay[s.category] * math.Pow(1000/float64(s.potML), 0.3)
	factor := math.Max(0, 1+0.05*(temp-22)+light/10000+(50-hum)/100)
	if light == 0 {
		factor /= 2
	}
	s.moisture -= rate * factor * dt.Hours() / 24

	drain := s.excess * (1 - math.Exp(-float64(dt)/float64(excessDrainTau)))
	s.excess -= drain
	s.moisture -= drain
	s.moisture = clamp(s.moisture)
}

// Water adds ml of water; the surplus drains out over the following hour.
func (s *Soil) Water(ml int) {
	rise := float64(ml) / float64(s.potML) * 150
	before := s.moisture
	s.moisture = clamp(s.moisture + rise*1.3)
	s.excess = math.Min(rise*0.3, s.moisture-before)
}

func (s *Soil) Moisture() float64 { return s.moisture }

func clamp(v float64) float64 { return math.Min(100, math.Max(0, v)) }

func deref(p *float64, def float64) float64 {
	if p == nil {
		return def
	}
	return *p
}
