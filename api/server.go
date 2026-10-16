package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/vlastrog/greenh/core"
	"github.com/vlastrog/greenh/ingest"
)

const (
	basePath         = "/api/v1"
	defaultWindow    = 24 * time.Hour
	adviceHistoryMax = 50
	maxWateringAge   = 7 * 24 * time.Hour
	maxWateringAhead = 5 * time.Minute
	defaultSource    = "manual"
)

type Deps struct {
	Store   core.Store
	Ingest  *ingest.Service
	Checker interface {
		Check(context.Context, int64, core.Trigger) (core.Advice, error)
	}
	Advisor core.ProviderInfo
	Now     func() time.Time
	Log     *slog.Logger
}

type server struct{ d Deps }

var _ StrictServerInterface = (*server)(nil)

// NewHandler serves the API under /api/v1; every error response has the Error shape.
func NewHandler(d Deps) http.Handler {
	s := &server{d: d}
	mux := http.NewServeMux()
	mux.HandleFunc(basePath+"/", s.routeNotFound)
	strict := NewStrictHandlerWithOptions(s, nil, StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  s.requestError,
		ResponseErrorHandlerFunc: s.responseError,
	})
	return HandlerWithOptions(strict, StdHTTPServerOptions{
		BaseURL:          basePath,
		BaseRouter:       mux,
		ErrorHandlerFunc: s.requestError,
	})
}

func (s *server) GetHealth(context.Context, GetHealthRequestObject) (GetHealthResponseObject, error) {
	return GetHealth200JSONResponse{
		Status:          HealthStatusOk,
		AdvisorProvider: s.d.Advisor.Provider,
		AdvisorModel:    s.d.Advisor.Model,
	}, nil
}

func (s *server) PostReadings(ctx context.Context, req PostReadingsRequestObject) (PostReadingsResponseObject, error) {
	stored, err := s.d.Ingest.Ingest(ctx, toReading(*req.Body))
	if err != nil {
		return nil, err
	}
	return PostReadings202JSONResponse{Stored: stored}, nil
}

func (s *server) ListNodes(ctx context.Context, _ ListNodesRequestObject) (ListNodesResponseObject, error) {
	nodes, err := s.d.Store.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	out := make(ListNodes200JSONResponse, len(nodes))
	for i, n := range nodes {
		out[i] = toNode(n)
	}
	return out, nil
}

func (s *server) PatchNode(ctx context.Context, req PatchNodeRequestObject) (PatchNodeResponseObject, error) {
	if n := utf8.RuneCountInString(req.Body.Name); n < 1 || n > 64 {
		return nil, validationError(core.FieldError{Field: "name", Message: "must be 1-64 characters"})
	}
	node, err := s.d.Store.RenameNode(ctx, req.Id, req.Body.Name)
	if err != nil {
		return nil, notFound(err, "node", req.Id)
	}
	return PatchNode200JSONResponse(toNode(node)), nil
}

func (s *server) ListPotWaterings(ctx context.Context, req ListPotWateringsRequestObject) (ListPotWateringsResponseObject, error) {
	if _, err := s.getPot(ctx, req.Id); err != nil {
		return nil, err
	}
	ws, err := s.d.Store.ListWaterings(ctx, req.Id, 0)
	if err != nil {
		return nil, err
	}
	out := make(ListPotWaterings200JSONResponse, len(ws))
	for i, w := range ws {
		out[i] = toWatering(w)
	}
	return out, nil
}

func (s *server) CreatePotWatering(ctx context.Context, req CreatePotWateringRequestObject) (CreatePotWateringResponseObject, error) {
	if _, err := s.getPot(ctx, req.Id); err != nil {
		return nil, err
	}
	now := s.d.Now()
	w := core.Watering{PotID: req.Id, TS: now, CreatedAt: now, AmountML: req.Body.AmountMl, Source: defaultSource}
	if req.Body.Ts != nil {
		w.TS = *req.Body.Ts
	}
	if req.Body.Source != nil && *req.Body.Source != "" {
		w.Source = *req.Body.Source
	}

	var errs []core.FieldError
	if w.AmountML < 1 || w.AmountML > 20000 {
		errs = append(errs, core.FieldError{Field: "amount_ml", Message: "must be between 1 and 20000"})
	}
	if w.TS.Before(now.Add(-maxWateringAge)) {
		errs = append(errs, core.FieldError{Field: "ts", Message: "must not be more than 7 days in the past"})
	} else if w.TS.After(now.Add(maxWateringAhead)) {
		errs = append(errs, core.FieldError{Field: "ts", Message: "must not be more than 5 minutes in the future"})
	}
	if len(errs) > 0 {
		return nil, validationError(errs...)
	}

	created, err := s.d.Store.CreateWatering(ctx, w)
	if err != nil {
		return nil, err
	}
	return CreatePotWatering201JSONResponse(toWatering(created)), nil
}

func (s *server) ListWaterings(ctx context.Context, req ListWateringsRequestObject) (ListWateringsResponseObject, error) {
	since := s.d.Now().Add(-defaultWindow)
	if req.Params.Since != nil {
		since = *req.Params.Since
	}
	ws, err := s.d.Store.WateringsSince(ctx, since)
	if err != nil {
		return nil, err
	}
	out := make(ListWaterings200JSONResponse, len(ws))
	for i, w := range ws {
		out[i] = toFeedItem(w)
	}
	return out, nil
}

func (s *server) GetPotAdvice(ctx context.Context, req GetPotAdviceRequestObject) (GetPotAdviceResponseObject, error) {
	if _, err := s.getPot(ctx, req.Id); err != nil {
		return nil, err
	}
	history, err := s.d.Store.ListAdvice(ctx, req.Id, adviceHistoryMax)
	if err != nil {
		return nil, err
	}
	list, err := toAdviceList(history)
	if err != nil {
		return nil, err
	}
	return GetPotAdvice200JSONResponse(list), nil
}

func (s *server) CheckPotNow(ctx context.Context, req CheckPotNowRequestObject) (CheckPotNowResponseObject, error) {
	// the checker keeps per-pot state, so unknown ids must not reach it
	if _, err := s.getPot(ctx, req.Id); err != nil {
		return nil, err
	}
	a, err := s.d.Checker.Check(ctx, req.Id, core.TriggerManual)
	if err != nil {
		return nil, err
	}
	out, err := toAdvice(a)
	if err != nil {
		return nil, err
	}
	return CheckPotNow200JSONResponse(out), nil
}
