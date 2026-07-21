package usecases

import (
	"errors"

	"github.com/lin-br/go-linai-tools/internal/configs"
	"github.com/lin-br/go-linai-tools/internal/core/domain"
	"github.com/lin-br/go-linai-tools/internal/core/ports/outbound"
)

type DoSendMessageUseCase struct {
	config   configs.Config
	provider outbound.ProviderModelHandler
}

func NewSendMessageUseCase(config configs.Config, provider outbound.ProviderModelHandler) *DoSendMessageUseCase {
	return &DoSendMessageUseCase{
		config:   config,
		provider: provider,
	}
}

func (uc *DoSendMessageUseCase) Send(message string) (*domain.Response, error) {
	model, err := uc.parseModel()
	if err != nil {
		return nil, err
	}

	request := &domain.Request{
		Model:   *model,
		Message: message,
	}
	response, err := uc.provider.DoMessagesRequest(request)
	return response, err
}

func (uc *DoSendMessageUseCase) parseModel() (*string, error) {
	model := uc.config.Models.Get()
	if model != nil {
		return model, nil
	}
	return nil, errors.New("the AI model is empty")
}
