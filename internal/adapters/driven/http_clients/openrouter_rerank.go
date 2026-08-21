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

const openRouterRerankURL = "https://openrouter.ai/api/v1/rerank"

// Compile-time interface check.
var _ outbound.Reranker = (*OpenRouterRerankProvider)(nil)

// OpenRouterRerankProvider implements outbound.Reranker for the OpenRouter
// rerank endpoint (POST /api/v1/rerank). It uses the same API key and headers
// as OpenRouterProvider — the model is supplied per-request via
// domain.RerankRequest.Model so a single adapter can serve any rerank model
// OpenRouter routes (Cohere, Voyage, etc.).
type OpenRouterRerankProvider struct {
	apiKey  string
	client  *http.Client
	baseURL string
}

// NewOpenRouterRerankProvider creates a reranker backed by OpenRouter.
func NewOpenRouterRerankProvider(apiKey string) *OpenRouterRerankProvider {
	return &OpenRouterRerankProvider{
		apiKey:  apiKey,
		client:  http.DefaultClient,
		baseURL: openRouterRerankURL,
	}
}

// Rerank sends a rerank request to OpenRouter and returns the ranked results.
func (o *OpenRouterRerankProvider) Rerank(ctx context.Context, req *domain.RerankRequest) (*domain.RerankResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	wire := toRerankWire(req)
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

	var wireResp rerankResponse
	if err := json.Unmarshal(respBody, &wireResp); err != nil {
		return nil, err
	}

	return fromRerankWire(&wireResp), nil
}

// rerankRequest is the wire body for POST /api/v1/rerank. Documents accept
// either plain strings or structured {text, image} objects; we use the
// structured form so multimodal models can carry images.
type rerankRequest struct {
	Model     string             `json:"model"`
	Query     string             `json:"query"`
	Documents []rerankDocument   `json:"documents"`
	TopN      int                `json:"top_n,omitempty"`
}

type rerankDocument struct {
	Text  string `json:"text,omitempty"`
	Image string `json:"image,omitempty"`
}

type rerankResult struct {
	Document       rerankDocument `json:"document"`
	Index          int            `json:"index"`
	RelevanceScore float64        `json:"relevance_score"`
}

type rerankUsage struct {
	SearchUnits int     `json:"search_units,omitempty"`
	TotalTokens int     `json:"total_tokens,omitempty"`
	Cost        float64 `json:"cost,omitempty"`
}

type rerankResponse struct {
	ID       string         `json:"id"`
	Model    string         `json:"model"`
	Provider string         `json:"provider,omitempty"`
	Results  []rerankResult `json:"results"`
	Usage    rerankUsage    `json:"usage,omitempty"`
}

func toRerankWire(req *domain.RerankRequest) *rerankRequest {
	docs := make([]rerankDocument, len(req.Documents))
	for i, d := range req.Documents {
		docs[i] = rerankDocument{Text: d.Text, Image: d.Image}
	}
	return &rerankRequest{
		Model:     req.Model,
		Query:     req.Query,
		Documents: docs,
		TopN:      req.TopN,
	}
}

func fromRerankWire(resp *rerankResponse) *domain.RerankResponse {
	out := &domain.RerankResponse{
		ID:    resp.ID,
		Model: resp.Model,
		Usage: domain.RerankUsage{
			SearchUnits: resp.Usage.SearchUnits,
			TotalTokens: resp.Usage.TotalTokens,
			Cost:        resp.Usage.Cost,
		},
	}
	if resp.Results != nil {
		out.Results = make([]domain.RerankResult, len(resp.Results))
		for i, r := range resp.Results {
			out.Results[i] = domain.RerankResult{
				Document: domain.RerankDocument{
					Text:  r.Document.Text,
					Image: r.Document.Image,
				},
				Index:          r.Index,
				RelevanceScore: r.RelevanceScore,
			}
		}
	}
	return out
}
