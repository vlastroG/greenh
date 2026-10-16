package scheduler_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/vlastrog/greenh/core"
	"github.com/vlastrog/greenh/scheduler"
	"github.com/vlastrog/greenh/store/sqlite/sqlitetest"
)

type runCall struct {
	potID int64
	trig  core.Trigger
}

type fakeRunner struct {
	mu     sync.Mutex
	calls  []runCall
	failID int64
}

func (f *fakeRunner) TryCheck(_ context.Context, potID int64, trig core.Trigger) (core.Advice, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, runCall{potID, trig})
	if potID == f.failID {
		return core.Advice{}, true, errors.New("boom")
	}
	return core.Advice{PotID: potID}, true, nil
}

func (f *fakeRunner) ids() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []int64
	for _, c := range f.calls {
		out = append(out, c.potID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func newScheduler(st core.Store, r *fakeRunner) *scheduler.Scheduler {
	return scheduler.NewScheduler(scheduler.SchedulerConfig{
		Pots: st, Advice: st, Checker: r, Now: func() time.Time { return now },
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

func adviceAgo(t *testing.T, st core.Store, potID int64, ago time.Duration) {
	t.Helper()
	statusAgo(t, st, potID, ago, core.StatusOK)
}

func statusAgo(t *testing.T, st core.Store, potID int64, ago time.Duration, status core.Status) {
	t.Helper()
	a := core.Advice{PotID: potID, CreatedAt: now.Add(-ago), Trigger: core.TriggerScheduled, Status: status}
	if status == core.StatusOK {
		rec := &core.Recommendation{Confidence: core.ConfidenceHigh, Reason: "moist"}
		a.Recommendation, a.Rules, a.Summary = rec, rec, []byte(`{}`)
	}
	if _, err := st.InsertAdvice(context.Background(), a); err != nil {
		t.Fatal(err)
	}
}

func equalIDs(got []int64, want ...int64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestRunOnceChecksDuePots(t *testing.T) {
	st := sqlitetest.New(t)
	a := createPot(t, st, false, false)
	b := createPot(t, st, false, false)
	c := createPot(t, st, false, false)
	adviceAgo(t, st, b.ID, 7*time.Hour)
	adviceAgo(t, st, c.ID, time.Hour)

	r := &fakeRunner{}
	newScheduler(st, r).RunOnce(context.Background())

	if got := r.ids(); !equalIDs(got, a.ID, b.ID) {
		t.Fatalf("checked %v, want [%d %d]", got, a.ID, b.ID)
	}
	for _, call := range r.calls {
		if call.trig != core.TriggerScheduled {
			t.Fatalf("trigger %q", call.trig)
		}
	}
}

func TestRunOnceRechecksNonOKAfterFiveMinutes(t *testing.T) {
	st := sqlitetest.New(t)
	offlineOld := createPot(t, st, false, false)
	uncalOld := createPot(t, st, false, false)
	offlineFresh := createPot(t, st, false, false)
	okOld := createPot(t, st, false, false)
	statusAgo(t, st, offlineOld.ID, 5*time.Minute, core.StatusSensorOffline)
	statusAgo(t, st, uncalOld.ID, 10*time.Minute, core.StatusNeedsCalibration)
	statusAgo(t, st, offlineFresh.ID, 4*time.Minute, core.StatusSensorOffline)
	adviceAgo(t, st, okOld.ID, 10*time.Minute)

	r := &fakeRunner{}
	newScheduler(st, r).RunOnce(context.Background())

	if got := r.ids(); !equalIDs(got, offlineOld.ID, uncalOld.ID) {
		t.Fatalf("checked %v, want [%d %d]", got, offlineOld.ID, uncalOld.ID)
	}
}

func TestRunOnceErrorDoesNotBlockOthers(t *testing.T) {
	st := sqlitetest.New(t)
	a := createPot(t, st, false, false)
	b := createPot(t, st, false, false)

	r := &fakeRunner{failID: a.ID}
	newScheduler(st, r).RunOnce(context.Background())

	if got := r.ids(); !equalIDs(got, a.ID, b.ID) {
		t.Fatalf("checked %v", got)
	}
}

func TestRunOnceOverdueOnlyOnce(t *testing.T) {
	st := sqlitetest.New(t)
	a := createPot(t, st, false, false)
	adviceAgo(t, st, a.ID, 72*time.Hour)

	r := &fakeRunner{}
	newScheduler(st, r).RunOnce(context.Background())

	if got := r.ids(); !equalIDs(got, a.ID) {
		t.Fatalf("checked %v, want exactly one call", got)
	}
}

func TestRunStopsOnCancel(t *testing.T) {
	st := sqlitetest.New(t)
	createPot(t, st, false, false)
	r := &fakeRunner{}
	s := scheduler.NewScheduler(scheduler.SchedulerConfig{
		Pots: st, Advice: st, Checker: r, Now: func() time.Time { return now },
		Tick: time.Millisecond, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return")
	}
	if len(r.ids()) == 0 {
		t.Fatal("expected at least one check")
	}
}
