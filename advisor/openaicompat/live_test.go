//go:build live

package openaicompat

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/vlastrog/greenh/core"
)

func TestLiveAdvise(t *testing.T) {
	key := os.Getenv("ADVISOR_API_KEY")
	if key == "" {
		t.Skip("ADVISOR_API_KEY not set")
	}
	c := New(Config{BaseURL: os.Getenv("ADVISOR_BASE_URL"), Model: os.Getenv("ADVISOR_MODEL"), APIKey: key})

	dry := 0.35
	summary := core.Summary{
		Pot:         core.SummaryPot{Species: "ficus", Category: "tropical", PotML: 2000, SoilType: "potting mix", Placement: "indoor"},
		Now:         core.SummaryNow{MoisturePct: 22},
		Trend:       core.SummaryTrend{DryingRatePctPerDay: &dry},
		DataQuality: []string{},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r, err := c.Advise(ctx, core.AdviceRequest{Summary: summary})
	if err != nil {
		t.Fatal(err)
	}
	if err := core.ValidateRecommendation(r, 2000); err != nil {
		t.Fatalf("invalid recommendation %+v: %v", r, err)
	}
}
