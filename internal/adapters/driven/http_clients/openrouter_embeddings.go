package http_clients

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
	"github.com/lin-br/go-linai-tools/internal/core/ports/outbound"
)

const openRouterEmbeddingsURL = "https://openrouter.ai/api/v1/embeddings"

// Compile-time interface check.
var _ outbound.Embedder = (*OpenRouterEmbeddingsProvider)(nil)

// OpenRouterEmbeddingsProvider implements outbound.Embedder for the OpenRouter
// embeddings endpoint (POST /api/v1/embeddings). It uses the same API key and
// headers as OpenRouterProvider — the model is supplied per-request via
// domain.EmbeddingRequest.Model so a single adapter can serve any embedding
// model OpenRouter routes (Voyage, OpenAI, Cohere, etc.).
type OpenRouterEmbeddingsProvider struct {
	apiKey  string
	client  *http.Client
	baseURL string
}

// NewOpenRouterEmbeddingsProvider creates an embedder backed by OpenRouter.
func NewOpenRouterEmbeddingsProvider(apiKey string) *OpenRouterEmbeddingsProvider {
	return &OpenRouterEmbeddingsProvider{
		apiKey:  apiKey,
		client:  http.DefaultClient,
		baseURL: openRouterEmbeddingsURL,
	}
}

// Embed sends an embedding request to OpenRouter and returns the vectors.
func (o *OpenRouterEmbeddingsProvider) Embed(ctx context.Context, req *domain.EmbeddingRequest) (*domain.EmbeddingResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	wire := toEmbeddingsWire(req)
	body, err := json.Marshal(wire)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	setOpenRouterHeaders(httpReq, o.apiKey)

	resp, err := o.client.Do(httpReq)
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

	var wireResp embeddingsResponse
	if err := json.Unmarshal(respBody, &wireResp); err != nil {
		return nil, err
	}

	return fromEmbeddingsWire(&wireResp), nil
}

// embeddingsRequest is the wire body for POST /api/v1/embeddings.
type embeddingsRequest struct {
	Model          string   `json:"model"`
	Input         []string `json:"input"`
	Dimensions     int      `json:"dimensions,omitempty"`
	EncodingFormat string   `json:"encoding_format,omitempty"`
	InputType      string   `json:"input_type,omitempty"`
}

// embeddingsData wraps a single vector in the response.
type embeddingsData struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
	Object    string    `json:"object"`
}

// embeddingsUsage reports token usage for an embeddings call.
type embeddingsUsage struct {
	PromptTokens int     `json:"prompt_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	Cost         float64 `json:"cost,omitempty"`
}

// embeddingsResponse is the wire body returned by POST /api/v1/embeddings.
type embeddingsResponse struct {
	Data   []embeddingsData `json:"data"`
	Model string           `json:"model"`
	Object string           `json:"object"`
	Usage  embeddingsUsage `json:"usage"`
}

func toEmbeddingsWire(req *domain.EmbeddingRequest) *embeddingsRequest {
	return &embeddingsRequest{
		Model:          req.Model,
		Input:          req.Input,
		Dimensions:     req.Dimensions,
		EncodingFormat: req.EncodingFormat,
		InputType:      req.InputType,
	}
}

func fromEmbeddingsWire(resp *embeddingsResponse) *domain.EmbeddingResponse {
	out := &domain.EmbeddingResponse{
		Model: resp.Model,
		Usage: domain.EmbeddingUsage{
			PromptTokens: resp.Usage.PromptTokens,
			TotalTokens:  resp.Usage.TotalTokens,
			Cost:         resp.Usage.Cost,
		},
	}
	if resp.Data != nil {
		out.Data = make([]domain.EmbeddingData, len(resp.Data))
		for i, d := range resp.Data {
			out.Data[i] = domain.EmbeddingData{
				Embedding: d.Embedding,
				Index:     d.Index,
				Object:    d.Object,
			}
		}
	}
	return out
}
