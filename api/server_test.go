package api_test

import (
	"context"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/oapi-codegen/nullable"

	"github.com/vlastrog/greenh/api"
	"github.com/vlastrog/greenh/core"
)

// val fails unless n holds a value.
func val[T any](t *testing.T, n nullable.Nullable[T], name string) T {
	t.Helper()
	v, err := n.Get()
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return v
}

func isNull[T any](t *testing.T, n nullable.Nullable[T], name string) {
	t.Helper()
	if !n.IsNull() {
		t.Fatalf("%s = %v, want null", name, n)
	}
}

func TestPostReadingsAndAutoRegister(t *testing.T) {
	srv, _ := newTestAPI(t)
	postReading(t, srv, "node-1", testNow.Add(-time.Minute), map[int]int{0: 500, 1: 600})

	nodes := expect[[]api.Node](t, do(t, http.MethodGet, srv.URL+"/api/v1/nodes", nil), http.StatusOK)
	if len(nodes) != 1 || nodes[0].Id != "node-1" || len(nodes[0].Channels) != 2 {
		t.Fatalf("nodes = %+v", nodes)
	}
	for i, want := range []int{500, 600} {
		c := nodes[0].Channels[i]
		if c.Ch != i || val(t, c.LastRaw, "last_raw") != want {
			t.Errorf("channel %d = %+v, want last_raw %d", i, c, want)
		}
		isNull(t, c.PotId, "pot_id")
	}

	renamed := expect[api.Node](t, do(t, http.MethodPatch, srv.URL+"/api/v1/nodes/node-1",
		map[string]any{"name": "Shelf"}), http.StatusOK)
	if renamed.Name != "Shelf" {
		t.Errorf("renamed = %+v", renamed)
	}
	expectError(t, do(t, http.MethodPatch, srv.URL+"/api/v1/nodes/nope",
		map[string]any{"name": "x"}), http.StatusNotFound, api.ErrorCodeNotFound)
}

func TestPostReadingsInvalid(t *testing.T) {
	srv, _ := newTestAPI(t)
	resp := do(t, http.MethodPost, srv.URL+"/api/v1/readings", map[string]any{
		"node_id": "node-1", "ts": testNow, "channels": []map[string]int{{"ch": 0, "moisture_raw": 5000}},
	})
	e := expectError(t, resp, http.StatusBadRequest, api.ErrorCodeInvalidRequest)
	if e.Details == nil || len(*e.Details) == 0 || (*e.Details)[0].Field != "channels[0].moisture_raw" {
		t.Fatalf("details = %+v", e.Details)
	}
}

func TestMalformedJSON(t *testing.T) {
	srv, _ := newTestAPI(t)
	expectError(t, do(t, http.MethodPost, srv.URL+"/api/v1/readings", "{"), http.StatusBadRequest, api.ErrorCodeInvalidRequest)
	expectError(t, do(t, http.MethodPost, srv.URL+"/api/v1/pots", "{"), http.StatusBadRequest, api.ErrorCodeInvalidRequest)
	expectError(t, do(t, http.MethodGet, srv.URL+"/api/v1/pots/abc", nil), http.StatusBadRequest, api.ErrorCodeInvalidRequest)
}

func TestUnknownRoute(t *testing.T) {
	srv, _ := newTestAPI(t)
	resp, err := http.Get(srv.URL + "/api/v1/nope")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	expectError(t, resp, http.StatusNotFound, api.ErrorCodeNotFound)
}

