package sim

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sort"
	"testing"
	"time"

	"github.com/vlastrog/greenh/core"
)

type fakeBackend struct {
	posts     []ReadingPayload
	err       error
	waterings []WateringEvent
	nodes     []NodeStatus
	nodesErr  error
}

func (f *fakeBackend) PostReading(_ context.Context, r ReadingPayload) error {
	if f.err != nil {
		return f.err
	}
	f.posts = append(f.posts, r)
	return nil
}

func (f *fakeBackend) WateringsSince(context.Context, time.Time) ([]WateringEvent, error) {
	return f.waterings, nil
}

func (f *fakeBackend) Nodes(context.Context) ([]NodeStatus, error) {
	return f.nodes, f.nodesErr
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func testConfig() Config {
	return Config{
		Backend:  "http://x",
		Interval: Duration{30 * time.Second},
		Speed:    1,
		Nodes: []NodeConfig{{ID: "n1", Channels: []ChannelConfig{
			{Ch: 0, Category: core.CategoryFoliage, PotML: 2000, StartMoisture: 40, DryRaw: 540, WetRaw: 270},
			{Ch: 1, Category: core.CategorySucculent, PotML: 500, StartMoisture: 20, DryRaw: 530, WetRaw: 265},
		}}},
	}
}

func newTestRunner(cfg Config, b Backend) (*Runner, *clock) {
	clk := &clock{t: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)}
	return NewRunner(cfg, b, clk.now, slog.New(slog.NewTextHandler(io.Discard, nil))), clk
}

func raw(p ReadingPayload, ch int) int {
	for _, c := range p.Channels {
		if c.Ch == ch {
			return c.MoistureRaw
		}
	}
	return -1
}

func TestBackfillPostsHistory(t *testing.T) {
	cfg := testConfig()
	cfg.Backfill = Duration{time.Hour}
	fb := &fakeBackend{}
	r, clk := newTestRunner(cfg, fb)
	if err := r.Backfill(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := len(fb.posts); n < 12 || n > 13 {
		t.Fatalf("posts = %d, want 12 or 13", n)
	}
	if !sort.SliceIsSorted(fb.posts, func(i, j int) bool { return fb.posts[i].TS.Before(fb.posts[j].TS) }) {
		t.Fatal("timestamps not ascending")
	}
	for _, p := range fb.posts {
		if p.TS.Before(clk.t.Add(-time.Hour)) || p.TS.After(clk.t) {
			t.Fatalf("ts %v outside [now-1h, now]", p.TS)
		}
	}
}

func TestBackfillResumesAfterLastSeen(t *testing.T) {
	testNow := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		nodes    []NodeStatus
		nodesErr error
		min, max int
	}{
		{"seen 10 min ago", []NodeStatus{{ID: "n1", LastSeen: testNow.Add(-10 * time.Minute)}}, nil, 1, 2},
		{"seen within one step", []NodeStatus{{ID: "n1", LastSeen: testNow.Add(-2 * time.Minute)}}, nil, 0, 0},
		{"unknown node", []NodeStatus{{ID: "other", LastSeen: testNow}}, nil, 12, 13},
		{"nodes unavailable", nil, errors.New("down"), 12, 13},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.Backfill = Duration{time.Hour}
			fb := &fakeBackend{nodes: tc.nodes, nodesErr: tc.nodesErr}
			r, clk := newTestRunner(cfg, fb)
			if !clk.t.Equal(testNow) {
				t.Fatalf("clock %v, want %v", clk.t, testNow)
			}
			if err := r.Backfill(context.Background()); err != nil {
				t.Fatal(err)
			}
			if n := len(fb.posts); n < tc.min || n > tc.max {
				t.Fatalf("posts = %d, want %d..%d", n, tc.min, tc.max)
			}
			for _, n := range tc.nodes {
				for _, p := range fb.posts {
					if p.NodeID == n.ID && !p.TS.After(n.LastSeen) {
						t.Fatalf("post at %v not after last_seen %v", p.TS, n.LastSeen)
					}
				}
			}
		})
	}
}

func TestTickAppliesWatering(t *testing.T) {
	fb := &fakeBackend{}
	r, clk := newTestRunner(testConfig(), fb)
	r.Tick(context.Background())
	before := fb.posts[0]

	n, ch := "n1", 0
	fb.waterings = []WateringEvent{{ID: 1, NodeID: &n, Channel: &ch, AmountML: 500, CreatedAt: clk.t}}
	clk.t = clk.t.Add(30 * time.Second)
	r.Tick(context.Background())
	after := fb.posts[1]

	if raw(after, 0) > raw(before, 0)-60 {
		t.Fatalf("ch0 raw %d -> %d, want a clear drop", raw(before, 0), raw(after, 0))
	}
	if d := raw(after, 1) - raw(before, 1); d < -20 || d > 20 {
		t.Fatalf("ch1 changed by %d", d)
	}
}

