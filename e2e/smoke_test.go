//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

type recommendation struct {
	NeedsWater bool   `json:"needs_water"`
	AmountML   int    `json:"amount_ml"`
	Reason     string `json:"reason"`
}

type advice struct {
	Status         string          `json:"status"`
	Recommendation *recommendation `json:"recommendation"`
}

type pot struct {
	ID      int64 `json:"id"`
	Current struct {
		MoisturePct *float64 `json:"moisture_pct"`
	} `json:"current"`
}

type apiClient struct {
	t    *testing.T
	base string
	http *http.Client
}

// do sends the request and decodes a 2xx JSON response into out (if non-nil).
func (c *apiClient) do(method, path string, body, out any) {
	c.t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+"/api/v1"+path, rd)
	if err != nil {
		c.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		c.t.Fatalf("%s %s: status %d: %s", method, path, resp.StatusCode, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			c.t.Fatalf("%s %s: decode %s: %v", method, path, raw, err)
		}
	}
}

func (c *apiClient) waitForNode(id string, channels int, within time.Duration) {
	c.t.Helper()
	deadline := time.Now().Add(within)
	for {
		var nodes []struct {
			ID       string `json:"id"`
			Channels []any  `json:"channels"`
		}
		resp, err := c.http.Get(c.base + "/api/v1/nodes")
		if err == nil {
			_ = json.NewDecoder(resp.Body).Decode(&nodes)
			resp.Body.Close()
			for _, n := range nodes {
				if n.ID == id && len(n.Channels) == channels {
					return
				}
			}
		}
		if time.Now().After(deadline) {
			c.t.Fatalf("node %s with %d channels not seen within %s", id, channels, within)
		}
		time.Sleep(2 * time.Second)
	}
}

func TestSmoke(t *testing.T) {
	base := os.Getenv("WATERER_URL")
	if base == "" {
		base = "http://localhost:8080"
	}
	// advice may take an LLM attempt plus a retry
	c := &apiClient{t: t, base: base, http: &http.Client{Timeout: 90 * time.Second}}

	c.waitForNode("windowsill-1", 3, 60*time.Second)

	var p pot
	c.do("POST", "/pots", map[string]any{
		"name": "e2e fern", "category": "foliage", "pot_ml": 2000,
		"node_id": "windowsill-1", "channel": 0, "dry_raw": 540, "wet_raw": 270,
	}, &p)
	t.Cleanup(func() {
		req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/pots/%d", base, p.ID), nil)
		if resp, err := c.http.Do(req); err == nil {
			resp.Body.Close()
		}
	})
	potPath := fmt.Sprintf("/pots/%d", p.ID)

	from := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	var history []json.RawMessage
	c.do("GET", potPath+"/readings?from="+from, nil, &history)
	if len(history) < 100 {
		t.Fatalf("history has %d points, want >= 100 (backfill)", len(history))
	}

	var first advice
	c.do("POST", potPath+"/advice", nil, &first)
	if first.Status != "ok" {
		t.Fatalf("first advice status = %q, want ok", first.Status)
	}

	c.do("GET", potPath, nil, &p)
	if p.Current.MoisturePct == nil {
		t.Fatal("current moisture_pct is null")
	}
	before := *p.Current.MoisturePct

	c.do("POST", potPath+"/waterings", map[string]any{"amount_ml": 400}, nil)

	deadline := time.Now().Add(120 * time.Second)
	for {
		c.do("GET", potPath, nil, &p)
		if p.Current.MoisturePct != nil && *p.Current.MoisturePct >= before+10 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("moisture did not rise by 10 points from %.1f within 120s", before)
		}
		time.Sleep(5 * time.Second)
	}

	var second advice
	c.do("POST", potPath+"/advice", nil, &second)
	if second.Recommendation == nil {
		t.Fatalf("second advice has no recommendation (status %q)", second.Status)
	}
	if second.Recommendation.NeedsWater && first.Recommendation != nil &&
		*second.Recommendation == *first.Recommendation {
		t.Fatalf("advice unchanged after watering: %+v", second.Recommendation)
	}
}
