package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"github.com/vlastrog/greenh/api"
	"github.com/vlastrog/greenh/core"
	"github.com/vlastrog/greenh/ingest"
	"github.com/vlastrog/greenh/scheduler"
	"github.com/vlastrog/greenh/store/sqlite"
	"github.com/vlastrog/greenh/store/sqlite/sqlitetest"
)

var testNow = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

func newTestAPI(t *testing.T) (*httptest.Server, *sqlite.Store) {
	t.Helper()
	st := sqlitetest.New(t)
	now := func() time.Time { return testNow }
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	checker := scheduler.NewChecker(scheduler.CheckerConfig{
		Store: st, Advisor: core.RulesAdvisor{}, Now: now,
		OfflineAfter: 30 * time.Minute, AttemptTimeout: 30 * time.Second, Log: log,
	})
	srv := httptest.NewServer(api.NewHandler(api.Deps{
		Store: st, Ingest: ingest.New(st, now, log), Checker: checker,
		Advisor: core.RulesAdvisor{}.Info(), OfflineAfter: 30 * time.Minute, Now: now, Log: log,
	}))
	t.Cleanup(srv.Close)
	return srv, st
}

// do sends body (a string is sent verbatim, anything else as JSON) to the absolute URL
// and fails the test unless the response matches the OpenAPI spec.
func do(t *testing.T, method, rawURL string, body any) *http.Response {
	t.Helper()
	var payload []byte
	switch b := body.(type) {
	case nil:
	case string:
		payload = []byte(b)
	default:
		var err error
		if payload, err = json.Marshal(b); err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}
	req, err := http.NewRequest(method, rawURL, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, rawURL, err)
	}
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	resp.Body = io.NopCloser(bytes.NewReader(respBody))

	validateResponse(t, req, resp, respBody)
	return resp
}

func validateResponse(t *testing.T, req *http.Request, resp *http.Response, body []byte) {
	t.Helper()
	ctx := context.Background()
	doc, err := api.GetSwagger()
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	if err := doc.Validate(ctx); err != nil {
		t.Fatalf("spec invalid: %v", err)
	}
	u, _ := url.Parse(req.URL.String())
	doc.Servers = openapi3.Servers{{URL: u.Scheme + "://" + u.Host + "/api/v1"}}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	route, params, err := router.FindRoute(req)
	if err != nil {
		t.Fatalf("find route %s %s: %v", req.Method, req.URL, err)
	}
	err = openapi3filter.ValidateResponse(ctx, &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route},
		Status:                 resp.StatusCode,
		Header:                 resp.Header,
		Body:                   io.NopCloser(bytes.NewReader(body)),
		Options:                &openapi3filter.Options{IncludeResponseStatus: true},
	})
	if err != nil {
		t.Fatalf("%s %s: response %d does not match spec: %v\nbody: %s", req.Method, req.URL.Path, resp.StatusCode, err, body)
	}
}

// expect fails unless resp has the status, then decodes the body into T.
func expect[T any](t *testing.T, resp *http.Response, status int) T {
	t.Helper()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != status {
		t.Fatalf("status %d, want %d; body: %s", resp.StatusCode, status, body)
	}
	var v T
	if len(body) > 0 {
		if err := json.Unmarshal(body, &v); err != nil {
			t.Fatalf("decode %T: %v; body: %s", v, err, body)
		}
	}
	return v
}

func expectError(t *testing.T, resp *http.Response, status int, code api.ErrorCode) api.Error {
	t.Helper()
	e := expect[api.Error](t, resp, status)
	if e.Code != code || e.Message == "" {
		t.Fatalf("error = %+v, want code %q with a message", e, code)
	}
	return e
}

func postReading(t *testing.T, srv *httptest.Server, node string, ts time.Time, raws map[int]int) {
	t.Helper()
	chans := []map[string]int{}
	for ch, raw := range raws {
		chans = append(chans, map[string]int{"ch": ch, "moisture_raw": raw})
	}
	resp := do(t, http.MethodPost, srv.URL+"/api/v1/readings", map[string]any{
		"node_id": node, "ts": ts, "ambient": map[string]any{"temp_c": 21.5}, "channels": chans,
	})
	if got := expect[api.ReadingAccepted](t, resp, http.StatusAccepted); !got.Stored {
		t.Fatalf("reading at %v not stored", ts)
	}
}

func createPot(t *testing.T, srv *httptest.Server, body map[string]any) api.Pot {
	t.Helper()
	full := map[string]any{"name": "Ficus", "category": "foliage", "pot_ml": 2000}
	for k, v := range body {
		full[k] = v
	}
	return expect[api.Pot](t, do(t, http.MethodPost, srv.URL+"/api/v1/pots", full), http.StatusCreated)
}

func potURL(srv *httptest.Server, id int64, suffix string) string {
	return fmt.Sprintf("%s/api/v1/pots/%d%s", srv.URL, id, suffix)
}

func ts(t time.Time) string { return url.QueryEscape(t.Format(time.RFC3339)) }
