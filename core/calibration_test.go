package core

import (
	"math"
	"testing"
)

func TestMoisturePct(t *testing.T) {
	cases := []struct {
		raw, dry, wet int
		want          float64
	}{
		{540, 540, 270, 0}, {270, 540, 270, 100}, {405, 540, 270, 50},
		{600, 540, 270, 0}, {200, 540, 270, 100}, // beyond calibration clamps
		{405, 270, 540, 50}, {540, 270, 540, 100}, // reversed calibration still works
	}
	for _, c := range cases {
		if got := MoisturePct(c.raw, c.dry, c.wet); math.Abs(got-c.want) > 0.01 {
			t.Errorf("%+v: got %v", c, got)
		}
	}
}

func TestPotMoisturePctUncalibrated(t *testing.T) {
	if _, ok := (Pot{}).MoisturePct(400); ok {
		t.Fatal("want !ok")
	}
}

func TestPotMoisturePctCalibrated(t *testing.T) {
	dry, wet := 540, 270
	p := Pot{DryRaw: &dry, WetRaw: &wet}
	if !p.Calibrated() {
		t.Fatal("want calibrated")
	}
	got, ok := p.MoisturePct(405)
	if !ok || math.Abs(got-50) > 0.01 {
		t.Fatalf("got %v, %v", got, ok)
	}
}
