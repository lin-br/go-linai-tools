package outbound

import (
	"context"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
)

// Embedder abstracts text embedding operations with an AI model provider.
// It mirrors the Provider interface for chat: the domain request carries the
// model name so a single adapter can serve any embedding model the provider
// routes (e.g. Voyage or Cohere via OpenRouter).
type Embedder interface {
	Embed(ctx context.Context, req *domain.EmbeddingRequest) (*domain.EmbeddingResponse, error)
}
