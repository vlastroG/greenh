package scheduler_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vlastrog/greenh/core"
	"github.com/vlastrog/greenh/scheduler"
	"github.com/vlastrog/greenh/store/sqlite/sqlitetest"
)

var now = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

const (
	dryRaw, wetRaw = 800, 300
	node           = "node-1"
)

// rawFor returns the raw value giving pct moisture.
func rawFor(pct float64) int { return dryRaw - int(pct/100*float64(dryRaw-wetRaw)) }

type adviseFunc func(ctx context.Context) (core.Recommendation, error)

type fakeAdvisor struct {
	mu        sync.Mutex
	responses []adviseFunc
	calls     int
}

func (f *fakeAdvisor) Info() core.ProviderInfo {
	return core.ProviderInfo{Provider: "fake", Model: "fake-1", PromptVersion: "pv1"}
}

func (f *fakeAdvisor) Advise(ctx context.Context, _ core.AdviceRequest) (core.Recommendation, error) {
	f.mu.Lock()
	i := f.calls
	f.calls++
	f.mu.Unlock()
	return f.responses[min(i, len(f.responses)-1)](ctx)
}

func (f *fakeAdvisor) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func answer(r core.Recommendation) adviseFunc {
	return func(context.Context) (core.Recommendation, error) { return r, nil }
}

func fail(err error) adviseFunc {
	return func(context.Context) (core.Recommendation, error) { return core.Recommendation{}, err }
}

var noWater = core.Recommendation{Confidence: core.ConfidenceHigh, Reason: "fine"}

