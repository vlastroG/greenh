package openaicompat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vlastrog/greenh/core"
)

const testKey = "sk-secret-key"

const answer = `{"needs_water": true, "amount_ml": 150, "confidence": "high", "reason": "dry"}`

func chatBody(content string) string {
	b, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": content}}},
	})
	return string(b)
}

func serve(t *testing.T, content string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, chatBody(content))
	}))
	t.Cleanup(srv.Close)
	return New(Config{BaseURL: srv.URL, Model: "m", APIKey: testKey})
}

func request() core.AdviceRequest {
	return core.AdviceRequest{
		Summary: core.Summary{Pot: core.SummaryPot{Species: "ficus", Category: "tropical", PotML: 2000}},
		Rules:   core.Rules{},
	}
}

func wantRec(t *testing.T, r core.Recommendation) {
	t.Helper()
	want := core.Recommendation{NeedsWater: true, AmountML: 150, Confidence: "high", Reason: "dry"}
	if r != want {
		t.Fatalf("got %+v, want %+v", r, want)
	}
}

func TestInfo(t *testing.T) {
	got := New(Config{Model: "deepseek-chat"}).Info()
	want := core.ProviderInfo{Provider: "openaicompat", Model: "deepseek-chat", PromptVersion: "v1"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestAdviseSendsSummaryOnly(t *testing.T) {
	var path, auth string
	var raw []byte
	var body struct {
		Model       string  `json:"model"`
		Temperature float64 `json:"temperature"`
		Messages    []struct{ Role, Content string }
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, auth = r.URL.Path, r.Header.Get("Authorization")
		raw, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, chatBody(answer))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL + "/v1/", Model: "m", APIKey: testKey})
	req := request()
	if _, err := c.Advise(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if path != "/v1/chat/completions" {
		t.Errorf("path = %q", path)
	}
	if auth != "Bearer "+testKey {
		t.Errorf("auth = %q", auth)
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body.Model != "m" || body.Temperature != 0.2 || len(body.Messages) != 2 {
		t.Fatalf("body = %+v", body)
	}
	wantUser, _ := json.Marshal(req.Summary)
	if body.Messages[0].Role != "system" || !strings.HasPrefix(body.Messages[0].Content, strings.TrimSpace(promptV1)[:40]) {
		t.Errorf("system message = %+v", body.Messages[0])
	}
	if body.Messages[1].Role != "user" || body.Messages[1].Content != string(wantUser) {
		t.Errorf("user message = %q", body.Messages[1].Content)
	}
	if strings.Contains(string(raw), "threshold") {
		t.Error("rules leaked into request body")
	}
}

func TestAdviseParsesPlainJSON(t *testing.T) {
	r, err := serve(t, answer).Advise(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	wantRec(t, r)
}

func TestAdviseParsesFencedJSON(t *testing.T) {
	r, err := serve(t, "```json\n"+answer+"\n```").Advise(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	wantRec(t, r)
}

func TestAdviseParsesJSONWithProse(t *testing.T) {
	r, err := serve(t, "Here is my answer: "+answer+" Hope it helps.").Advise(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	wantRec(t, r)
}

func TestAdviseBadJSON(t *testing.T) {
	if _, err := serve(t, "no idea").Advise(context.Background(), request()); err == nil {
		t.Fatal("expected error")
	}
}

func TestAdviseEmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"choices":[]}`)
	}))
	defer srv.Close()
	c := New(Config{BaseURL: srv.URL, Model: "m", APIKey: testKey})
	if _, err := c.Advise(context.Background(), request()); err == nil {
		t.Fatal("expected error")
	}
}

func TestAdviseHTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := New(Config{BaseURL: srv.URL, Model: "m", APIKey: testKey})
	_, err := c.Advise(context.Background(), request())
	if err == nil || !strings.Contains(err.Error(), "500") || strings.Contains(err.Error(), testKey) {
		t.Fatalf("err = %v", err)
	}
}

func TestAdviseContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = io.WriteString(w, chatBody(answer))
	}))
	defer srv.Close()
	c := New(Config{BaseURL: srv.URL, Model: "m", APIKey: testKey})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := c.Advise(ctx, request())
	if err == nil || strings.Contains(err.Error(), testKey) {
		t.Fatalf("err = %v", err)
	}
}
