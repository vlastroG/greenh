package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/vlastrog/greenh/core"
	"github.com/vlastrog/greenh/store/sqlite"
	"github.com/vlastrog/greenh/store/sqlite/sqlitetest"
)

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func fp(v float64) *float64 { return &v }

func reading(node string, ts time.Time, raw0, raw1 int) core.Reading {
	return core.Reading{
		NodeID:   node,
		TS:       ts,
		Ambient:  core.Ambient{TempC: fp(21.5), HumidityPct: fp(40), LightLux: fp(300)},
		Channels: []core.ChannelReading{{Ch: 0, MoistureRaw: raw0}, {Ch: 1, MoistureRaw: raw1}},
	}
}

func TestInsertReadingAndHistory(t *testing.T) {
	s := sqlitetest.New(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		ok, err := s.InsertReading(ctx, reading("n1", t0.Add(time.Duration(i)*time.Minute), 100+i, 200+i))
		if err != nil || !ok {
			t.Fatalf("insert %d: ok=%v err=%v", i, ok, err)
		}
	}
	pts, err := s.History(ctx, "n1", 1, t0, t0.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 3 {
		t.Fatalf("got %d points", len(pts))
	}
	for i, p := range pts {
		if p.MoistureRaw != 200+i || !p.TS.Equal(t0.Add(time.Duration(i)*time.Minute)) {
			t.Errorf("point %d = %+v", i, p)
		}
		if p.Ambient.TempC == nil || *p.Ambient.TempC != 21.5 || *p.Ambient.HumidityPct != 40 || *p.Ambient.LightLux != 300 {
			t.Errorf("point %d ambient = %+v", i, p.Ambient)
		}
	}
	// to is exclusive, from inclusive
	pts, _ = s.History(ctx, "n1", 1, t0.Add(time.Minute), t0.Add(2*time.Minute))
	if len(pts) != 1 || pts[0].MoistureRaw != 201 {
		t.Errorf("range = %+v", pts)
	}
}

func TestInsertReadingDuplicateIgnored(t *testing.T) {
	s := sqlitetest.New(t)
	ctx := context.Background()
	if ok, err := s.InsertReading(ctx, reading("n1", t0, 100, 200)); err != nil || !ok {
		t.Fatalf("first: ok=%v err=%v", ok, err)
	}
	ok, err := s.InsertReading(ctx, reading("n1", t0, 999, 999))
	if err != nil || ok {
		t.Fatalf("second: ok=%v err=%v", ok, err)
	}
	pts, _ := s.History(ctx, "n1", 0, t0, t0.Add(time.Hour))
	if len(pts) != 1 || pts[0].MoistureRaw != 100 {
		t.Errorf("history = %+v", pts)
	}
}

func TestInsertReadingOutOfOrder(t *testing.T) {
	s := sqlitetest.New(t)
	ctx := context.Background()
	for _, m := range []int{3, 1, 2} {
		if _, err := s.InsertReading(ctx, reading("n1", t0.Add(time.Duration(m)*time.Minute), m, m)); err != nil {
			t.Fatal(err)
		}
	}
	pts, _ := s.History(ctx, "n1", 0, t0, t0.Add(time.Hour))
	if len(pts) != 3 || pts[0].MoistureRaw != 1 || pts[1].MoistureRaw != 2 || pts[2].MoistureRaw != 3 {
		t.Errorf("history = %+v", pts)
	}
}

func TestInsertReadingNilAmbient(t *testing.T) {
	s := sqlitetest.New(t)
	ctx := context.Background()
	r := core.Reading{NodeID: "n1", TS: t0, Channels: []core.ChannelReading{{Ch: 0, MoistureRaw: 5}}}
	if _, err := s.InsertReading(ctx, r); err != nil {
		t.Fatal(err)
	}
	pts, _ := s.History(ctx, "n1", 0, t0, t0.Add(time.Hour))
	if len(pts) != 1 {
		t.Fatalf("history = %+v", pts)
	}
	a := pts[0].Ambient
	if a.TempC != nil || a.HumidityPct != nil || a.LightLux != nil {
		t.Errorf("ambient = %+v", a)
	}
}

func TestAutoRegisterNode(t *testing.T) {
	s := sqlitetest.New(t)
	ctx := context.Background()
	if _, err := s.InsertReading(ctx, reading("n1", t0, 100, 200)); err != nil {
		t.Fatal(err)
	}
	// older backfill must not move last_seen back
	if _, err := s.InsertReading(ctx, reading("n1", t0.Add(-time.Hour), 90, 190)); err != nil {
		t.Fatal(err)
	}
	nodes, err := s.ListNodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("nodes = %+v", nodes)
	}
	n := nodes[0]
	if n.ID != "n1" || n.Name != "" || !n.LastSeen.Equal(t0) || n.LastSeen.Location() != time.UTC {
		t.Errorf("node = %+v", n)
	}
	if len(n.Channels) != 2 || n.Channels[0].Ch != 0 || n.Channels[1].Ch != 1 {
		t.Fatalf("channels = %+v", n.Channels)
	}
	c := n.Channels[1]
	if c.LastRaw == nil || *c.LastRaw != 200 || c.LastAt == nil || !c.LastAt.Equal(t0) || c.PotID != nil {
		t.Errorf("channel = %+v", c)
	}
}

func TestLastRaw(t *testing.T) {
	s := sqlitetest.New(t)
	ctx := context.Background()
	p, err := s.LastRaw(ctx, "n1", 0)
	if err != nil || p != nil {
		t.Fatalf("unknown: p=%v err=%v", p, err)
	}
	for i := 0; i < 3; i++ {
		s.InsertReading(ctx, reading("n1", t0.Add(time.Duration(i)*time.Minute), 100+i, 200+i))
	}
	p, err = s.LastRaw(ctx, "n1", 0)
	if err != nil || p == nil || p.MoistureRaw != 102 || !p.TS.Equal(t0.Add(2*time.Minute)) {
		t.Fatalf("p=%+v err=%v", p, err)
	}
	if p, _ = s.LastRaw(ctx, "n1", 7); p != nil {
		t.Errorf("unknown channel = %+v", p)
	}
}

func TestRenameNode(t *testing.T) {
	s := sqlitetest.New(t)
	ctx := context.Background()
	s.InsertReading(ctx, reading("n1", t0, 1, 2))
	n, err := s.RenameNode(ctx, "n1", "Kitchen")
	if err != nil || n.Name != "Kitchen" || len(n.Channels) != 2 {
		t.Fatalf("n=%+v err=%v", n, err)
	}
	nodes, _ := s.ListNodes(ctx)
	if nodes[0].Name != "Kitchen" {
		t.Errorf("nodes = %+v", nodes)
	}
	if _, err := s.RenameNode(ctx, "nope", "x"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}

func TestOpenTwiceMigratesOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "twice.db")
	for i := 0; i < 2; i++ {
		s, err := sqlite.Open(context.Background(), path)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		s.Close()
	}
}