func TestTickIgnoresUnlinkedAndDuplicateWaterings(t *testing.T) {
	fb := &fakeBackend{}
	r, clk := newTestRunner(testConfig(), fb)
	r.Tick(context.Background())
	before := raw(fb.posts[0], 0)

	n, ch, other := "n1", 0, "other"
	fb.waterings = []WateringEvent{
		{ID: 1, CreatedAt: clk.t, AmountML: 500},
		{ID: 2, NodeID: &other, Channel: &ch, CreatedAt: clk.t, AmountML: 500},
	}
	r.Tick(context.Background())
	if d := before - raw(fb.posts[1], 0); d > 20 {
		t.Fatalf("unlinked/foreign watering applied, raw dropped %d", d)
	}

	// the same event returned on every poll must only apply once
	fb.waterings = []WateringEvent{{ID: 3, NodeID: &n, Channel: &ch, CreatedAt: clk.t, AmountML: 500}}
	r.Tick(context.Background())
	afterFirst := raw(fb.posts[2], 0)
	r.Tick(context.Background())
	if afterFirst > before-60 {
		t.Fatalf("valid watering not applied: %d -> %d", before, afterFirst)
	}
	if raw(fb.posts[3], 0) < afterFirst-20 {
		t.Fatalf("duplicate watering applied again: %d -> %d", afterFirst, raw(fb.posts[3], 0))
	}
}

func TestBufferAndResend(t *testing.T) {
	fb := &fakeBackend{err: errors.New("connection refused")}
	r, clk := newTestRunner(testConfig(), fb)
	for range 3 {
		r.Tick(context.Background())
		clk.t = clk.t.Add(30 * time.Second)
	}
	if len(fb.posts) != 0 {
		t.Fatalf("accepted %d while down", len(fb.posts))
	}
	fb.err = nil
	r.Tick(context.Background())
	if len(fb.posts) != 4 {
		t.Fatalf("posts = %d, want 4", len(fb.posts))
	}
	for i := 1; i < len(fb.posts); i++ {
		if !fb.posts[i].TS.After(fb.posts[i-1].TS) {
			t.Fatal("resend not in ts order")
		}
	}
}

func TestBufferDrops4xx(t *testing.T) {
	fb := &fakeBackend{err: &HTTPError{Status: 400}}
	r, clk := newTestRunner(testConfig(), fb)
	r.Tick(context.Background())
	fb.err = nil
	clk.t = clk.t.Add(30 * time.Second)
	r.Tick(context.Background())
	if len(fb.posts) != 1 {
		t.Fatalf("posts = %d, want 1 (400 payload dropped)", len(fb.posts))
	}
}

func TestBufferKeeps5xxAnd429(t *testing.T) {
	for _, status := range []int{500, 429} {
		fb := &fakeBackend{err: &HTTPError{Status: status}}
		r, clk := newTestRunner(testConfig(), fb)
		r.Tick(context.Background())
		fb.err = nil
		clk.t = clk.t.Add(30 * time.Second)
		r.Tick(context.Background())
		if len(fb.posts) != 2 {
			t.Fatalf("status %d: posts = %d, want 2", status, len(fb.posts))
		}
	}
}

func TestBufferDropsExpired(t *testing.T) {
	fb := &fakeBackend{err: errors.New("down")}
	r, clk := newTestRunner(testConfig(), fb)
	r.Tick(context.Background())
	fb.err = nil
	clk.t = clk.t.Add(8 * 24 * time.Hour)
	r.Tick(context.Background())
	if len(fb.posts) != 1 {
		t.Fatalf("posts = %d, want 1 (stale payload dropped)", len(fb.posts))
	}
}

func TestDropout(t *testing.T) {
	cfg := testConfig()
	cfg.Failures.Dropout = &DropoutFailure{Node: "n1", After: Duration{time.Minute}, For: Duration{time.Minute}}
	fb := &fakeBackend{}
	r, clk := newTestRunner(cfg, fb)

	r.Tick(context.Background())
	if len(fb.posts) != 1 {
		t.Fatalf("before window: posts = %d, want 1", len(fb.posts))
	}
	clk.t = clk.t.Add(90 * time.Second)
	r.Tick(context.Background())
	if len(fb.posts) != 1 {
		t.Fatalf("inside window: posts = %d, want 1", len(fb.posts))
	}
	clk.t = clk.t.Add(90 * time.Second)
	r.Tick(context.Background())
	if len(fb.posts) != 2 {
		t.Fatalf("after window: posts = %d, want 2", len(fb.posts))
	}
}

func TestStuckChannel(t *testing.T) {
	cfg := testConfig()
	cfg.Speed = 600
	cfg.Failures.Stuck = &StuckFailure{Node: "n1", Ch: 0}
	fb := &fakeBackend{}
	r, clk := newTestRunner(cfg, fb)
	for range 5 {
		r.Tick(context.Background())
		clk.t = clk.t.Add(30 * time.Second)
	}
	varies := false
	for _, p := range fb.posts {
		if raw(p, 0) != raw(fb.posts[0], 0) {
			t.Fatalf("stuck channel changed: %d vs %d", raw(p, 0), raw(fb.posts[0], 0))
		}
		if raw(p, 1) != raw(fb.posts[0], 1) {
			varies = true
		}
	}
	if !varies {
		t.Fatal("other channel never varied")
	}
}

func TestOutOfRange(t *testing.T) {
	cfg := testConfig()
	cfg.Failures.OutOfRange = &OutOfRangeFailure{Node: "n1", Ch: 0, Every: 3}
	fb := &fakeBackend{}
	r, clk := newTestRunner(cfg, fb)
	for range 6 {
		r.Tick(context.Background())
		clk.t = clk.t.Add(30 * time.Second)
	}
	for i, p := range fb.posts {
		want := (i+1)%3 == 0
		if got := raw(p, 0) == 9999; got != want {
			t.Fatalf("reading %d: out of range = %v, want %v", i+1, got, want)
		}
		if raw(p, 1) == 9999 {
			t.Fatalf("reading %d: ch1 must be unaffected", i+1)
		}
	}
}
