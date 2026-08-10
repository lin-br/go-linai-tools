package usecases

import (
	"context"
	"errors"

	"github.com/lin-br/go-linai-tools/internal/configs"
	"github.com/lin-br/go-linai-tools/internal/core/domain"
	"github.com/lin-br/go-linai-tools/internal/core/ports/outbound"
)

type DoSendMessageUseCase struct {
	config   configs.Config
	provider outbound.Provider
}

func NewSendMessageUseCase(config configs.Config, provider outbound.Provider) *DoSendMessageUseCase {
	return &DoSendMessageUseCase{
		config:   config,
		provider: provider,
	}
}

func (uc *DoSendMessageUseCase) Send(ctx context.Context, message string) (*domain.ChatResponse, error) {
	model, err := uc.parseModel()
	if err != nil {
		return nil, err
	}

	req := &domain.ChatRequest{
		Model: *model,
		Messages: []domain.Message{
			{Role: domain.MessageRoleUser, Content: message},
		},
	}

	return uc.provider.Chat(ctx, req)
}

func (uc *DoSendMessageUseCase) parseModel() (*string, error) {
	model := uc.config.Models.Get()
	if model != nil {
		return model, nil
	}
	return nil, errors.New("the AI model is empty")
}
