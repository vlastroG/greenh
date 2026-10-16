package api

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/oapi-codegen/nullable"

	"github.com/vlastrog/greenh/core"
)

// nullableOf maps nil to an explicit null; an unset Nullable would marshal as T's zero value.
func nullableOf[T any](p *T) nullable.Nullable[T] {
	if p == nil {
		return nullable.NewNullNullable[T]()
	}
	return nullable.NewNullableWithValue(*p)
}

// applyNullable writes a specified patch field to dst; null clears it.
func applyNullable[T any](dst **T, n nullable.Nullable[T]) {
	if !n.IsSpecified() {
		return
	}
	if n.IsNull() {
		*dst = nil
		return
	}
	v := n.MustGet()
	*dst = &v
}

func toNode(n core.Node) Node {
	chans := make([]NodeChannel, len(n.Channels))
	for i, c := range n.Channels {
		chans[i] = NodeChannel{
			Ch:      c.Ch,
			LastRaw: nullableOf(c.LastRaw),
			LastAt:  nullableOf(c.LastAt),
			PotId:   nullableOf(c.PotID),
		}
	}
	return Node{Id: n.ID, Name: n.Name, LastSeen: n.LastSeen, Channels: chans}
}

func toReading(in ReadingIn) core.Reading {
	r := core.Reading{NodeID: in.NodeId, Channels: make([]core.ChannelReading, len(in.Channels))}
	if in.Ts != nil {
		r.TS = *in.Ts
	}
	if in.Ambient != nil {
		r.Ambient = core.Ambient{TempC: in.Ambient.TempC, HumidityPct: in.Ambient.HumidityPct, LightLux: in.Ambient.LightLux}
	}
	for i, c := range in.Channels {
		r.Channels[i] = core.ChannelReading{Ch: c.Ch, MoistureRaw: c.MoistureRaw}
	}
	return r
}

// toPot needs the pot's last reading (nil if none) and latest advice (nil if none).
func toPot(p core.Pot, last *core.RawPoint, latest *core.Advice) (Pot, error) {
	out := Pot{
		Id:               p.ID,
		Name:             p.Name,
		Species:          p.Species,
		Category:         Category(p.Category),
		PotMl:            p.PotML,
		SoilType:         p.SoilType,
		Placement:        p.Placement,
		NodeId:           nullableOf(p.NodeID),
		Channel:          nullableOf(p.Channel),
		DryRaw:           nullableOf(p.DryRaw),
		WetRaw:           nullableOf(p.WetRaw),
		ThresholdPct:     p.Rules.ThresholdPct,
		AmountFraction:   p.Rules.AmountFraction,
		CheckIntervalMin: int(p.CheckInterval / time.Minute),
		CreatedAt:        p.CreatedAt,
		Current: PotCurrent{
			MoistureRaw:   nullable.NewNullNullable[int](),
			MoisturePct:   nullable.NewNullNullable[float64](),
			LastReadingAt: nullable.NewNullNullable[time.Time](),
		},
		LatestAdvice: nullable.NewNullNullable[Advice](),
	}
	if last != nil {
		out.Current.MoistureRaw.Set(last.MoistureRaw)
		out.Current.LastReadingAt.Set(last.TS)
		if pct, ok := p.MoisturePct(last.MoistureRaw); ok {
			out.Current.MoisturePct.Set(pct)
		}
	}
	if latest != nil {
		a, err := toAdvice(*latest)
		if err != nil {
			return Pot{}, err
		}
		out.LatestAdvice.Set(a)
	}
	return out, nil
}

func toRecommendation(r *core.Recommendation) nullable.Nullable[Recommendation] {
	if r == nil {
		return nullable.NewNullNullable[Recommendation]()
	}
	return nullable.NewNullableWithValue(Recommendation{
		NeedsWater: r.NeedsWater,
		AmountMl:   r.AmountML,
		Confidence: RecommendationConfidence(r.Confidence),
		Reason:     r.Reason,
	})
}

func toAdvice(a core.Advice) (Advice, error) {
	summary := nullable.NewNullNullable[map[string]any]()
	if len(a.Summary) > 0 {
		var m map[string]any
		if err := json.Unmarshal(a.Summary, &m); err != nil {
			return Advice{}, fmt.Errorf("advice %d summary: %w", a.ID, err)
		}
		summary.Set(m)
	}
	return Advice{
		Id:             a.ID,
		PotId:          a.PotID,
		CreatedAt:      a.CreatedAt,
		Trigger:        AdviceTrigger(a.Trigger),
		Status:         AdviceStatus(a.Status),
		Recommendation: toRecommendation(a.Recommendation),
		Rules:          toRecommendation(a.Rules),
		Provider:       a.Provider,
		Model:          a.Model,
		PromptVersion:  a.PromptVersion,
		Fallback:       a.Fallback,
		FallbackReason: a.FallbackReason,
		Disagree:       a.Disagree,
		Summary:        summary,
	}, nil
}

func toAdviceList(history []core.Advice) (AdviceList, error) {
	out := AdviceList{Latest: nullable.NewNullNullable[Advice](), History: make([]Advice, len(history))}
	for i, a := range history {
		conv, err := toAdvice(a)
		if err != nil {
			return AdviceList{}, err
		}
		out.History[i] = conv
	}
	if len(out.History) > 0 {
		out.Latest.Set(out.History[0])
	}
	return out, nil
}

// toHistoryPoint leaves moisture_pct nil when the pot is not calibrated.
func toHistoryPoint(p core.Pot, rp core.RawPoint) core.HistoryPoint {
	raw := float64(rp.MoistureRaw)
	hp := core.HistoryPoint{
		TS:          rp.TS,
		MoistureRaw: &raw,
		TempC:       rp.Ambient.TempC,
		HumidityPct: rp.Ambient.HumidityPct,
		LightLux:    rp.Ambient.LightLux,
	}
	if pct, ok := p.MoisturePct(rp.MoistureRaw); ok {
		hp.MoisturePct = &pct
	}
	return hp
}

func fromHistoryPoints(points []core.HistoryPoint) []HistoryPoint {
	out := make([]HistoryPoint, len(points))
	for i, p := range points {
		out[i] = HistoryPoint{
			Ts:          p.TS,
			MoistureRaw: nullableOf(p.MoistureRaw),
			MoisturePct: nullableOf(p.MoisturePct),
			TempC:       nullableOf(p.TempC),
			HumidityPct: nullableOf(p.HumidityPct),
			LightLux:    nullableOf(p.LightLux),
		}
	}
	return out
}

func toWatering(w core.Watering) Watering {
	return Watering{Id: w.ID, PotId: w.PotID, Ts: w.TS, CreatedAt: w.CreatedAt, AmountMl: w.AmountML, Source: w.Source}
}

func toFeedItem(w core.WateringLink) WateringFeedItem {
	return WateringFeedItem{
		Id: w.ID, PotId: w.PotID, Ts: w.TS, CreatedAt: w.CreatedAt, AmountMl: w.AmountML, Source: w.Source,
		NodeId:  nullableOf(w.NodeID),
		Channel: nullableOf(w.Channel),
	}
}
