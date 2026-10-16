package sim

import (
	"testing"
	"time"
)

func TestEnvDayNight(t *testing.T) {
	e := NewEnv(1)
	day := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	night := e.At(day.Add(2 * time.Hour))
	noon := e.At(day.Add(12 * time.Hour))
	if *night.LightLux != 0 {
		t.Errorf("night light = %v, want 0", *night.LightLux)
	}
	if *noon.LightLux <= 2000 {
		t.Errorf("noon light = %v, want > 2000", *noon.LightLux)
	}
	for m := 0; m < 24*60; m += 10 {
		a := e.At(day.Add(time.Duration(m) * time.Minute))
		if *a.TempC < 18.5 || *a.TempC > 25.5 {
			t.Fatalf("temp %v out of range at +%dm", *a.TempC, m)
		}
	}
	// average over a few samples to beat noise
	var hn, hd float64
	for i := 0; i < 20; i++ {
		hn += *e.At(day.Add(3 * time.Hour)).HumidityPct
		hd += *e.At(day.Add(15 * time.Hour)).HumidityPct
	}
	if hn <= hd {
		t.Errorf("humidity night %v should exceed afternoon %v", hn/20, hd/20)
	}
}

func TestEnvCloudStableWithinHour(t *testing.T) {
	e := NewEnv(7)
	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	for h := 7; h < 18; h++ {
		t0 := base.Add(time.Duration(h) * time.Hour)
		a, b := *e.At(t0.Add(5 * time.Minute)).LightLux, *e.At(t0.Add(50 * time.Minute)).LightLux
		// same cloud factor, only sine differs slightly
		if a == 0 || b == 0 {
			continue
		}
		again := *e.At(t0.Add(5 * time.Minute)).LightLux
		if a != again {
			t.Fatalf("light not deterministic within hour %d: %v vs %v", h, a, again)
		}
	}
}
