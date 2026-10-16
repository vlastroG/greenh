package api

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/vlastrog/greenh/core"
)

const (
	defaultCheckInterval = 360 * time.Minute
	maxHistorySpan       = 31 * 24 * time.Hour
	maxStepSeconds       = 86400
)

func (s *server) getPot(ctx context.Context, id int64) (core.Pot, error) {
	p, err := s.d.Store.GetPot(ctx, id)
	return p, notFound(err, "pot", id)
}

// potView adds the pot's current reading and latest advice.
func (s *server) potView(ctx context.Context, p core.Pot) (Pot, error) {
	var last *core.RawPoint
	if p.NodeID != nil && p.Channel != nil {
		var err error
		if last, err = s.d.Store.LastRaw(ctx, *p.NodeID, *p.Channel); err != nil {
			return Pot{}, err
		}
	}
	latest, err := s.d.Store.LatestAdvice(ctx, p.ID)
	if err != nil {
		return Pot{}, err
	}
	var lastAt *time.Time
	if last != nil {
		lastAt = &last.TS
	}
	status := core.Precheck(p, lastAt, s.d.Now(), s.d.OfflineAfter)
	return toPot(p, last, latest, status)
}

func (s *server) ListPots(ctx context.Context, _ ListPotsRequestObject) (ListPotsResponseObject, error) {
	pots, err := s.d.Store.ListPots(ctx)
	if err != nil {
		return nil, err
	}
	out := make(ListPots200JSONResponse, len(pots))
	for i, p := range pots {
		if out[i], err = s.potView(ctx, p); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *server) GetPot(ctx context.Context, req GetPotRequestObject) (GetPotResponseObject, error) {
	p, err := s.getPot(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	view, err := s.potView(ctx, p)
	if err != nil {
		return nil, err
	}
	return GetPot200JSONResponse(view), nil
}

func (s *server) CreatePot(ctx context.Context, req CreatePotRequestObject) (CreatePotResponseObject, error) {
	b := req.Body
	p := core.Pot{
		Name:          b.Name,
		Category:      core.Category(b.Category),
		PotML:         b.PotMl,
		NodeID:        b.NodeId,
		Channel:       b.Channel,
		DryRaw:        b.DryRaw,
		WetRaw:        b.WetRaw,
		Rules:         core.DefaultRules(core.Category(b.Category)),
		CheckInterval: defaultCheckInterval,
		CreatedAt:     s.d.Now(),
	}
	setIf(&p.Species, b.Species)
	setIf(&p.SoilType, b.SoilType)
	setIf(&p.Placement, b.Placement)
	setIf(&p.Rules.ThresholdPct, b.ThresholdPct)
	setIf(&p.Rules.AmountFraction, b.AmountFraction)
	if b.CheckIntervalMin != nil {
		p.CheckInterval = time.Duration(*b.CheckIntervalMin) * time.Minute
	}
	if errs := validatePot(p); len(errs) > 0 {
		return nil, validationError(errs...)
	}

	created, err := s.d.Store.CreatePot(ctx, p)
	if err != nil {
		return nil, linkConflict(err, p)
	}
	view, err := s.potView(ctx, created)
	if err != nil {
		return nil, err
	}
	return CreatePot201JSONResponse(view), nil
}

func (s *server) PatchPot(ctx context.Context, req PatchPotRequestObject) (PatchPotResponseObject, error) {
	p, err := s.getPot(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	b := req.Body
	setIf(&p.Name, b.Name)
	setIf(&p.Species, b.Species)
	setIf(&p.PotML, b.PotMl)
	setIf(&p.SoilType, b.SoilType)
	setIf(&p.Placement, b.Placement)
	setIf(&p.Rules.ThresholdPct, b.ThresholdPct)
	setIf(&p.Rules.AmountFraction, b.AmountFraction)
	if b.Category != nil {
		p.Category = core.Category(*b.Category)
	}
	if b.CheckIntervalMin != nil {
		p.CheckInterval = time.Duration(*b.CheckIntervalMin) * time.Minute
	}
	applyNullable(&p.NodeID, b.NodeId)
	applyNullable(&p.Channel, b.Channel)
	applyNullable(&p.DryRaw, b.DryRaw)
	applyNullable(&p.WetRaw, b.WetRaw)
	if errs := validatePot(p); len(errs) > 0 {
		return nil, validationError(errs...)
	}

	updated, err := s.d.Store.UpdatePot(ctx, p)
	if err != nil {
		return nil, notFound(linkConflict(err, p), "pot", p.ID)
	}
	view, err := s.potView(ctx, updated)
	if err != nil {
		return nil, err
	}
	return PatchPot200JSONResponse(view), nil
}

func (s *server) DeletePot(ctx context.Context, req DeletePotRequestObject) (DeletePotResponseObject, error) {
	if err := s.d.Store.DeletePot(ctx, req.Id); err != nil {
		return nil, notFound(err, "pot", req.Id)
	}
	return DeletePot204Response{}, nil
}

func (s *server) GetPotReadings(ctx context.Context, req GetPotReadingsRequestObject) (GetPotReadingsResponseObject, error) {
	p, err := s.getPot(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	now := s.d.Now()
	from, to, step := now.Add(-defaultWindow), now, 0
	setIf(&from, req.Params.From)
	setIf(&to, req.Params.To)
	setIf(&step, req.Params.Step)

	var errs []core.FieldError
	if step < 0 || step > maxStepSeconds {
		errs = append(errs, core.FieldError{Field: "step", Message: "must be between 0 and 86400 seconds"})
	}
	if to.Before(from) {
		errs = append(errs, core.FieldError{Field: "to", Message: "must not be before from"})
	} else if to.Sub(from) > maxHistorySpan {
		errs = append(errs, core.FieldError{Field: "to", Message: "must be at most 31 days after from"})
	}
	if len(errs) > 0 {
		return nil, validationError(errs...)
	}

	if p.NodeID == nil || p.Channel == nil {
		return GetPotReadings200JSONResponse{}, nil
	}
	raw, err := s.d.Store.History(ctx, *p.NodeID, *p.Channel, from, to)
	if err != nil {
		return nil, err
	}
	points := make([]core.HistoryPoint, len(raw))
	for i, rp := range raw {
		points[i] = toHistoryPoint(p, rp)
	}
	points = core.Downsample(points, time.Duration(step)*time.Second)
	return GetPotReadings200JSONResponse(fromHistoryPoints(points)), nil
}

// validatePot checks a pot after defaults or a patch have been applied.
func validatePot(p core.Pot) []core.FieldError {
	var errs []core.FieldError
	add := func(field, msg string) { errs = append(errs, core.FieldError{Field: field, Message: msg}) }

	if n := utf8.RuneCountInString(p.Name); n < 1 || n > 64 {
		add("name", "must be 1-64 characters")
	}
	if !p.Category.Valid() {
		add("category", "must be succulent, foliage, flowering or herb")
	}
	if p.PotML < 50 || p.PotML > 50000 {
		add("pot_ml", "must be between 50 and 50000")
	}
	if p.Rules.ThresholdPct < 0 || p.Rules.ThresholdPct > 100 {
		add("threshold_pct", "must be between 0 and 100")
	}
	if p.Rules.AmountFraction <= 0 || p.Rules.AmountFraction > 0.5 {
		add("amount_fraction", "must be greater than 0 and at most 0.5")
	}
	if m := p.CheckInterval / time.Minute; m < 15 || m > 10080 {
		add("check_interval_min", "must be between 15 and 10080")
	}

	if p.NodeID != nil && utf8.RuneCountInString(*p.NodeID) == 0 {
		add("node_id", "must not be empty")
	}
	if (p.NodeID == nil) != (p.Channel == nil) {
		add("channel", "node_id and channel must be set together")
	} else if p.Channel != nil && (*p.Channel < 0 || *p.Channel > 15) {
		add("channel", "must be between 0 and 15")
	}

	if (p.DryRaw == nil) != (p.WetRaw == nil) {
		add("dry_raw", "dry_raw and wet_raw must be set together")
	} else if p.DryRaw != nil && *p.DryRaw == *p.WetRaw {
		add("wet_raw", "dry and wet calibration values must differ")
	}
	return errs
}

// linkConflict names the contested link in a store conflict.
func linkConflict(err error, p core.Pot) error {
	if !errors.Is(err, core.ErrConflict) || p.NodeID == nil || p.Channel == nil {
		return err
	}
	return conflictError{msg: fmt.Sprintf("node %s channel %d is already linked to another pot", *p.NodeID, *p.Channel)}
}

func setIf[T any](dst *T, v *T) {
	if v != nil {
		*dst = *v
	}
}
