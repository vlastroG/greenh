package core

import (
	"context"
	"testing"
)

func TestRulesAdvisor(t *testing.T) {
	req := func(pct float64) AdviceRequest {
		return AdviceRequest{Summary: Summary{Pot: SummaryPot{PotML: 2000}, Now: SummaryNow{MoisturePct: pct}}, Rules: Rules{30, 0.2}}
	}
	r, _ := RulesAdvisor{}.Advise(context.Background(), req(18))
	if !r.NeedsWater || r.AmountML != 400 || r.Confidence != ConfidenceHigh {
		t.Errorf("%+v", r)
	}
	if r.Reason != "Moisture 18% is below the 30% threshold." {
		t.Error(r.Reason)
	}
	r, _ = RulesAdvisor{}.Advise(context.Background(), req(27))
	if !r.NeedsWater || r.Confidence != ConfidenceMedium {
		t.Errorf("%+v", r)
	}
	r, _ = RulesAdvisor{}.Advise(context.Background(), req(45))
	if r.NeedsWater || r.AmountML != 0 || r.Confidence != ConfidenceHigh {
		t.Errorf("%+v", r)
	}
	if r.Reason != "Moisture 45% is at or above the 30% threshold." {
		t.Error(r.Reason)
	}
	if err := ValidateRecommendation(r, 2000); err != nil {
		t.Error(err)
	}
	if info := (RulesAdvisor{}).Info(); info != (ProviderInfo{Provider: "rules", Model: "rules-v1"}) {
		t.Errorf("%+v", info)
	}
}

func TestRulesAdvisorRoundsAmountToTen(t *testing.T) {
	r, _ := RulesAdvisor{}.Advise(context.Background(), AdviceRequest{
		Summary: Summary{Pot: SummaryPot{PotML: 1230}, Now: SummaryNow{MoisturePct: 5}},
		Rules:   Rules{30, 0.2},
	})
	if r.AmountML != 250 { // 246 rounds to 250
		t.Errorf("%+v", r)
	}
}
