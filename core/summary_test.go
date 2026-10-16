package core

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

func testPot() Pot {
	return Pot{
		ID: 7, Name: "Kitchen Monstera", Species: "Monstera", Category: CategoryFoliage,
		PotML: 2000, SoilType: "peat mix", Placement: "window",
		NodeID: ptr("node-secret"), Channel: ptr(1), DryRaw: ptr(540), WetRaw: ptr(270),
	}
}

// series builds points from percentages; pct 100 maps to raw 270, pct 0 to raw 540.
func series(start time.Time, step time.Duration, pcts ...float64) []RawPoint {
	pts := make([]RawPoint, len(pcts))
	for i, p := range pcts {
		pts[i] = RawPoint{TS: start.Add(time.Duration(i) * step), MoistureRaw: int(math.Round(540 - p/100*270))}
	}
	return pts
}

func linear(from, to float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = from + (to-from)*float64(i)/float64(n-1)
	}
	return out
}

func near(t *testing.T, got *float64, want, tol float64) {
	t.Helper()
	if got == nil || math.Abs(*got-want) > tol {
		t.Errorf("got %v, want %v ±%v", got, want, tol)
	}
}

func TestSummarizeNowAndProfile(t *testing.T) {
	pts := series(t0.Add(-2*time.Hour), time.Hour, 50, 40, 30)
	s := Summarize(SummaryInput{Pot: testPot(), Now: t0, Points: pts})
	if math.Abs(s.Now.MoisturePct-30) > 0.3 {
		t.Error(s.Now.MoisturePct)
	}
	if s.Pot != (SummaryPot{"Monstera", CategoryFoliage, 2000, "peat mix", "window"}) {
		t.Errorf("%+v", s.Pot)
	}
	b, _ := json.Marshal(s)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	for _, k := range []string{"name", "node_id", "id", "channel", "pot_id"} {
		for _, sub := range []string{"pot", "now"} {
			if _, ok := m[sub].(map[string]any)[k]; ok {
				t.Errorf("%s.%s leaked", sub, k)
			}
		}
		if _, ok := m[k]; ok {
			t.Errorf("%s leaked", k)
		}
	}
	if strings.Contains(string(b), "Kitchen") || strings.Contains(string(b), "node-secret") {
		t.Error(string(b))
	}
	if m["last_watering"] != nil {
		t.Error("last_watering should be null")
	}
}

func TestSummarizeMissingAmbient(t *testing.T) {
	pts := series(t0.Add(-2*time.Hour), time.Hour, 50, 40, 30)
	s := Summarize(SummaryInput{Pot: testPot(), Now: t0, Points: pts})
	if s.Now.TempC != nil || s.Trend.LightLuxHours24h != 0 {
		t.Errorf("%+v", s)
	}
	b, _ := json.Marshal(s)
	if !strings.Contains(string(b), `"temp_c":null`) || !strings.Contains(string(b), `"last_watering":null`) {
		t.Error(string(b))
	}
}

func TestSummarizeLight(t *testing.T) {
	pts := series(t0.Add(-210*time.Minute), time.Hour, 50, 50, 50, 50)
	for i := range pts {
		pts[i].Ambient.LightLux = ptr(1000.0)
	}
	pts[1].Ambient.LightLux = nil
	s := Summarize(SummaryInput{Pot: testPot(), Now: t0, Points: pts})
	if s.Trend.LightLuxHours24h != 3000 {
		t.Error(s.Trend.LightLuxHours24h)
	}
	if s.Now.LightLux == nil || *s.Now.LightLux != 1000 {
		t.Error(s.Now.LightLux)
	}
}

func TestSummarizeHourlyBuckets(t *testing.T) {
	pts := series(t0.Add(-10*time.Hour).Add(time.Minute), 30*time.Minute, linear(50, 40, 20)...)
	s := Summarize(SummaryInput{Pot: testPot(), Now: t0, Points: pts})
	h := s.Trend.HourlyMoisturePct
	if len(h) != 48 {
		t.Fatal(len(h))
	}
	for i := 0; i < 38; i++ {
		if h[i] != nil {
			t.Fatalf("bucket %d not nil", i)
		}
	}
	if h[47] == nil || h[38] == nil {
		t.Error("recent buckets empty")
	}
}