func TestPotLifecycle(t *testing.T) {
	srv, _ := newTestAPI(t)
	created := createPot(t, srv, nil)
	if created.ThresholdPct != 30 || created.AmountFraction != 0.2 || created.CheckIntervalMin != 360 {
		t.Fatalf("defaults = %v/%v/%v, want 30/0.2/360", created.ThresholdPct, created.AmountFraction, created.CheckIntervalMin)
	}
	if !created.CreatedAt.Equal(testNow) {
		t.Errorf("created_at = %v", created.CreatedAt)
	}
	isNull(t, created.NodeId, "node_id")
	isNull(t, created.Current.MoisturePct, "current.moisture_pct")
	isNull(t, created.LatestAdvice, "latest_advice")

	got := expect[api.Pot](t, do(t, http.MethodGet, potURL(srv, created.Id, ""), nil), http.StatusOK)
	if got.Name != "Ficus" || got.Id != created.Id {
		t.Fatalf("get = %+v", got)
	}
	list := expect[[]api.Pot](t, do(t, http.MethodGet, srv.URL+"/api/v1/pots", nil), http.StatusOK)
	if len(list) != 1 {
		t.Fatalf("list len %d", len(list))
	}

	patched := expect[api.Pot](t, do(t, http.MethodPatch, potURL(srv, created.Id, ""),
		map[string]any{"name": "Big ficus"}), http.StatusOK)
	if patched.Name != "Big ficus" || patched.ThresholdPct != 30 || patched.PotMl != 2000 {
		t.Fatalf("patched = %+v", patched)
	}

	expect[struct{}](t, do(t, http.MethodDelete, potURL(srv, created.Id, ""), nil), http.StatusNoContent)
	expectError(t, do(t, http.MethodGet, potURL(srv, created.Id, ""), nil), http.StatusNotFound, api.ErrorCodeNotFound)
	expectError(t, do(t, http.MethodDelete, potURL(srv, created.Id, ""), nil), http.StatusNotFound, api.ErrorCodeNotFound)
}

func TestEmptyListsAreArrays(t *testing.T) {
	srv, _ := newTestAPI(t)
	for _, path := range []string{"/api/v1/pots", "/api/v1/nodes", "/api/v1/waterings"} {
		resp := do(t, http.MethodGet, srv.URL+path, nil)
		body := expect[any](t, resp, http.StatusOK)
		if arr, ok := body.([]any); !ok || len(arr) != 0 {
			t.Errorf("%s = %#v, want []", path, body)
		}
	}
	pot := createPot(t, srv, nil)
	for _, suffix := range []string{"/waterings", "/readings"} {
		body := expect[any](t, do(t, http.MethodGet, potURL(srv, pot.Id, suffix), nil), http.StatusOK)
		if arr, ok := body.([]any); !ok || len(arr) != 0 {
			t.Errorf("%s = %#v, want []", suffix, body)
		}
	}
	adv := expect[map[string]any](t, do(t, http.MethodGet, potURL(srv, pot.Id, "/advice"), nil), http.StatusOK)
	if arr, ok := adv["history"].([]any); !ok || len(arr) != 0 || adv["latest"] != nil {
		t.Errorf("advice = %#v", adv)
	}
}

func TestPotLinkConflict(t *testing.T) {
	srv, _ := newTestAPI(t)
	first := createPot(t, srv, map[string]any{"node_id": "node-1", "channel": 0})
	resp := do(t, http.MethodPost, srv.URL+"/api/v1/pots",
		map[string]any{"name": "Second", "category": "herb", "pot_ml": 500, "node_id": "node-1", "channel": 0})
	e := expectError(t, resp, http.StatusConflict, api.ErrorCodeConflict)
	if e.Message != "node node-1 channel 0 is already linked to another pot" {
		t.Fatalf("message = %q", e.Message)
	}

	second := createPot(t, srv, map[string]any{"name": "Second", "node_id": "node-1", "channel": 1})
	resp = do(t, http.MethodPatch, potURL(srv, second.Id, ""), map[string]any{"channel": 0})
	expectError(t, resp, http.StatusConflict, api.ErrorCodeConflict)

	// re-saving a pot's own link is not a conflict
	expect[api.Pot](t, do(t, http.MethodPatch, potURL(srv, first.Id, ""),
		map[string]any{"node_id": "node-1", "channel": 0}), http.StatusOK)
}

