package rerank

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/google/uuid"
	"github.com/lin-br/go-linai-tools/internal/core/ports/outbound"
)

// DefaultModel is the Cohere rerank model used when none is configured.
const DefaultModel = "rerank-v3.5"

const cohereBaseURL = "https://api.cohere.com/v2/rerank"

// Client calls the Cohere Rerank API over net/http. It does not implement
// outbound.Provider (see design D2).
type Client struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

// NewClient returns a Cohere rerank client. The API key must be non-empty.
// When httpClient is nil, http.DefaultClient is used.
func NewClient(apiKey string, httpClient *http.Client) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("cohere api key is required")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		apiKey:  apiKey,
		model:   DefaultModel,
		baseURL: cohereBaseURL,
		http:    httpClient,
	}, nil
}

// WithModel overrides the rerank model. No-op when model is empty.
func (c *Client) WithModel(model string) *Client {
	if model != "" {
		c.model = model
	}
	return c
}

// Candidate is a document to be reranked, carrying its stable ID and text.
type Candidate struct {
	ID   uuid.UUID `json:"id"`
	Text string    `json:"text"`
}

// RankedResult is a reranked candidate with its original index and score.
type RankedResult struct {
	ID    uuid.UUID `json:"id"`
	Text  string    `json:"text"`
	Index int       `json:"index"`
	Score float64   `json:"score"`
}

// Rerank sends candidates to Cohere and returns at most topN results ordered by
// descending relevance. Empty candidates short-circuit without an API call.
func (c *Client) Rerank(ctx context.Context, query string, candidates []Candidate, topN int) ([]RankedResult, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	if topN <= 0 || topN > len(candidates) {
		topN = len(candidates)
	}

	docs := make([]string, len(candidates))
	for i, cand := range candidates {
		docs[i] = cand.Text
	}
	req := rerankRequest{
		Query:     query,
		Model:     c.model,
		Documents: docs,
		TopN:      topN,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &outbound.ProviderError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	var wire rerankResponse
	if err := json.Unmarshal(respBody, &wire); err != nil {
		return nil, err
	}

	results := make([]rerankResult, len(wire.Results))
	copy(results, wire.Results)
	sort.Slice(results, func(i, j int) bool {
		return results[i].RelevanceScore > results[j].RelevanceScore
	})

	out := make([]RankedResult, 0, len(results))
	for _, r := range results {
		if r.Index < 0 || r.Index >= len(candidates) {
			return nil, fmt.Errorf("rerank: invalid candidate index %d (have %d)", r.Index, len(candidates))
		}
		cand := candidates[r.Index]
		out = append(out, RankedResult{
			ID:    cand.ID,
			Text:  cand.Text,
			Index: r.Index,
			Score: r.RelevanceScore,
		})
	}
	if len(out) > topN {
		out = out[:topN]
	}
	return out, nil
}
