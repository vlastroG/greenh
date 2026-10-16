package core

import (
	"context"
	"math"
	"strconv"
)

type AdviceRequest struct {
	Summary Summary
	Rules   Rules
}

type Advisor interface {
	Info() ProviderInfo
	Advise(ctx context.Context, req AdviceRequest) (Recommendation, error)
}

// RulesAdvisor decides from the pot's threshold and amount fraction alone.
type RulesAdvisor struct{}

func (RulesAdvisor) Info() ProviderInfo {
	return ProviderInfo{Provider: "rules", Model: "rules-v1"}
}

func (RulesAdvisor) Advise(_ context.Context, req AdviceRequest) (Recommendation, error) {
	m, th := req.Summary.Now.MoisturePct, req.Rules.ThresholdPct
	if m >= th {
		return Recommendation{
			Confidence: ConfidenceHigh,
			Reason:     "Moisture " + pctStr(m) + "% is at or above the " + pctStr(th) + "% threshold.",
		}, nil
	}
	potML := req.Summary.Pot.PotML
	amount := int(math.Round(req.Rules.AmountFraction*float64(potML)/10)) * 10
	amount = min(amount, potML/2)
	conf := ConfidenceMedium
	if m < th-5 {
		conf = ConfidenceHigh
	}
	return Recommendation{
		NeedsWater: true,
		AmountML:   amount,
		Confidence: conf,
		Reason:     "Moisture " + pctStr(m) + "% is below the " + pctStr(th) + "% threshold.",
	}, nil
}

func pctStr(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