func TestPotCalibrationValidation(t *testing.T) {
	srv, _ := newTestAPI(t)
	pot := createPot(t, srv, nil)

	e := expectError(t, do(t, http.MethodPatch, potURL(srv, pot.Id, ""),
		map[string]any{"dry_raw": 500, "wet_raw": 500}), http.StatusBadRequest, api.ErrorCodeInvalidRequest)
	if e.Details == nil || (*e.Details)[0].Message != "dry and wet calibration values must differ" {
		t.Fatalf("details = %+v", e.Details)
	}
	expectError(t, do(t, http.MethodPost, srv.URL+"/api/v1/pots",
		map[string]any{"name": "x", "category": "herb", "pot_ml": 500, "dry_raw": 500, "wet_raw": 500}),
		http.StatusBadRequest, api.ErrorCodeInvalidRequest)
	expectError(t, do(t, http.MethodPatch, potURL(srv, pot.Id, ""),
		map[string]any{"dry_raw": 500}), http.StatusBadRequest, api.ErrorCodeInvalidRequest)

	reversed := expect[api.Pot](t, do(t, http.MethodPatch, potURL(srv, pot.Id, ""),
		map[string]any{"dry_raw": 270, "wet_raw": 540}), http.StatusOK)
	if val(t, reversed.DryRaw, "dry_raw") != 270 || val(t, reversed.WetRaw, "wet_raw") != 540 {
		t.Fatalf("reversed = %+v", reversed)
	}
}

func TestPotFieldValidation(t *testing.T) {
	srv, _ := newTestAPI(t)
	cases := map[string]struct {
		body  map[string]any
		field string
	}{
		"empty name":       {map[string]any{"name": ""}, "name"},
		"long name":        {map[string]any{"name": strings.Repeat("a", 65)}, "name"},
		"bad category":     {map[string]any{"category": "cactus"}, "category"},
		"small pot":        {map[string]any{"pot_ml": 49}, "pot_ml"},
		"threshold > 100":  {map[string]any{"threshold_pct": 101}, "threshold_pct"},
		"fraction zero":    {map[string]any{"amount_fraction": 0}, "amount_fraction"},
		"fraction > 0.5":   {map[string]any{"amount_fraction": 0.6}, "amount_fraction"},
		"interval < 15":    {map[string]any{"check_interval_min": 14}, "check_interval_min"},
		"interval > week":  {map[string]any{"check_interval_min": 10081}, "check_interval_min"},
		"channel > 15":     {map[string]any{"node_id": "n", "channel": 16}, "channel"},
		"channel only":     {map[string]any{"channel": 1}, "channel"},
		"node only":        {map[string]any{"node_id": "n"}, "channel"},
		"wet without dry":  {map[string]any{"wet_raw": 300}, "dry_raw"},
		"dry equal to wet": {map[string]any{"dry_raw": 300, "wet_raw": 300}, "wet_raw"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			body := map[string]any{"name": "Ficus", "category": "foliage", "pot_ml": 2000}
			for k, v := range tc.body {
				body[k] = v
			}
			e := expectError(t, do(t, http.MethodPost, srv.URL+"/api/v1/pots", body),
				http.StatusBadRequest, api.ErrorCodeInvalidRequest)
			if e.Details == nil || (*e.Details)[0].Field != tc.field {
				t.Fatalf("details = %+v, want field %q", e.Details, tc.field)
			}
		})
	}
}

