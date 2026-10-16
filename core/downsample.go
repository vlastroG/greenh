package core

import "time"

// HistoryPoint is one chart sample; nil fields have no data.
type HistoryPoint struct {
	TS                                                     time.Time
	MoistureRaw, MoisturePct, TempC, HumidityPct, LightLux *float64
}

// Downsample averages ascending points into step-wide buckets aligned to the Unix epoch.
// Each bucket is stamped with its start and averages only non-nil values per field.
// A non-positive step returns points unchanged.
func Downsample(points []HistoryPoint, step time.Duration) []HistoryPoint {
	if step <= 0 {
		return points
	}
	var (
		out []HistoryPoint
		acc bucket
	)
	for _, p := range points {
		start := bucketStart(p.TS, step)
		if acc.n > 0 && !start.Equal(acc.start) {
			out = append(out, acc.point())
			acc = bucket{}
		}
		acc.start = start
		acc.add(p)
	}
	if acc.n > 0 {
		out = append(out, acc.point())
	}
	return out
}

func bucketStart(t time.Time, step time.Duration) time.Time {
	ns, s := t.UnixNano(), int64(step)
	return time.Unix(0, ns-((ns%s)+s)%s).In(t.Location())
}

type mean struct {
	sum float64
	n   int
}

func (m *mean) add(v *float64) {
	if v != nil {
		m.sum += *v
		m.n++
	}
}

func (m mean) value() *float64 {
	if m.n == 0 {
		return nil
	}
	v := m.sum / float64(m.n)
	return &v
}

type bucket struct {
	start                         time.Time
	n                             int
	raw, pct, temp, humidity, lux mean
}

func (b *bucket) add(p HistoryPoint) {
	b.n++
	b.raw.add(p.MoistureRaw)
	b.pct.add(p.MoisturePct)
	b.temp.add(p.TempC)
	b.humidity.add(p.HumidityPct)
	b.lux.add(p.LightLux)
}

func (b bucket) point() HistoryPoint {
	return HistoryPoint{
		TS:          b.start,
		MoistureRaw: b.raw.value(),
		MoisturePct: b.pct.value(),
		TempC:       b.temp.value(),
		HumidityPct: b.humidity.value(),
		LightLux:    b.lux.value(),
	}
}
