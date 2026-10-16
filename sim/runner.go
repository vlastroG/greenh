package sim

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/vlastrog/greenh/core"
)

const (
	maxBuffer       = 10000
	maxReadingAge   = 7*24*time.Hour - time.Hour
	minBackfillStep = 5 * time.Minute
	outOfRangeRaw   = 9999
)

type channelState struct {
	cfg    ChannelConfig
	soil   *Soil
	sensor *Sensor
	count  int // live readings produced, for out_of_range
	stuck  *int
}

type nodeState struct {
	id       string
	env      *Env
	channels []*channelState
}

type chanKey struct {
	node string
	ch   int
}

// Runner drives the simulated nodes against a Backend.
type Runner struct {
	cfg   Config
	b     Backend
	now   func() time.Time
	log   *slog.Logger
	start time.Time

	nodes  []*nodeState
	byChan map[chanKey]*channelState

	simNow time.Time // simulated clock for the environment, advances by interval×speed
	since  time.Time
	seen   map[int64]bool
	buffer []ReadingPayload
}

func NewRunner(cfg Config, b Backend, now func() time.Time, log *slog.Logger) *Runner {
	start := now()
	r := &Runner{
		cfg: cfg, b: b, now: now, log: log, start: start,
		byChan: map[chanKey]*channelState{},
		simNow: start,
		since:  start.Add(-time.Second),
		seen:   map[int64]bool{},
	}
	for ni, n := range cfg.Nodes {
		ns := &nodeState{id: n.ID, env: NewEnv(cfg.Seed + int64(ni))}
		for _, c := range n.Channels {
			cs := &channelState{
				cfg:    c,
				soil:   NewSoil(c.Category, c.PotML, c.StartMoisture),
				sensor: NewSensor(c.DryRaw, c.WetRaw, cfg.Seed+int64(ni)*1000+int64(c.Ch)),
			}
			ns.channels = append(ns.channels, cs)
			r.byChan[chanKey{n.ID, c.Ch}] = cs
		}
		r.nodes = append(r.nodes, ns)
	}
	return r
}

// Backfill posts past readings at speed 1, starting cfg.Backfill ago.
func (r *Runner) Backfill(ctx context.Context) error {
	span := r.cfg.Backfill.Duration
	if span <= 0 {
		return nil
	}
	step := max(r.cfg.Interval.Duration, minBackfillStep)
	end := r.now()
	for t := end.Add(-span); !t.After(end); t = t.Add(step) {
		for _, n := range r.nodes {
			amb := n.env.At(t)
			for _, c := range n.channels {
				if t != end.Add(-span) {
					c.soil.Step(step, amb)
				}
			}
			r.enqueue(n.payload(t, amb, func(c *channelState) int {
				return c.sensor.Raw(c.soil.Moisture())
			}))
		}
	}
	r.flush(ctx)
	return nil
}

// Tick runs one interval: waterings, physics, then delivery.
func (r *Runner) Tick(ctx context.Context) {
	r.applyWaterings(ctx)

	dt := time.Duration(float64(r.cfg.Interval.Duration) * r.cfg.Speed)
	r.simNow = r.simNow.Add(dt)
	ts := r.now()
	for _, n := range r.nodes {
		amb := n.env.At(r.simNow)
		for _, c := range n.channels {
			c.soil.Step(dt, amb)
		}
		p := n.payload(ts, amb, r.liveRaw(n.id))
		if r.inDropout(n.id, ts) {
			continue
		}
		r.enqueue(p)
	}
	r.flush(ctx)
}

func (r *Runner) Run(ctx context.Context) error {
	if err := r.Backfill(ctx); err != nil {
		return err
	}
	t := time.NewTicker(r.cfg.Interval.Duration)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			r.Tick(ctx)
		}
	}
}

// liveRaw returns the raw value for a channel, applying stuck and out_of_range failures.
func (r *Runner) liveRaw(node string) func(*channelState) int {
	return func(c *channelState) int {
		raw := c.sensor.Raw(c.soil.Moisture())
		c.count++
		if f := r.cfg.Failures.Stuck; f != nil && f.Node == node && f.Ch == c.cfg.Ch {
			if c.stuck == nil {
				c.stuck = &raw
			}
			raw = *c.stuck
		}
		if f := r.cfg.Failures.OutOfRange; f != nil && f.Node == node && f.Ch == c.cfg.Ch && c.count%f.Every == 0 {
			raw = outOfRangeRaw
		}
		return raw
	}
}

func (r *Runner) inDropout(node string, t time.Time) bool {
	f := r.cfg.Failures.Dropout
	if f == nil || f.Node != node {
		return false
	}
	from := r.start.Add(f.After.Duration)
	return !t.Before(from) && t.Before(from.Add(f.For.Duration))
}

func (n *nodeState) payload(ts time.Time, amb core.Ambient, raw func(*channelState) int) ReadingPayload {
	p := ReadingPayload{
		NodeID:  n.id,
		TS:      ts,
		Ambient: AmbientPayload{TempC: amb.TempC, HumidityPct: amb.HumidityPct, LightLux: amb.LightLux},
	}
	for _, c := range n.channels {
		p.Channels = append(p.Channels, ChannelPayload{Ch: c.cfg.Ch, MoistureRaw: raw(c)})
	}
	return p
}

func (r *Runner) applyWaterings(ctx context.Context) {
	events, err := r.b.WateringsSince(ctx, r.since)
	if err != nil {
		r.log.Warn("poll waterings failed", "err", err)
		return
	}
	for _, e := range events {
		if e.CreatedAt.After(r.since) {
			r.since = e.CreatedAt
		}
		if r.seen[e.ID] {
			continue
		}
		r.seen[e.ID] = true
		if e.NodeID == nil || e.Channel == nil {
			continue
		}
		if c := r.byChan[chanKey{*e.NodeID, *e.Channel}]; c != nil {
			c.soil.Water(e.AmountML)
			r.log.Info("watering applied", "node", *e.NodeID, "ch", *e.Channel, "ml", e.AmountML)
		}
	}
}

func (r *Runner) enqueue(p ReadingPayload) {
	if len(r.buffer) >= maxBuffer {
		r.log.Warn("buffer full, dropping oldest reading")
		r.buffer = r.buffer[1:]
	}
	r.buffer = append(r.buffer, p)
}

// flush sends buffered readings oldest-first, stopping at the first retryable failure.
func (r *Runner) flush(ctx context.Context) {
	cutoff := r.now().Add(-maxReadingAge)
	for len(r.buffer) > 0 {
		p := r.buffer[0]
		if p.TS.Before(cutoff) {
			r.log.Warn("dropping expired reading", "node", p.NodeID, "ts", p.TS)
			r.buffer = r.buffer[1:]
			continue
		}
		if err := r.b.PostReading(ctx, p); err != nil {
			if !retryable(err) {
				r.log.Error("backend rejected reading, dropping", "node", p.NodeID, "ts", p.TS, "err", err)
				r.buffer = r.buffer[1:]
				continue
			}
			r.log.Warn("post failed, buffering", "buffered", len(r.buffer), "err", err)
			return
		}
		r.buffer = r.buffer[1:]
	}
}

// retryable reports whether a failed post should be kept for a later resend.
func retryable(err error) bool {
	var he *HTTPError
	if errors.As(err, &he) {
		return he.Status >= 500 || he.Status == 429
	}
	return true
}
