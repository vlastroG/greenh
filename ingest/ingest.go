// Package ingest validates and stores incoming sensor readings.
package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/vlastrog/greenh/core"
)

const (
	maxAge    = 7 * 24 * time.Hour
	maxFuture = 5 * time.Minute
)

var nodeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type Service struct {
	store core.ReadingStore
	now   func() time.Time
	log   *slog.Logger
}

func New(store core.ReadingStore, now func() time.Time, log *slog.Logger) *Service {
	return &Service{store: store, now: now, log: log}
}

// Ingest returns a *core.ValidationError on bad input and stored=false on a duplicate.
func (s *Service) Ingest(ctx context.Context, r core.Reading) (bool, error) {
	now := s.now()
	if r.TS.IsZero() {
		r.TS = now
	}
	if errs := validate(r, now); len(errs) > 0 {
		verr := &core.ValidationError{Fields: errs}
		s.log.Warn("reading rejected", "node_id", r.NodeID, "reason", verr.Error())
		return false, verr
	}
	return s.store.InsertReading(ctx, r)
}

func validate(r core.Reading, now time.Time) []core.FieldError {
	var errs []core.FieldError
	add := func(field, msg string) { errs = append(errs, core.FieldError{Field: field, Message: msg}) }

	if n := len(r.NodeID); n < 1 || n > 64 || !nodeIDPattern.MatchString(r.NodeID) {
		add("node_id", "must be 1-64 characters of letters, digits, '_' or '-'")
	}
	if r.TS.Before(now.Add(-maxAge)) {
		add("ts", "must not be more than 7 days in the past")
	} else if r.TS.After(now.Add(maxFuture)) {
		add("ts", "must not be more than 5 minutes in the future")
	}

	if len(r.Channels) == 0 {
		add("channels", "at least one channel is required")
	}
	seen := map[int]bool{}
	for i, c := range r.Channels {
		if c.Ch < 0 || c.Ch > 15 {
			add(fmt.Sprintf("channels[%d].ch", i), "must be between 0 and 15")
		} else if seen[c.Ch] {
			add(fmt.Sprintf("channels[%d].ch", i), "duplicate channel")
		}
		seen[c.Ch] = true
		if c.MoistureRaw < 0 || c.MoistureRaw > 4095 {
			add(fmt.Sprintf("channels[%d].moisture_raw", i), "must be between 0 and 4095")
		}
	}

	checkRange := func(field string, v *float64, lo, hi float64) {
		if v != nil && (*v < lo || *v > hi) {
			add(field, fmt.Sprintf("must be between %g and %g", lo, hi))
		}
	}
	checkRange("ambient.temp_c", r.Ambient.TempC, -40, 85)
	checkRange("ambient.humidity_pct", r.Ambient.HumidityPct, 0, 100)
	checkRange("ambient.light_lux", r.Ambient.LightLux, 0, 200000)
	return errs
}
