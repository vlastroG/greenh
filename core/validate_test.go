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
