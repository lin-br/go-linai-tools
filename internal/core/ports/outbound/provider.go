package outbound

import (
	"context"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
)

// Provider abstracts interactions with an AI model provider.
type Provider interface {
	Chat(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error)
	ChatStream(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamEvent, error)
}
