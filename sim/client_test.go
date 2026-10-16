package sim

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWateringsSinceDecodesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`[{"amount_ml":400,"created_at":"2026-01-01T00:00:00Z","node_id":"n","channel":0}]`))
	}))
	defer srv.Close()

	events, err := NewHTTPBackend(srv.URL, nil).WateringsSince(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].AmountML != 400 {
		t.Fatalf("events = %+v, want one 400 ml event", events)
	}
}
