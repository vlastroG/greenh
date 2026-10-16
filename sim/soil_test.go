package sim

import (
	"math"
	"testing"
	"time"

	"github.com/vlastrog/greenh/core"
)

func amb(temp, hum, lux float64) core.Ambient {
	return core.Ambient{TempC: &temp, HumidityPct: &hum, LightLux: &lux}
}

func TestSoilDriesFasterInDayAndHeat(t *testing.T) {
	day, night := NewSoil(core.CategoryFoliage, 2000, 60), NewSoil(core.CategoryFoliage, 2000, 60)
	day.Step(time.Hour, amb(25, 45, 5000))
	night.Step(time.Hour, amb(19, 55, 0))
	if 60-day.Moisture() <= 60-night.Moisture() {
		t.Errorf("day loss %v should exceed night loss %v", 60-day.Moisture(), 60-night.Moisture())
	}
}

func dryOneDay(c core.Category, ml int) float64 {
	e := NewEnv(3)
	s := NewSoil(c, ml, 80)
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 24*12; i++ {
		tm := start.Add(time.Duration(i) * 5 * time.Minute)
		s.Step(5*time.Minute, e.At(tm))
	}
	return 80 - s.Moisture()
}

func TestSoilCategoryRates(t *testing.T) {
	herb, succ := dryOneDay(core.CategoryHerb, 2000), dryOneDay(core.CategorySucculent, 2000)
	if herb <= succ {
		t.Errorf("herb loss %v should exceed succulent %v", herb, succ)
	}
	if f := dryOneDay(core.CategoryFoliage, 2000); f < 4 || f > 12 {
		t.Errorf("foliage 2000ml loss = %v, want 4..12", f)
	}
}

func TestSoilWateringOvershootThenSettles(t *testing.T) {
	s := NewSoil(core.CategoryFoliage, 2000, 40)
	s.Water(300)
	if s.Moisture() <= 62.5 {
		t.Fatalf("immediate moisture %v, want > 62.5", s.Moisture())
	}
	for i := 0; i < 90; i++ {
		s.Step(time.Minute, amb(22, 50, 0))
	}
	if math.Abs(s.Moisture()-62.5) > 3 {
		t.Errorf("settled moisture %v, want ~62.5", s.Moisture())
	}
}

func TestSoilStaysInRange(t *testing.T) {
	s := NewSoil(core.CategoryHerb, 500, 50)
	s.Water(5000)
	if s.Moisture() > 100 {
		t.Errorf("moisture %v > 100", s.Moisture())
	}
	for i := 0; i < 60*24; i++ {
		s.Step(time.Hour, amb(25, 40, 5000))
	}
	if s.Moisture() < 0 {
		t.Errorf("moisture %v < 0", s.Moisture())
	}
}
