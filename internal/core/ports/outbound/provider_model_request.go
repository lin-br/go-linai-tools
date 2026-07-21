package outbound

import "github.com/lin-br/go-linai-tools/internal/core/domain"

type ProviderModelHandler interface {
	DoMessagesRequest(request *domain.Request) (*domain.Response, error)
}
