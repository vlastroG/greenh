package sim

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type AmbientPayload struct {
	TempC       *float64 `json:"temp_c,omitempty"`
	HumidityPct *float64 `json:"humidity_pct,omitempty"`
	LightLux    *float64 `json:"light_lux,omitempty"`
}

type ChannelPayload struct {
	Ch          int `json:"ch"`
	MoistureRaw int `json:"moisture_raw"`
}

type ReadingPayload struct {
	NodeID   string           `json:"node_id"`
	TS       time.Time        `json:"ts"`
	Ambient  AmbientPayload   `json:"ambient"`
	Channels []ChannelPayload `json:"channels"`
}

type WateringEvent struct {
	ID        int64     `json:"id"`
	NodeID    *string   `json:"node_id"`
	Channel   *int      `json:"channel"`
	AmountML  int       `json:"amount_ml"`
	CreatedAt time.Time `json:"created_at"`
}

// HTTPError is returned for non-2xx responses.
type HTTPError struct {
	Status int
	Body   string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("backend returned %d: %s", e.Status, e.Body) }

type Backend interface {
	PostReading(ctx context.Context, r ReadingPayload) error
	WateringsSince(ctx context.Context, since time.Time) ([]WateringEvent, error)
}

type httpBackend struct {
	base string
	hc   *http.Client
}

func NewHTTPBackend(baseURL string, hc *http.Client) Backend {
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	return &httpBackend{base: strings.TrimRight(baseURL, "/"), hc: hc}
}

func (b *httpBackend) PostReading(ctx context.Context, r ReadingPayload) error {
	body, err := json.Marshal(r)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.base+"/api/v1/readings", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	err = statusError(resp)
	io.Copy(io.Discard, resp.Body)
	return err
}

func (b *httpBackend) WateringsSince(ctx context.Context, since time.Time) ([]WateringEvent, error) {
	q := url.Values{"since": {since.UTC().Format(time.RFC3339)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.base+"/api/v1/waterings?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := b.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := statusError(resp); err != nil {
		return nil, err
	}
	var events []WateringEvent
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return nil, fmt.Errorf("decode waterings: %w", err)
	}
	return events, nil
}

func statusError(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return &HTTPError{Status: resp.StatusCode, Body: strings.TrimSpace(string(body))}
}
