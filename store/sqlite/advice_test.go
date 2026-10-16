package sqlite_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/vlastrog/greenh/core"
	"github.com/vlastrog/greenh/store/sqlite/sqlitetest"
)

func TestAdviceRoundTrip(t *testing.T) {
	s := sqlitetest.New(t)
	ctx := context.Background()
	p := mustCreatePot(t, s, newPot("a", nil, nil))

	ok := core.Advice{
		PotID: p.ID, CreatedAt: t0, Trigger: core.TriggerScheduled, Status: core.StatusOK,
		Recommendation: &core.Recommendation{NeedsWater: true, AmountML: 200, Confidence: core.ConfidenceHigh, Reason: "dry"},
		Rules:          &core.Recommendation{NeedsWater: false, AmountML: 0, Confidence: core.ConfidenceLow, Reason: "rules"},
		ProviderInfo:   core.ProviderInfo{Provider: "openaicompat", Model: "m", PromptVersion: "v1"},
		Fallback:       true, FallbackReason: "bad answer", Disagree: true,
		Summary: json.RawMessage(`{"moisture_pct":12.5}`),
	}
	stored, err := s.InsertAdvice(ctx, ok)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID == 0 {
		t.Fatal("expected assigned id")
	}
	ok.ID = stored.ID
	got, err := s.LatestAdvice(ctx, p.ID)
	if err != nil || got == nil {
		t.Fatalf("latest: %v %v", got, err)
	}
	if !reflect.DeepEqual(*got, ok) {
		t.Fatalf("mismatch:\n got %+v\nwant %+v", *got, ok)
	}

	off := core.Advice{PotID: p.ID, CreatedAt: t0.Add(time.Hour), Trigger: core.TriggerManual, Status: core.StatusSensorOffline}
	stored, err = s.InsertAdvice(ctx, off)
	if err != nil {
		t.Fatal(err)
	}
	off.ID = stored.ID
	got, err = s.LatestAdvice(ctx, p.ID)
	if err != nil || got == nil {
		t.Fatalf("latest: %v %v", got, err)
	}
	if got.Recommendation != nil || got.Rules != nil || got.Summary != nil {
		t.Fatalf("nils not preserved: %+v", *got)
	}
	if !reflect.DeepEqual(*got, off) {
		t.Fatalf("mismatch:\n got %+v\nwant %+v", *got, off)
	}

	list, err := s.ListAdvice(ctx, p.ID, 1)
	if err != nil || len(list) != 1 || list[0].ID != off.ID {
		t.Fatalf("list: %v %+v", err, list)
	}
	list, err = s.ListAdvice(ctx, p.ID, 10)
	if err != nil || len(list) != 2 || list[1].ID != ok.ID {
		t.Fatalf("list all: %v %+v", err, list)
	}

	other := mustCreatePot(t, s, newPot("b", nil, nil))
	if a, err := s.LatestAdvice(ctx, other.ID); err != nil || a != nil {
		t.Fatalf("latest empty: %v %v", a, err)
	}
}

func TestLastAdviceStamps(t *testing.T) {
	s := sqlitetest.New(t)
	ctx := context.Background()
	p1 := mustCreatePot(t, s, newPot("a", nil, nil))
	p2 := mustCreatePot(t, s, newPot("b", nil, nil))
	mustCreatePot(t, s, newPot("none", nil, nil))

	for _, in := range []struct {
		pot    int64
		at     time.Time
		status core.Status
	}{
		{p1.ID, t0, core.StatusSensorOffline},
		{p1.ID, t0.Add(2 * time.Hour), core.StatusNeedsCalibration},
		{p1.ID, t0.Add(time.Hour), core.StatusSensorOffline},
		{p2.ID, t0.Add(time.Hour), core.StatusSensorOffline},
	} {
		a := core.Advice{PotID: in.pot, CreatedAt: in.at, Trigger: core.TriggerScheduled, Status: in.status}
		if _, err := s.InsertAdvice(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.LastAdviceStamps(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int64]core.AdviceStamp{
		p1.ID: {At: t0.Add(2 * time.Hour), Status: core.StatusNeedsCalibration},
		p2.ID: {At: t0.Add(time.Hour), Status: core.StatusSensorOffline},
	}
	if len(got) != len(want) {
		t.Fatalf("stamps: %v", got)
	}
	for id, w := range want {
		if g := got[id]; !g.At.Equal(w.At) || g.Status != w.Status {
			t.Errorf("pot %d: %+v, want %+v", id, g, w)
		}
	}
}
