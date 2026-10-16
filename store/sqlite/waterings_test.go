package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/vlastrog/greenh/core"
	"github.com/vlastrog/greenh/store/sqlite/sqlitetest"
)

func mustWatering(t *testing.T, s core.WateringStore, w core.Watering) core.Watering {
	t.Helper()
	got, err := s.CreateWatering(context.Background(), w)
	if err != nil {
		t.Fatalf("create watering: %v", err)
	}
	return got
}

func TestWateringsSince(t *testing.T) {
	s := sqlitetest.New(t)
	ctx := context.Background()
	linked := mustCreatePot(t, s, newPot("linked", sp("n1"), ip(0)))
	unlinked := mustCreatePot(t, s, newPot("unlinked", nil, nil))

	t1, t2 := t0.Add(time.Minute), t0.Add(2*time.Minute)
	mustWatering(t, s, core.Watering{PotID: linked.ID, TS: t1, CreatedAt: t1, AmountML: 100, Source: "manual"})
	w2 := mustWatering(t, s, core.Watering{PotID: unlinked.ID, TS: t2, CreatedAt: t2, AmountML: 50, Source: "manual"})
	if w2.ID == 0 {
		t.Fatal("expected assigned id")
	}

	got, err := s.WateringsSince(ctx, t1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].PotID != unlinked.ID || got[0].NodeID != nil || got[0].Channel != nil {
		t.Fatalf("since t1: %+v", got)
	}

	all, err := s.WateringsSince(ctx, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].PotID != linked.ID || all[1].PotID != unlinked.ID {
		t.Fatalf("since zero: %+v", all)
	}
	if all[0].NodeID == nil || *all[0].NodeID != "n1" || all[0].Channel == nil || *all[0].Channel != 0 {
		t.Fatalf("link not filled: %+v", all[0])
	}
	if !all[0].TS.Equal(t1) || all[0].AmountML != 100 || all[0].Source != "manual" {
		t.Fatalf("fields: %+v", all[0])
	}
}

func TestListWaterings(t *testing.T) {
	s := sqlitetest.New(t)
	ctx := context.Background()
	p := mustCreatePot(t, s, newPot("a", nil, nil))
	for i := 0; i < 3; i++ {
		ts := t0.Add(time.Duration(i) * time.Hour)
		mustWatering(t, s, core.Watering{PotID: p.ID, TS: ts, CreatedAt: ts, AmountML: 10 + i, Source: "manual"})
	}
	got, err := s.ListWaterings(ctx, p.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].AmountML != 12 || got[1].AmountML != 11 {
		t.Fatalf("list: %+v", got)
	}
}

func TestLastWateringByTS(t *testing.T) {
	s := sqlitetest.New(t)
	ctx := context.Background()
	p := mustCreatePot(t, s, newPot("a", nil, nil))

	if w, err := s.LastWatering(ctx, p.ID); err != nil || w != nil {
		t.Fatalf("empty: %v %v", w, err)
	}

	newer := t0.Add(time.Hour)
	mustWatering(t, s, core.Watering{PotID: p.ID, TS: newer, CreatedAt: newer, AmountML: 100, Source: "manual"})
	// backdated: created later but happened earlier
	mustWatering(t, s, core.Watering{PotID: p.ID, TS: t0, CreatedAt: newer.Add(time.Hour), AmountML: 5, Source: "manual"})

	w, err := s.LastWatering(ctx, p.ID)
	if err != nil || w == nil {
		t.Fatalf("last: %v %v", w, err)
	}
	if !w.TS.Equal(newer) || w.AmountML != 100 {
		t.Fatalf("last = %+v", w)
	}
}

func TestDeletePotCascades(t *testing.T) {
	s := sqlitetest.New(t)
	ctx := context.Background()
	p := mustCreatePot(t, s, newPot("a", nil, nil))
	mustWatering(t, s, core.Watering{PotID: p.ID, TS: t0, CreatedAt: t0, AmountML: 10, Source: "manual"})
	if _, err := s.InsertAdvice(ctx, core.Advice{PotID: p.ID, CreatedAt: t0, Trigger: core.TriggerManual, Status: core.StatusSensorOffline}); err != nil {
		t.Fatal(err)
	}

	if err := s.DeletePot(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if w, err := s.WateringsSince(ctx, time.Time{}); err != nil || len(w) != 0 {
		t.Fatalf("waterings left: %v %v", w, err)
	}
	if a, err := s.ListAdvice(ctx, p.ID, 10); err != nil || len(a) != 0 {
		t.Fatalf("advice left: %v %v", a, err)
	}
}
