package core

import (
	"math"
	"strconv"
	"time"
)

type SummaryInput struct {
	Pot          Pot
	Now          time.Time
	Points       []RawPoint // last 48h, ascending
	LastWatering *Watering
}

// Summary is the anonymized view of a pot sent to an advisor.
type Summary struct {
	Pot          SummaryPot       `json:"pot"`
	Now          SummaryNow       `json:"now"`
	Trend        SummaryTrend     `json:"trend"`
	LastWatering *SummaryWatering `json:"last_watering"`
	DataQuality  []string         `json:"data_quality"`
}

type SummaryPot struct {
	Species   string   `json:"species"`
	Category  Category `json:"category"`
	PotML     int      `json:"pot_ml"`
	SoilType  string   `json:"soil_type"`
	Placement string   `json:"placement"`
}

type SummaryNow struct {
	MoisturePct float64  `json:"moisture_pct"`
	TempC       *float64 `json:"temp_c"`
	HumidityPct *float64 `json:"humidity_pct"`
	LightLux    *float64 `json:"light_lux"`
}

type SummaryTrend struct {
	HourlyMoisturePct   []*float64 `json:"hourly_moisture_pct"`
	DryingRatePctPerDay *float64   `json:"drying_rate_pct_per_day"`
	LightLuxHours24h    float64    `json:"light_lux_hours_24h"`
}

type SummaryWatering struct {
	HoursAgo          float64  `json:"hours_ago"`
	AmountML          int      `json:"amount_ml"`
	MoistureBeforePct *float64 `json:"moisture_before_pct"`
	MoistureAfterPct  *float64 `json:"moisture_after_pct"`
}

const (
	hourlyBuckets   = 48
	lightBuckets    = 24
	dryingWindow    = 24 * time.Hour
	minDryingSpan   = 2 * time.Hour
	postWateringGap = time.Hour
	historyNeeded   = 47 * time.Hour
)

// Summarize requires in.Pot.Calibrated() and len(in.Points) > 0.
func Summarize(in SummaryInput) Summary {
	pot := in.Pot
	pct := func(p RawPoint) float64 {
		v, _ := pot.MoisturePct(p.MoistureRaw)
		return v
	}
	last := in.Points[len(in.Points)-1]

	s := Summary{
		Pot: SummaryPot{
			Species:   pot.Species,
			Category:  pot.Category,
			PotML:     pot.PotML,
			SoilType:  pot.SoilType,
			Placement: pot.Placement,
		},
		Now: SummaryNow{
			MoisturePct: round1(pct(last)),
			TempC:       last.Ambient.TempC,
			HumidityPct: last.Ambient.HumidityPct,
			LightLux:    last.Ambient.LightLux,
		},
		DataQuality: dataQuality(in),
	}
	s.Trend.HourlyMoisturePct, s.Trend.LightLuxHours24h = hourly(in, pct)
	s.Trend.DryingRatePctPerDay = dryingRate(in, pct)
	if w := in.LastWatering; w != nil {
		s.LastWatering = summarizeWatering(in, w, pct)
	}
	return s
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func roundedPtr(v float64) *float64 {
	r := round1(v)
	return &r
}

// hourly returns mean moisture per hour for the last 48h and the 24h light integral.
func hourly(in SummaryInput, pct func(RawPoint) float64) ([]*float64, float64) {
	var moistSum, luxSum [hourlyBuckets]float64
	var moistN, luxN [hourlyBuckets]int
	start := in.Now.Add(-hourlyBuckets * time.Hour)
	for _, p := range in.Points {
		if p.TS.Before(start) || p.TS.After(in.Now) {
			continue
		}
		i := min(int(p.TS.Sub(start)/time.Hour), hourlyBuckets-1) // a point exactly at now joins the newest bucket
		moistSum[i] += pct(p)
		moistN[i]++
		if p.Ambient.LightLux != nil {
			luxSum[i] += *p.Ambient.LightLux
			luxN[i]++
		}
	}
	out := make([]*float64, hourlyBuckets)
	var lux float64
	for i := range out {
		if moistN[i] > 0 {
			out[i] = roundedPtr(moistSum[i] / float64(moistN[i]))
		}
		if i >= hourlyBuckets-lightBuckets && luxN[i] > 0 {
			lux += luxSum[i] / float64(luxN[i])
		}
	}
	return out, lux
}

// dryingRate is the least-squares moisture slope over the last 24h, as % lost per day.
// Points up to an hour after a watering are ignored.
func dryingRate(in SummaryInput, pct func(RawPoint) float64) *float64 {
	from := in.Now.Add(-dryingWindow)
	var xs, ys []float64
	var first, lastTS time.Time
	for _, p := range in.Points {
		if p.TS.Before(from) {
			continue
		}
		if w := in.LastWatering; w != nil && !p.TS.After(w.TS.Add(postWateringGap)) {
			continue
		}
		if len(xs) == 0 {
			first = p.TS
		}
		lastTS = p.TS
		xs = append(xs, p.TS.Sub(first).Hours())
		ys = append(ys, pct(p))
	}
	if len(xs) < 2 || lastTS.Sub(first) < minDryingSpan {
		return nil
	}
	n := float64(len(xs))
	var sx, sy, sxx, sxy float64
	for i := range xs {
		sx += xs[i]
		sy += ys[i]
		sxx += xs[i] * xs[i]
		sxy += xs[i] * ys[i]
	}
	slopePerHour := (n*sxy - sx*sy) / (n*sxx - sx*sx)
	return roundedPtr(-slopePerHour * 24)
}

func summarizeWatering(in SummaryInput, w *Watering, pct func(RawPoint) float64) *SummaryWatering {
	sw := &SummaryWatering{
		HoursAgo: round1(in.Now.Sub(w.TS).Hours()),
		AmountML: w.AmountML,
	}
	target := w.TS.Add(time.Hour)
	var best *RawPoint
	for i, p := range in.Points {
		if p.TS.Before(w.TS) {
			sw.MoistureBeforePct = roundedPtr(pct(p))
			continue
		}
		if p.TS.After(w.TS.Add(2 * time.Hour)) {
			continue
		}
		if best == nil || absDur(p.TS.Sub(target)) < absDur(best.TS.Sub(target)) {
			best = &in.Points[i]
		}
	}
	if best != nil {
		sw.MoistureAfterPct = roundedPtr(pct(*best))
	}
	return sw
}

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func dataQuality(in SummaryInput) []string {
	q := []string{}
	for i := 1; i < len(in.Points); i++ {
		gap := in.Points[i].TS.Sub(in.Points[i-1].TS)
		if gap > time.Hour {
			q = append(q, "gap of "+strconv.Itoa(max(1, int(math.Round(gap.Hours()))))+" h in readings")
		}
	}
	if in.Points[0].TS.After(in.Now.Add(-historyNeeded)) {
		q = append(q, "less than 48 h of history")
	}
	return q
}
