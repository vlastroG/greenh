package core

import (
	"strings"
	"testing"
)

func TestValidateRecommendation(t *testing.T) {
	ok := Recommendation{true, 250, ConfidenceMedium, "dry"}
	bad := []Recommendation{
		{true, 250, "sure", "x"},
		{false, 10, ConfidenceLow, "x"},
		{true, -1, ConfidenceLow, "x"},
		{true, 1001, ConfidenceLow, "x"},
		{true, 100, ConfidenceLow, strings.Repeat("a", 201)},
		{true, 100, ConfidenceLow, ""},
	}
	if err := ValidateRecommendation(ok, 2000); err != nil {
		t.Fatal(err)
	}
	for _, r := range bad {
		if ValidateRecommendation(r, 2000) == nil {
			t.Errorf("accepted %+v", r)
		}
	}
}

func TestDisagree(t *testing.T) {
	w := func(n bool, ml int) Recommendation { return Recommendation{NeedsWater: n, AmountML: ml} }
	if !Disagree(w(true, 200), w(false, 0)) {
		t.Error("needs_water differs")
	}
	if !Disagree(w(true, 100), w(true, 250)) {
		t.Error(">2x amount")
	}
	if Disagree(w(true, 200), w(true, 300)) || Disagree(w(false, 0), w(false, 0)) {
		t.Error("agree")
	}
}

func TestValidateRecommendationReasonRunes(t *testing.T) {
	r := func(reason string) Recommendation { return Recommendation{true, 100, ConfidenceLow, reason} }
	if err := ValidateRecommendation(r(strings.Repeat("ж", 200)), 2000); err != nil {
		t.Errorf("200 cyrillic runes rejected: %v", err)
	}
	if err := ValidateRecommendation(r("  "+strings.Repeat("ж", 200)+"\n"), 2000); err != nil {
		t.Errorf("padded 200 runes rejected: %v", err)
	}
	for _, bad := range []string{strings.Repeat("ж", 201), "   ", "\t\n"} {
		if ValidateRecommendation(r(bad), 2000) == nil {
			t.Errorf("accepted reason %q", bad)
		}
	}
}
