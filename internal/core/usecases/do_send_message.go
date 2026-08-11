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
	req, err := uc.buildRequest(message)
	if err != nil {
		return nil, err
	}

	return uc.provider.Chat(ctx, req)
}

func (uc *DoSendMessageUseCase) Stream(ctx context.Context, message string) (<-chan domain.StreamEvent, error) {
	req, err := uc.buildRequest(message)
	if err != nil {
		return nil, err
	}

	return uc.provider.ChatStream(ctx, req)
}

func (uc *DoSendMessageUseCase) buildRequest(message string) (*domain.ChatRequest, error) {
	model, err := uc.parseModel()
	if err != nil {
		return nil, err
	}

	return &domain.ChatRequest{
		Model: *model,
		Messages: []domain.Message{
			{Role: domain.MessageRoleUser, Content: message},
		},
	}, nil
}

func (uc *DoSendMessageUseCase) parseModel() (*string, error) {
	model := uc.config.Models.Get()
	if model != nil {
		return model, nil
	}
	return nil, errors.New("the AI model is empty")
}
