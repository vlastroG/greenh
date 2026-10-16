package sqlite_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/vlastrog/greenh/core"
	"github.com/vlastrog/greenh/store/sqlite/sqlitetest"
)

func sp(v string) *string { return &v }
func ip(v int) *int       { return &v }

func newPot(name string, node *string, ch *int) core.Pot {
	return core.Pot{
		Name: name, Species: "ficus", Category: core.CategoryFoliage, PotML: 2000,
		SoilType: "loam", Placement: "window",
		NodeID: node, Channel: ch,
		Rules:         core.Rules{ThresholdPct: 30, AmountFraction: 0.2},
		CheckInterval: 6 * time.Hour,
		CreatedAt:     t0,
	}
}

func mustCreatePot(t *testing.T, s core.PotStore, p core.Pot) core.Pot {
	t.Helper()
	got, err := s.CreatePot(context.Background(), p)
	if err != nil {
		t.Fatalf("create pot: %v", err)
	}
	return got
}

func TestPotCRUD(t *testing.T) {
	s := sqlitetest.New(t)
	ctx := context.Background()

	bare := mustCreatePot(t, s, newPot("bare", nil, nil))
	if bare.ID == 0 {
		t.Fatal("expected assigned id")
	}
	full := newPot("full", sp("n1"), ip(0))
	full.DryRaw, full.WetRaw = ip(800), ip(300)
	full = mustCreatePot(t, s, full)

	for _, want := range []core.Pot{bare, full} {
		got, err := s.GetPot(ctx, want.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("get mismatch:\n got %+v\nwant %+v", got, want)
		}
	}
	if bare.NodeID != nil || bare.DryRaw != nil || bare.CheckInterval != 6*time.Hour {
		t.Fatalf("unexpected bare pot: %+v", bare)
	}

	list, err := s.ListPots(ctx)
	if err != nil || len(list) != 2 || list[0].ID != bare.ID || list[1].ID != full.ID {
		t.Fatalf("list: %v %+v", err, list)
	}

	upd := full
	upd.Name = "renamed"
	upd.PotML = 3000
	upd.NodeID, upd.Channel = nil, nil
	upd.DryRaw = nil
	upd.CheckInterval = 90 * time.Minute
	got, err := s.UpdatePot(ctx, upd)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, upd) {
		t.Fatalf("update mismatch:\n got %+v\nwant %+v", got, upd)
	}
	if _, err := s.UpdatePot(ctx, core.Pot{ID: 999}); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("update missing: %v", err)
	}

	if err := s.DeletePot(ctx, bare.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetPot(ctx, bare.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("get after delete: %v", err)
	}
	if err := s.DeletePot(ctx, bare.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
}

func TestPotLinkConflict(t *testing.T) {
	s := sqlitetest.New(t)
	ctx := context.Background()

	mustCreatePot(t, s, newPot("a", sp("n1"), ip(0)))
	if _, err := s.CreatePot(ctx, newPot("b", sp("n1"), ip(0))); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("create dup link: %v", err)
	}

	other := mustCreatePot(t, s, newPot("c", sp("n1"), ip(1)))
	other.Channel = ip(0)
	if _, err := s.UpdatePot(ctx, other); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("update dup link: %v", err)
	}

	mustCreatePot(t, s, newPot("u1", nil, nil))
	mustCreatePot(t, s, newPot("u2", nil, nil))
}

func TestListNodesFillsPotID(t *testing.T) {
	s := sqlitetest.New(t)
	ctx := context.Background()
	if _, err := s.InsertReading(ctx, reading("n1", t0, 100, 200)); err != nil {
		t.Fatal(err)
	}
	pot := mustCreatePot(t, s, newPot("a", sp("n1"), ip(1)))

	nodes, err := s.ListNodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || len(nodes[0].Channels) != 2 {
		t.Fatalf("nodes: %+v", nodes)
	}
	if nodes[0].Channels[0].PotID != nil {
		t.Fatalf("channel 0 should be unlinked: %+v", nodes[0].Channels[0])
	}
	if p := nodes[0].Channels[1].PotID; p == nil || *p != pot.ID {
		t.Fatalf("channel 1 pot id = %v, want %d", p, pot.ID)
	}
}
