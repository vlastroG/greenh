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

func TestNodesDecodesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/nodes" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`[{"id":"n1","name":"","last_seen":"2026-01-01T00:00:00Z","channels":[]}]`))
	}))
	defer srv.Close()

	nodes, err := NewHTTPBackend(srv.URL, nil).Nodes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if len(nodes) != 1 || nodes[0].ID != "n1" || !nodes[0].LastSeen.Equal(want) {
		t.Fatalf("nodes = %+v", nodes)
	}
}
