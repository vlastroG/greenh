// Package scheduler runs advice checks for pots, on a timer and on demand.
package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/vlastrog/greenh/core"
)

const (
	historyWindow   = 48 * time.Hour
	maxAttempts     = 2
	maxReasonLength = 200
	rulesProvider   = "rules"
)

type CheckerConfig struct {
	Store                        core.Store
	Advisor                      core.Advisor // may itself be core.RulesAdvisor{}
	Now                          func() time.Time
	OfflineAfter, AttemptTimeout time.Duration
	Log                          *slog.Logger
}

// Checker is the one check path shared by the scheduler and the "Check now" API.
type Checker struct {
	cfg CheckerConfig

	mu    sync.Mutex
	locks map[int64]*sync.Mutex
}

func NewChecker(cfg CheckerConfig) *Checker {
	return &Checker{cfg: cfg, locks: map[int64]*sync.Mutex{}}
}

func (c *Checker) potLock(potID int64) *sync.Mutex {
	c.mu.Lock()
	defer c.mu.Unlock()
	l, ok := c.locks[potID]
	if !ok {
		l = &sync.Mutex{}
		c.locks[potID] = l
	}
	return l
}

// Check waits for any running check of the same pot, then runs one.
func (c *Checker) Check(ctx context.Context, potID int64, trig core.Trigger) (core.Advice, error) {
	l := c.potLock(potID)
	l.Lock()
	defer l.Unlock()
	return c.check(ctx, potID, trig)
}

// TryCheck reports false without checking when a check of this pot is already running.
func (c *Checker) TryCheck(ctx context.Context, potID int64, trig core.Trigger) (core.Advice, bool, error) {
	l := c.potLock(potID)
	if !l.TryLock() {
		return core.Advice{}, false, nil
	}
	defer l.Unlock()
	a, err := c.check(ctx, potID, trig)
	return a, true, err
}

func (c *Checker) check(ctx context.Context, potID int64, trig core.Trigger) (core.Advice, error) {
	st := c.cfg.Store
	now := c.cfg.Now()
	pot, err := st.GetPot(ctx, potID)
	if err != nil {
		return core.Advice{}, err
	}

	var last *core.RawPoint
	if pot.NodeID != nil && pot.Channel != nil {
		if last, err = st.LastRaw(ctx, *pot.NodeID, *pot.Channel); err != nil {
			return core.Advice{}, fmt.Errorf("last reading: %w", err)
		}
	}
	var lastTS *time.Time
	if last != nil {
		lastTS = &last.TS
	}

	info := c.cfg.Advisor.Info()
	adv := core.Advice{PotID: potID, CreatedAt: now, Trigger: trig, ProviderInfo: info}
	if status := core.Precheck(pot, lastTS, now, c.cfg.OfflineAfter); status != core.StatusOK {
		adv.Status = status
		return st.InsertAdvice(ctx, adv)
	}

	points, err := st.History(ctx, *pot.NodeID, *pot.Channel, now.Add(-historyWindow), now.Add(time.Second))
	if err != nil {
		return core.Advice{}, fmt.Errorf("history: %w", err)
	}
	if len(points) == 0 {
		// last reading is recent but outside the window only if the clock moved; treat as offline
		adv.Status = core.StatusSensorOffline
		return st.InsertAdvice(ctx, adv)
	}
	watering, err := st.LastWatering(ctx, potID)
	if err != nil {
		return core.Advice{}, fmt.Errorf("last watering: %w", err)
	}
	summary := core.Summarize(core.SummaryInput{Pot: pot, Now: now, Points: points, LastWatering: watering})
	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return core.Advice{}, fmt.Errorf("marshal summary: %w", err)
	}
	req := core.AdviceRequest{Summary: summary, Rules: pot.Rules}

	rules, err := core.RulesAdvisor{}.Advise(ctx, req)
	if err != nil {
		return core.Advice{}, fmt.Errorf("rules advisor: %w", err)
	}
	primary := rules
	if info.Provider != rulesProvider {
		rec, reason := c.askAdvisor(ctx, req, pot.PotML)
		if reason == "" {
			primary = rec
		} else {
			adv.Fallback = true
			adv.FallbackReason = reason
		}
	}

	adv.Status = core.StatusOK
	adv.Recommendation = &primary
	adv.Rules = &rules
	adv.Disagree = !adv.Fallback && core.Disagree(primary, rules)
	adv.Summary = summaryJSON
	return st.InsertAdvice(ctx, adv)
}

// askAdvisor makes up to maxAttempts tries; a non-empty reason means all failed.
func (c *Checker) askAdvisor(ctx context.Context, req core.AdviceRequest, potML int) (core.Recommendation, string) {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		rec, err := c.attempt(ctx, req, potML, attempt)
		if err == nil {
			return rec, ""
		}
		lastErr = err
	}
	reason := lastErr.Error()
	if len([]rune(reason)) > maxReasonLength {
		reason = string([]rune(reason)[:maxReasonLength])
	}
	c.cfg.Log.Warn("advisor failed, using rules fallback", "error", reason)
	return core.Recommendation{}, reason
}

func (c *Checker) attempt(ctx context.Context, req core.AdviceRequest, potML, attempt int) (core.Recommendation, error) {
	actx, cancel := context.WithTimeout(ctx, c.cfg.AttemptTimeout)
	defer cancel()
	start := time.Now()
	rec, err := c.cfg.Advisor.Advise(actx, req)
	if err == nil {
		err = core.ValidateRecommendation(rec, potML)
	}
	c.cfg.Log.Info("advisor attempt", "attempt", attempt, "latency_ms", time.Since(start).Milliseconds(), "ok", err == nil)
	return rec, err
}