func TestPotPatchUnlink(t *testing.T) {
	srv, _ := newTestAPI(t)
	pot := createPot(t, srv, map[string]any{"node_id": "node-1", "channel": 3, "dry_raw": 540, "wet_raw": 270})
	got := expect[api.Pot](t, do(t, http.MethodPatch, potURL(srv, pot.Id, ""),
		`{"node_id":null,"channel":null}`), http.StatusOK)
	isNull(t, got.NodeId, "node_id")
	isNull(t, got.Channel, "channel")
	if val(t, got.DryRaw, "dry_raw") != 540 {
		t.Errorf("calibration changed: %+v", got)
	}
	got = expect[api.Pot](t, do(t, http.MethodPatch, potURL(srv, pot.Id, ""),
		`{"dry_raw":null,"wet_raw":null}`), http.StatusOK)
	isNull(t, got.DryRaw, "dry_raw")
	isNull(t, got.WetRaw, "wet_raw")
}

func TestPotCurrentAndRecalibration(t *testing.T) {
	srv, _ := newTestAPI(t)
	readingAt := testNow.Add(-time.Minute)
	postReading(t, srv, "node-1", readingAt, map[int]int{0: 405})
	pot := createPot(t, srv, map[string]any{"node_id": "node-1", "channel": 0})
	if val(t, pot.Current.MoistureRaw, "moisture_raw") != 405 {
		t.Fatalf("current = %+v", pot.Current)
	}
	if !val(t, pot.Current.LastReadingAt, "last_reading_at").Equal(readingAt) {
		t.Errorf("last_reading_at = %v", pot.Current.LastReadingAt)
	}
	isNull(t, pot.Current.MoisturePct, "moisture_pct (uncalibrated)")
	history := expect[[]api.HistoryPoint](t, do(t, http.MethodGet, potURL(srv, pot.Id, "/readings"), nil), http.StatusOK)
	if len(history) != 1 {
		t.Fatalf("history len %d", len(history))
	}
	isNull(t, history[0].MoisturePct, "history moisture_pct (uncalibrated)")

	for _, tc := range []struct{ dry, wet, want int }{{540, 270, 50}, {600, 300, 65}} {
		got := expect[api.Pot](t, do(t, http.MethodPatch, potURL(srv, pot.Id, ""),
			map[string]any{"dry_raw": tc.dry, "wet_raw": tc.wet}), http.StatusOK)
		if pct := val(t, got.Current.MoisturePct, "moisture_pct"); math.Abs(pct-float64(tc.want)) > 1e-9 {
			t.Errorf("calibration %d/%d: moisture_pct = %v, want %d", tc.dry, tc.wet, pct, tc.want)
		}
		history := expect[[]api.HistoryPoint](t, do(t, http.MethodGet, potURL(srv, pot.Id, "/readings"), nil), http.StatusOK)
		if pct := val(t, history[0].MoisturePct, "history moisture_pct"); math.Abs(pct-float64(tc.want)) > 1e-9 {
			t.Errorf("calibration %d/%d: history moisture_pct = %v, want %d", tc.dry, tc.wet, pct, tc.want)
		}
	}
}

func TestPotReadingsHistoryStep(t *testing.T) {
	srv, _ := newTestAPI(t)
	start := testNow.Add(-time.Hour)
	for i := range 12 {
		postReading(t, srv, "node-1", start.Add(time.Duration(i)*5*time.Minute), map[int]int{0: 400 + 10*i})
	}
	pot := createPot(t, srv, map[string]any{"node_id": "node-1", "channel": 0})
	window := "?from=" + ts(testNow.Add(-2*time.Hour)) + "&to=" + ts(testNow)

	raw := expect[[]api.HistoryPoint](t, do(t, http.MethodGet, potURL(srv, pot.Id, "/readings"+window), nil), http.StatusOK)
	if len(raw) != 12 {
		t.Fatalf("raw points %d, want 12", len(raw))
	}

	hourly := expect[[]api.HistoryPoint](t, do(t, http.MethodGet,
		potURL(srv, pot.Id, "/readings"+window+"&step=3600"), nil), http.StatusOK)
	if len(hourly) != 1 {
		t.Fatalf("hourly points %d, want 1: %+v", len(hourly), hourly)
	}
	if !hourly[0].Ts.Equal(start) || val(t, hourly[0].MoistureRaw, "moisture_raw") != 455 ||
		val(t, hourly[0].TempC, "temp_c") != 21.5 {
		t.Fatalf("hourly = %+v", hourly[0])
	}
	isNull(t, hourly[0].HumidityPct, "humidity_pct")

	for name, query := range map[string]string{
		"step too big":   window + "&step=86401",
		"negative step":  window + "&step=-1",
		"over 31 days":   "?from=" + ts(testNow.Add(-32*24*time.Hour)) + "&to=" + ts(testNow),
		"to before from": "?from=" + ts(testNow) + "&to=" + ts(testNow.Add(-time.Hour)),
	} {
		resp := do(t, http.MethodGet, potURL(srv, pot.Id, "/readings"+query), nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, resp.StatusCode)
		}
	}
}

