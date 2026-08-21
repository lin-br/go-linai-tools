package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"

	"github.com/lin-br/go-linai-tools/internal/core/ports/outbound"
)

// DefaultModel is the Voyage embedding model used when none is configured.
const DefaultModel = "voyage-3-large"

const voyageBaseURL = "https://api.voyageai.com/v1/embeddings"

// Client calls the Voyage AI embeddings endpoint over net/http. It does not
// implement outbound.Provider; embeddings have their own request/response
// shape and base URL (see design D2).
type Client struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

// NewClient returns a Voyage embeddings client. The API key must be non-empty.
// When httpClient is nil, http.DefaultClient is used so callers can inject a
// test double via the explicit *http.Client argument.
func NewClient(apiKey string, httpClient *http.Client) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("voyage api key is required")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		apiKey:  apiKey,
		model:   DefaultModel,
		baseURL: voyageBaseURL,
		http:    httpClient,
	}, nil
}

// WithModel overrides the embedding model used for subsequent Embed calls. It
// returns the receiver for chaining and is a no-op when model is empty.
func (c *Client) WithModel(model string) *Client {
	if model != "" {
		c.model = model
	}
	return c
}

// Embed sends inputs to Voyage and returns L2-normalized float32 vectors. The
// outer slice length matches the input batch size and preserves input order.
// Context cancellation aborts the in-flight HTTP request.
func (c *Client) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	req := voyageRequest{
		Input:     inputs,
		Model:     c.model,
		InputType: "document",
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

	var wire voyageResponse
	if err := json.Unmarshal(respBody, &wire); err != nil {
		return nil, err
	}

	out := make([][]float32, 0, len(wire.Data))
	for _, e := range wire.Data {
		out = append(out, normalize(e.Embedding))
	}
	return out, nil
}

// normalize returns an L2-normalized copy of v. A zero vector is returned as a
// copy of the input so callers cannot mutate the decoded slice.
func normalize(v []float32) []float32 {
	out := make([]float32, len(v))
	copy(out, v)
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return out
	}
	inv := float32(1.0 / math.Sqrt(sum))
	for i, x := range out {
		out[i] = x * inv
	}
	return out
}

// l2Norm returns the L2 norm of v, used by tests to assert normalization.
func l2Norm(v []float32) float64 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	return math.Sqrt(sum)
}
