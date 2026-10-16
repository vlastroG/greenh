package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/vlastrog/greenh/core"
)

const (
	defaultTick          = time.Minute
	defaultParallel      = 4
	defaultCheckInterval = 6 * time.Hour
)

type checkRunner interface {
	TryCheck(ctx context.Context, potID int64, trig core.Trigger) (core.Advice, bool, error)
}

type SchedulerConfig struct {
	Pots     core.PotStore
	Advice   core.AdviceStore
	Checker  checkRunner
	Now      func() time.Time
	Tick     time.Duration // default 1m
	Parallel int           // default 4
	Log      *slog.Logger  // default slog.Default()
}

// Scheduler checks each due pot on a timer.
type Scheduler struct {
	cfg SchedulerConfig
}

func NewScheduler(cfg SchedulerConfig) *Scheduler {
	if cfg.Tick <= 0 {
		cfg.Tick = defaultTick
	}
	if cfg.Parallel <= 0 {
		cfg.Parallel = defaultParallel
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	return &Scheduler{cfg: cfg}
}

// Run checks due pots immediately, then every Tick until ctx is done.
func (s *Scheduler) Run(ctx context.Context) {
	s.RunOnce(ctx)
	t := time.NewTicker(s.cfg.Tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.RunOnce(ctx)
		}
	}
}

// RunOnce checks every due pot once, with bounded concurrency, and returns when all finish.
func (s *Scheduler) RunOnce(ctx context.Context) {
	pots, err := s.cfg.Pots.ListPots(ctx)
	if err != nil {
		s.cfg.Log.Error("scheduler: list pots", "error", err)
		return
	}
	last, err := s.cfg.Advice.LastAdviceTimes(ctx)
	if err != nil {
		s.cfg.Log.Error("scheduler: last advice times", "error", err)
		return
	}
	now := s.cfg.Now()

	sem := make(chan struct{}, s.cfg.Parallel)
	var wg sync.WaitGroup
	for _, p := range pots {
		if !due(p, last, now) {
			continue
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return
		}
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			defer func() { <-sem }()
			if _, _, err := s.cfg.Checker.TryCheck(ctx, id, core.TriggerScheduled); err != nil {
				s.cfg.Log.Error("scheduled check failed", "pot_id", id, "error", err)
			}
		}(p.ID)
	}
	wg.Wait()
}

func due(p core.Pot, last map[int64]time.Time, now time.Time) bool {
	at, ok := last[p.ID]
	if !ok {
		return true
	}
	interval := p.CheckInterval
	if interval <= 0 {
		interval = defaultCheckInterval
	}
	return !at.Add(interval).After(now)
}