func TestSummarizeDryingRate(t *testing.T) {
	pts := series(t0.Add(-24*time.Hour), time.Hour, linear(60, 54, 25)...)
	s := Summarize(SummaryInput{Pot: testPot(), Now: t0, Points: pts})
	near(t, s.Trend.DryingRatePctPerDay, 6.0, 0.1)
}

func TestSummarizeDryingRateTooShort(t *testing.T) {
	pts := series(t0.Add(-time.Hour), 30*time.Minute, 50, 49, 48)
	s := Summarize(SummaryInput{Pot: testPot(), Now: t0, Points: pts})
	if s.Trend.DryingRatePctPerDay != nil {
		t.Error(*s.Trend.DryingRatePctPerDay)
	}
}

func TestSummarizeDryingRateIgnoresWateringJump(t *testing.T) {
	start := t0.Add(-24 * time.Hour)
	pts := series(start, time.Hour, linear(30, 20, 13)...) // 12h
	wTS := start.Add(12*time.Hour + 10*time.Minute)
	pts = append(pts, series(start.Add(13*time.Hour), time.Hour, linear(50, 47, 12)...)...) // 11h
	s := Summarize(SummaryInput{Pot: testPot(), Now: t0, Points: pts, LastWatering: &Watering{TS: wTS, AmountML: 300}})
	near(t, s.Trend.DryingRatePctPerDay, 6.5, 0.5)
}

func TestSummarizeLastWatering(t *testing.T) {
	start := t0.Add(-6 * time.Hour)
	w := &Watering{TS: start.Add(2*time.Hour + 30*time.Minute), AmountML: 400}
	// 18 is the last point before TS; 60 is 30m after TS, 58 is 1h30m after
	pts := series(start, time.Hour, 20, 19, 18, 60, 58, 56, 55)
	s := Summarize(SummaryInput{Pot: testPot(), Now: t0, Points: pts, LastWatering: w})
	lw := s.LastWatering
	if lw == nil {
		t.Fatal("nil")
	}
	if lw.HoursAgo != 3.5 || lw.AmountML != 400 {
		t.Errorf("%+v", lw)
	}
	near(t, lw.MoistureBeforePct, 18, 0.3)
	near(t, lw.MoistureAfterPct, 60, 0.3) // equidistant from TS+1h; the first wins
}

func TestSummarizeLastWateringMissingBefore(t *testing.T) {
	w := &Watering{TS: t0.Add(-time.Hour), AmountML: 100}
	pts := series(t0.Add(-30*time.Minute), 10*time.Minute, 50, 49)
	s := Summarize(SummaryInput{Pot: testPot(), Now: t0, Points: pts, LastWatering: w})
	if s.LastWatering.MoistureBeforePct != nil {
		t.Error("before should be nil")
	}
	near(t, s.LastWatering.MoistureAfterPct, 49, 0.3) // closest to TS+1h
}

func TestSummarizeDataQualityGap(t *testing.T) {
	pts := series(t0.Add(-10*time.Hour), time.Hour, 50, 49, 48)
	pts = append(pts, series(t0.Add(-5*time.Hour), time.Hour, 47, 46, 45, 44, 43, 42)...)
	s := Summarize(SummaryInput{Pot: testPot(), Now: t0, Points: pts})
	found := false
	for _, q := range s.DataQuality {
		if q == "gap of 3 h in readings" {
			found = true
		}
	}
	if !found {
		t.Errorf("%v", s.DataQuality)
	}
}

func TestSummarizeShortHistory(t *testing.T) {
	pts := series(t0.Add(-5*time.Hour), time.Hour, 50, 49, 48, 47, 46, 45)
	s := Summarize(SummaryInput{Pot: testPot(), Now: t0, Points: pts})
	if len(s.DataQuality) != 1 || s.DataQuality[0] != "less than 48 h of history" {
		t.Errorf("%v", s.DataQuality)
	}
}

func TestSummarizeFullHistoryDataQualityEmptyNotNull(t *testing.T) {
	pts := series(t0.Add(-48*time.Hour), time.Hour, linear(80, 40, 49)...)
	s := Summarize(SummaryInput{Pot: testPot(), Now: t0, Points: pts})
	b, _ := json.Marshal(s)
	if !strings.Contains(string(b), `"data_quality":[]`) {
		t.Error(string(b))
	}
}
