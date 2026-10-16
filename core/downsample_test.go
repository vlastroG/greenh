package core

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func fptr(v float64) *float64 { return &v }

func TestDownsample(t *testing.T) {
	base := time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)
	points := []HistoryPoint{
		{TS: base.Add(5 * time.Minute), MoistureRaw: fptr(400), MoisturePct: fptr(40), TempC: fptr(20)},
		{TS: base.Add(35 * time.Minute), MoistureRaw: fptr(500), MoisturePct: fptr(30), TempC: nil, LightLux: fptr(100)},
		{TS: base.Add(65 * time.Minute), MoistureRaw: fptr(600)},
	}

	t.Run("means per bucket, nil fields ignored", func(t *testing.T) {
		got := Downsample(points, time.Hour)
		want := []HistoryPoint{
			{TS: base, MoistureRaw: fptr(450), MoisturePct: fptr(35), TempC: fptr(20), LightLux: fptr(100)},
			{TS: base.Add(time.Hour), MoistureRaw: fptr(600)},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %s, want %s", dump(got), dump(want))
		}
	})

	t.Run("buckets aligned to the unix epoch", func(t *testing.T) {
		got := Downsample(points[:1], 7*time.Second)
		if ns := got[0].TS.UnixNano(); ns%int64(7*time.Second) != 0 {
			t.Fatalf("bucket start %v not aligned to 7s since epoch", got[0].TS)
		}
		if got[0].TS.After(points[0].TS) || points[0].TS.Sub(got[0].TS) >= 7*time.Second {
			t.Fatalf("bucket start %v does not contain %v", got[0].TS, points[0].TS)
		}
	})

	t.Run("step 0 returns input unchanged", func(t *testing.T) {
		if got := Downsample(points, 0); !reflect.DeepEqual(got, points) {
			t.Fatalf("got %s", dump(got))
		}
	})
}

func dump(ps []HistoryPoint) string {
	s := ""
	for _, p := range ps {
		s += p.TS.Format(time.RFC3339) + "{"
		for _, f := range []*float64{p.MoistureRaw, p.MoisturePct, p.TempC, p.HumidityPct, p.LightLux} {
			if f == nil {
				s += "nil "
			} else {
				s += fmt.Sprint(*f) + " "
			}
		}
		s += "} "
	}
	return s
}
