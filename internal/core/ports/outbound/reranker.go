package outbound

import (
	"context"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
)

// Reranker abstracts document reranking operations with an AI model provider.
// The domain request carries the model name so a single adapter can serve any
// rerank model the provider routes (e.g. Cohere rerank-v3.5 via OpenRouter).
type Reranker interface {
	Rerank(ctx context.Context, req *domain.RerankRequest) (*domain.RerankResponse, error)
}
