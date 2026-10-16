// Package openaicompat implements core.Advisor over an OpenAI-compatible chat completions API.
package openaicompat

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/vlastrog/greenh/core"
)

const promptVersion = "v1"

//go:embed prompt_v1.txt
var promptV1 string

type Config struct {
	BaseURL, Model, APIKey string
	HTTPClient             *http.Client
}

type Client struct {
	cfg Config
}

func New(cfg Config) *Client {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{}
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &Client{cfg: cfg}
}

func (c *Client) Info() core.ProviderInfo {
	return core.ProviderInfo{Provider: "openaicompat", Model: c.cfg.Model, PromptVersion: promptVersion}
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string    `json:"model"`
	Temperature float64   `json:"temperature"`
	Messages    []message `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

// Advise sends only the summary; req.Rules stays local.
func (c *Client) Advise(ctx context.Context, req core.AdviceRequest) (core.Recommendation, error) {
	summary, err := json.Marshal(req.Summary)
	if err != nil {
		return core.Recommendation{}, fmt.Errorf("marshal summary: %w", err)
	}
	body, err := json.Marshal(chatRequest{
		Model:       c.cfg.Model,
		Temperature: 0.2,
		Messages: []message{
			{Role: "system", Content: promptV1},
			{Role: "user", Content: string(summary)},
		},
	})
	if err != nil {
		return core.Recommendation{}, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return core.Recommendation{}, errors.New("build request")
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.cfg.HTTPClient.Do(httpReq)
	if err != nil {
		// url errors carry no headers, so the key can't leak here
		return core.Recommendation{}, fmt.Errorf("advisor request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return core.Recommendation{}, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return core.Recommendation{}, fmt.Errorf("advisor returned status %d", resp.StatusCode)
	}

	var cr chatResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return core.Recommendation{}, fmt.Errorf("decode response: %w", err)
	}
	if len(cr.Choices) == 0 {
		return core.Recommendation{}, errors.New("advisor returned no choices")
	}
	obj, ok := extractJSONObject(cr.Choices[0].Message.Content)
	if !ok {
		return core.Recommendation{}, errors.New("no JSON object in advisor answer")
	}
	var rec core.Recommendation
	if err := json.Unmarshal([]byte(obj), &rec); err != nil {
		return core.Recommendation{}, fmt.Errorf("decode recommendation: %w", err)
	}
	return rec, nil
}

// extractJSONObject takes the span from the first '{' to the last '}', which also strips code fences.
func extractJSONObject(s string) (string, bool) {
	start, end := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if start < 0 || end < start {
		return "", false
	}
	return s[start : end+1], true
}