func newChecker(st core.Store, adv core.Advisor, attemptTimeout time.Duration) *scheduler.Checker {
	return scheduler.NewChecker(scheduler.CheckerConfig{
		Store: st, Advisor: adv, Now: func() time.Time { return now },
		OfflineAfter: 30 * time.Minute, AttemptTimeout: attemptTimeout,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

func createPot(t *testing.T, st core.Store, linked, calibrated bool) core.Pot {
	t.Helper()
	p := core.Pot{
		Name: "p", Species: "ficus", Category: core.CategoryFoliage, PotML: 2000,
		SoilType: "loam", Placement: "window",
		Rules:         core.Rules{ThresholdPct: 30, AmountFraction: 0.2},
		CheckInterval: 6 * time.Hour, CreatedAt: now.Add(-24 * time.Hour),
	}
	if linked {
		n, ch := node, 0
		p.NodeID, p.Channel = &n, &ch
	}
	if calibrated {
		d, w := dryRaw, wetRaw
		p.DryRaw, p.WetRaw = &d, &w
	}
	got, err := st.CreatePot(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// addReadings inserts one reading per value, 10 minutes apart, the last one at now.
func addReadings(t *testing.T, st core.Store, pcts ...float64) {
	t.Helper()
	for i, pct := range pcts {
		addReadingAt(t, st, now.Add(-time.Duration(len(pcts)-1-i)*10*time.Minute), pct)
	}
}

func addReadingAt(t *testing.T, st core.Store, ts time.Time, pct float64) {
	t.Helper()
	_, err := st.InsertReading(context.Background(), core.Reading{
		NodeID: node, TS: ts, Channels: []core.ChannelReading{{Ch: 0, MoistureRaw: rawFor(pct)}},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func checkOffline(t *testing.T, st core.Store, a core.Advice, pot core.Pot) {
	t.Helper()
	if a.Status != core.StatusSensorOffline || a.Recommendation != nil || a.Rules != nil {
		t.Fatalf("got %+v", a)
	}
	got, err := st.LatestAdvice(context.Background(), pot.ID)
	if err != nil || got == nil || got.Status != a.Status {
		t.Fatalf("not persisted: %v %v", got, err)
	}
}

func TestCheckUnlinkedPotIsOffline(t *testing.T) {
	st := sqlitetest.New(t)
	pot := createPot(t, st, false, true)
	a, err := newChecker(st, core.RulesAdvisor{}, time.Second).Check(context.Background(), pot.ID, core.TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	checkOffline(t, st, a, pot)
	if a.Trigger != core.TriggerManual || a.Provider != "rules" {
		t.Fatalf("got %+v", a)
	}
}

func TestCheckNeverReportedIsOffline(t *testing.T) {
	st := sqlitetest.New(t)
	pot := createPot(t, st, true, true)
	a, err := newChecker(st, core.RulesAdvisor{}, time.Second).Check(context.Background(), pot.ID, core.TriggerScheduled)
	if err != nil {
		t.Fatal(err)
	}
	checkOffline(t, st, a, pot)
}

func TestCheckNeedsCalibration(t *testing.T) {
	st := sqlitetest.New(t)
	pot := createPot(t, st, true, false)
	addReadings(t, st, 50)
	a, err := newChecker(st, core.RulesAdvisor{}, time.Second).Check(context.Background(), pot.ID, core.TriggerScheduled)
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != core.StatusNeedsCalibration || a.Recommendation != nil {
		t.Fatalf("got %+v", a)
	}
}

func TestCheckGetPotErrorUnchanged(t *testing.T) {
	st := sqlitetest.New(t)
	_, err := newChecker(st, core.RulesAdvisor{}, time.Second).Check(context.Background(), 999, core.TriggerManual)
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestCheckOKUsesLLM(t *testing.T) {
	st := sqlitetest.New(t)
	pot := createPot(t, st, true, true)
	addReadings(t, st, 60, 60, 60)
	rec := core.Recommendation{NeedsWater: true, AmountML: 300, Confidence: core.ConfidenceMedium, Reason: "llm"}
	fake := &fakeAdvisor{responses: []adviseFunc{answer(rec)}}
	a, err := newChecker(st, fake, time.Second).Check(context.Background(), pot.ID, core.TriggerScheduled)
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != core.StatusOK || a.Recommendation == nil || *a.Recommendation != rec {
		t.Fatalf("got %+v", a)
	}
	if a.Rules == nil || a.Rules.NeedsWater || a.Fallback {
		t.Fatalf("rules/fallback wrong: %+v", a)
	}
	if !strings.Contains(string(a.Summary), `"pot"`) {
		t.Fatalf("summary: %s", a.Summary)
	}
	if a.ProviderInfo != fake.Info() {
		t.Fatalf("provider info: %+v", a.ProviderInfo)
	}
	if !a.CreatedAt.Equal(now) {
		t.Fatalf("created_at %v", a.CreatedAt)
	}
}

func TestCheckRetriesOnceThenSucceeds(t *testing.T) {
	st := sqlitetest.New(t)
	pot := createPot(t, st, true, true)
	addReadings(t, st, 60, 60)
	fake := &fakeAdvisor{responses: []adviseFunc{fail(errors.New("boom")), answer(noWater)}}
	a, err := newChecker(st, fake, time.Second).Check(context.Background(), pot.ID, core.TriggerScheduled)
	if err != nil {
		t.Fatal(err)
	}
	if fake.count() != 2 || a.Fallback || *a.Recommendation != noWater {
		t.Fatalf("calls=%d advice=%+v", fake.count(), a)
	}
}

func TestCheckFallsBackAfterTwoFailures(t *testing.T) {
	st := sqlitetest.New(t)
	pot := createPot(t, st, true, true)
	addReadings(t, st, 20, 20)
	bad := core.Recommendation{NeedsWater: true, AmountML: 5000, Confidence: core.ConfidenceHigh, Reason: "too much"}
	fake := &fakeAdvisor{responses: []adviseFunc{answer(bad)}}
	a, err := newChecker(st, fake, time.Second).Check(context.Background(), pot.ID, core.TriggerScheduled)
	if err != nil {
		t.Fatal(err)
	}
	if fake.count() != 2 || !a.Fallback || a.FallbackReason == "" {
		t.Fatalf("calls=%d advice=%+v", fake.count(), a)
	}
	if *a.Recommendation != *a.Rules || a.Disagree {
		t.Fatalf("fallback should equal rules: %+v", a)
	}
	if a.Provider != "fake" {
		t.Fatalf("provider info should stay the configured one: %+v", a.ProviderInfo)
	}
}

func TestCheckTimeoutFallsBack(t *testing.T) {
	st := sqlitetest.New(t)
	pot := createPot(t, st, true, true)
	addReadings(t, st, 50, 50)
	block := func(ctx context.Context) (core.Recommendation, error) {
		<-ctx.Done()
		return core.Recommendation{}, ctx.Err()
	}
	fake := &fakeAdvisor{responses: []adviseFunc{block}}
	a, err := newChecker(st, fake, 20*time.Millisecond).Check(context.Background(), pot.ID, core.TriggerScheduled)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Fallback || fake.count() != 2 || !strings.Contains(a.FallbackReason, "deadline") {
		t.Fatalf("calls=%d advice=%+v", fake.count(), a)
	}
}

func TestCheckCanceledStopsWithoutFallback(t *testing.T) {
	st := sqlitetest.New(t)
	pot := createPot(t, st, true, true)
	addReadings(t, st, 50, 50)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelDuringAttempt := func(actx context.Context) (core.Recommendation, error) {
		cancel()
		return core.Recommendation{}, actx.Err()
	}
	fake := &fakeAdvisor{responses: []adviseFunc{cancelDuringAttempt, answer(noWater)}}
	_, err := newChecker(st, fake, time.Second).Check(ctx, pot.ID, core.TriggerScheduled)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if fake.count() != 1 {
		t.Errorf("calls = %d, want no retry after cancel", fake.count())
	}
	if got, err := st.LatestAdvice(context.Background(), pot.ID); err != nil || got != nil {
		t.Fatalf("advice stored after cancel: %+v %v", got, err)
	}
}

func TestCheckRulesProviderNoRetry(t *testing.T) {
	st := sqlitetest.New(t)
	pot := createPot(t, st, true, true)
	addReadings(t, st, 20, 20)
	a, err := newChecker(st, core.RulesAdvisor{}, time.Second).Check(context.Background(), pot.ID, core.TriggerScheduled)
	if err != nil {
		t.Fatal(err)
	}
	if a.Provider != "rules" || a.Fallback || !a.Recommendation.NeedsWater || *a.Recommendation != *a.Rules || a.Disagree {
		t.Fatalf("got %+v", a)
	}
}

func TestCheckDisagree(t *testing.T) {
	st := sqlitetest.New(t)
	pot := createPot(t, st, true, true)
	addReadings(t, st, 18, 18)
	fake := &fakeAdvisor{responses: []adviseFunc{answer(noWater)}}
	a, err := newChecker(st, fake, time.Second).Check(context.Background(), pot.ID, core.TriggerScheduled)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Rules.NeedsWater || a.Recommendation.NeedsWater || !a.Disagree || a.Fallback {
		t.Fatalf("got %+v", a)
	}
}

func TestTryCheckSkipsWhenRunning(t *testing.T) {
	st := sqlitetest.New(t)
	pot := createPot(t, st, true, true)
	addReadings(t, st, 50, 50)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	fake := &fakeAdvisor{responses: []adviseFunc{
		func(context.Context) (core.Recommendation, error) {
			once.Do(func() { close(entered) })
			<-release
			return noWater, nil
		},
	}}
	c := newChecker(st, fake, time.Minute)

	done := make(chan error, 1)
	go func() {
		_, err := c.Check(context.Background(), pot.ID, core.TriggerScheduled)
		done <- err
	}()
	<-entered
	if _, ok, err := c.TryCheck(context.Background(), pot.ID, core.TriggerScheduled); ok || err != nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, ok, err := c.TryCheck(context.Background(), pot.ID, core.TriggerScheduled); !ok || err != nil {
		t.Fatalf("after release ok=%v err=%v", ok, err)
	}
}

func TestWateringChangesRecommendation(t *testing.T) {
	st := sqlitetest.New(t)
	pot := createPot(t, st, true, true)
	c := newChecker(st, core.RulesAdvisor{}, time.Second)
	ctx := context.Background()

	addReadingAt(t, st, now.Add(-30*time.Minute), 15)
	addReadingAt(t, st, now.Add(-20*time.Minute), 15)
	a, err := c.Check(ctx, pot.ID, core.TriggerScheduled)
	if err != nil || !a.Recommendation.NeedsWater {
		t.Fatalf("expected needs water: %+v %v", a, err)
	}

	wt := now.Add(-15 * time.Minute)
	if _, err := st.CreateWatering(ctx, core.Watering{PotID: pot.ID, TS: wt, CreatedAt: wt, AmountML: 300, Source: "manual"}); err != nil {
		t.Fatal(err)
	}
	addReadingAt(t, st, now.Add(-10*time.Minute), 60)
	addReadingAt(t, st, now.Add(-1*time.Minute), 60)
	a, err = c.Check(ctx, pot.ID, core.TriggerScheduled)
	if err != nil || a.Recommendation.NeedsWater {
		t.Fatalf("expected no water: %+v %v", a, err)
	}
	if !strings.Contains(string(a.Summary), `"amount_ml":300`) {
		t.Fatalf("summary lacks watering: %s", a.Summary)
	}
}