func TestWateringsFeed(t *testing.T) {
	srv, _ := newTestAPI(t)
	pot := createPot(t, srv, map[string]any{"node_id": "node-1", "channel": 2})
	w := expect[api.Watering](t, do(t, http.MethodPost, potURL(srv, pot.Id, "/waterings"),
		map[string]any{"amount_ml": 200}), http.StatusCreated)
	if w.Source != "manual" || !w.Ts.Equal(testNow) || !w.CreatedAt.Equal(testNow) || w.PotId != pot.Id {
		t.Fatalf("watering = %+v", w)
	}

	feed := expect[[]api.WateringFeedItem](t, do(t, http.MethodGet,
		srv.URL+"/api/v1/waterings?since="+ts(testNow.Add(-time.Minute)), nil), http.StatusOK)
	if len(feed) != 1 || feed[0].Id != w.Id || val(t, feed[0].NodeId, "node_id") != "node-1" ||
		val(t, feed[0].Channel, "channel") != 2 || feed[0].AmountMl != 200 {
		t.Fatalf("feed = %+v", feed)
	}
	if feed = expect[[]api.WateringFeedItem](t, do(t, http.MethodGet,
		srv.URL+"/api/v1/waterings?since="+ts(testNow), nil), http.StatusOK); len(feed) != 0 {
		t.Fatalf("feed after created_at = %+v", feed)
	}

	list := expect[[]api.Watering](t, do(t, http.MethodGet, potURL(srv, pot.Id, "/waterings"), nil), http.StatusOK)
	if len(list) != 1 || list[0].Id != w.Id {
		t.Fatalf("list = %+v", list)
	}

	for name, body := range map[string]map[string]any{
		"too old":       {"amount_ml": 100, "ts": testNow.Add(-8 * 24 * time.Hour)},
		"future":        {"amount_ml": 100, "ts": testNow.Add(10 * time.Minute)},
		"zero amount":   {"amount_ml": 0},
		"amount > 20 l": {"amount_ml": 20001},
	} {
		resp := do(t, http.MethodPost, potURL(srv, pot.Id, "/waterings"), body)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, resp.StatusCode)
		}
	}
	expectError(t, do(t, http.MethodPost, potURL(srv, 999, "/waterings"),
		map[string]any{"amount_ml": 100}), http.StatusNotFound, api.ErrorCodeNotFound)
}

func TestCheckNowAndAdviceList(t *testing.T) {
	srv, _ := newTestAPI(t)
	postReading(t, srv, "node-1", testNow.Add(-time.Minute), map[int]int{0: 520})
	pot := createPot(t, srv, map[string]any{"node_id": "node-1", "channel": 0, "dry_raw": 540, "wet_raw": 270})

	adv := expect[api.Advice](t, do(t, http.MethodPost, potURL(srv, pot.Id, "/advice"), nil), http.StatusOK)
	if adv.Status != api.AdviceStatusOk || adv.Trigger != api.Manual || adv.Provider != "rules" {
		t.Fatalf("advice = %+v", adv)
	}
	if rec := val(t, adv.Recommendation, "recommendation"); !rec.NeedsWater || rec.AmountMl <= 0 {
		t.Fatalf("recommendation = %+v", rec)
	}
	if val(t, adv.Summary, "summary")["pot"] == nil {
		t.Errorf("summary = %v", adv.Summary)
	}

	list := expect[api.AdviceList](t, do(t, http.MethodGet, potURL(srv, pot.Id, "/advice"), nil), http.StatusOK)
	if latest := val(t, list.Latest, "latest"); latest.Id != adv.Id {
		t.Fatalf("latest id %d, want %d", latest.Id, adv.Id)
	}
	if len(list.History) != 1 {
		t.Fatalf("history len %d", len(list.History))
	}
	got := expect[api.Pot](t, do(t, http.MethodGet, potURL(srv, pot.Id, ""), nil), http.StatusOK)
	if val(t, got.LatestAdvice, "latest_advice").Id != adv.Id {
		t.Errorf("pot latest_advice = %+v", got.LatestAdvice)
	}
}

func TestHealth(t *testing.T) {
	srv, _ := newTestAPI(t)
	h := expect[api.Health](t, do(t, http.MethodGet, srv.URL+"/api/v1/health", nil), http.StatusOK)
	if h.Status != api.HealthStatusOk || h.AdvisorProvider != "rules" || h.AdvisorModel != "rules-v1" {
		t.Fatalf("health = %+v", h)
	}
}

func TestCheckNowUnknownPot(t *testing.T) {
	srv, _ := newTestAPI(t)
	expectError(t, do(t, http.MethodPost, potURL(srv, 999, "/advice"), nil), http.StatusNotFound, api.ErrorCodeNotFound)
}

func TestPotCurrentStatusIsLive(t *testing.T) {
	srv, st := newTestAPI(t)
	unlinked := createPot(t, srv, nil)
	if unlinked.Current.Status != api.SensorOffline {
		t.Errorf("unlinked status = %q, want sensor_offline", unlinked.Current.Status)
	}

	postReading(t, srv, "node-1", testNow.Add(-time.Minute), map[int]int{0: 405})
	fresh := createPot(t, srv, map[string]any{"node_id": "node-1", "channel": 0})
	if fresh.Current.Status != api.NeedsCalibration {
		t.Errorf("uncalibrated status = %q, want needs_calibration", fresh.Current.Status)
	}
	got := expect[api.Pot](t, do(t, http.MethodPatch, potURL(srv, fresh.Id, ""),
		map[string]any{"dry_raw": 540, "wet_raw": 270}), http.StatusOK)
	if got.Current.Status != api.Ok {
		t.Errorf("calibrated status = %q, want ok", got.Current.Status)
	}

	// dropout: the last reading is older than offline_after while the stored advice still says ok
	postReading(t, srv, "node-2", testNow.Add(-40*time.Minute), map[int]int{0: 405})
	pot := createPot(t, srv, map[string]any{"node_id": "node-2", "channel": 0, "dry_raw": 540, "wet_raw": 270})
	rec := &core.Recommendation{NeedsWater: false, Confidence: core.ConfidenceHigh, Reason: "moist"}
	if _, err := st.InsertAdvice(context.Background(), core.Advice{
		PotID: pot.Id, CreatedAt: testNow.Add(-time.Hour), Trigger: core.TriggerScheduled,
		Status: core.StatusOK, Recommendation: rec, Rules: rec, Summary: []byte(`{}`),
	}); err != nil {
		t.Fatal(err)
	}
	got = expect[api.Pot](t, do(t, http.MethodGet, potURL(srv, pot.Id, ""), nil), http.StatusOK)
	if got.Current.Status != api.SensorOffline {
		t.Errorf("dropout status = %q, want sensor_offline", got.Current.Status)
	}
	if latest := val(t, got.LatestAdvice, "latest_advice"); latest.Status != api.AdviceStatusOk {
		t.Errorf("latest_advice.status = %q, want ok", latest.Status)
	}
}
