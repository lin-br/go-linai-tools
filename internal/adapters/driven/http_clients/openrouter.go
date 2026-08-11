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

const openRouterBaseURL = "https://openrouter.ai/api/v1/chat/completions"

// OpenRouterProvider implements the outbound.Provider interface for OpenRouter.
type OpenRouterProvider struct {
	apiKey string
	client *http.Client
}

// NewOpenRouterProvider creates a provider backed by OpenRouter.
func NewOpenRouterProvider(apiKey string) *OpenRouterProvider {
	return &OpenRouterProvider{
		apiKey: apiKey,
		client: http.DefaultClient,
	}
}

// Chat sends a non-streaming chat completion request to OpenRouter.
func (o *OpenRouterProvider) Chat(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	payload := o.toWire(req)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterBaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	o.setHeaders(httpReq)

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

	var wireResp ChatCompletionResponse
	if err := json.Unmarshal(respBody, &wireResp); err != nil {
		return nil, err
	}

	return o.fromWire(&wireResp), nil
}

// ChatStream opens a streaming chat completion request to OpenRouter and returns
// a channel that emits domain.StreamEvent values as SSE chunks arrive.
func (o *OpenRouterProvider) ChatStream(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamEvent, error) {
	req.Stream = true
	payload := o.toWire(req)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterBaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	o.setHeaders(httpReq)

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, &outbound.ProviderError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	ch := make(chan domain.StreamEvent)
	go streamLoop(ctx, resp.Body, ch)
	return ch, nil
}

func (o *OpenRouterProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("HTTP-Referer", "lin.com.br")
	req.Header.Set("X-OpenRouter-Title", "lin.com.br")
}

func (o *OpenRouterProvider) toWire(req *domain.ChatRequest) *ChatCompletionRequest {
	wire := &ChatCompletionRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
	}

	messages := make([]WireMessage, 0, len(req.Messages)+1)

	if req.System != "" {
		messages = append(messages, WireMessage{Role: domain.MessageRoleSystem, Content: req.System})
	}

	for _, m := range req.Messages {
		wm := WireMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		if len(m.ToolCalls) > 0 {
			wm.ToolCalls = make([]WireToolCall, len(m.ToolCalls))
			for i, tc := range m.ToolCalls {
				wm.ToolCalls[i] = WireToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: WireFuncCall{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				}
			}
		}
		messages = append(messages, wm)
	}

	wire.Messages = messages

	if len(req.Tools) > 0 {
		wire.Tools = make([]WireTool, len(req.Tools))
		for i, t := range req.Tools {
			wire.Tools[i] = WireTool{
				Type: "function",
				Function: WireFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.InputSchema,
				},
			}
		}
	}

	if req.ToolChoice != nil {
		wire.ToolChoice = &WireToolChoice{Type: req.ToolChoice.Type}
		if req.ToolChoice.Type == domain.ToolChoiceTool {
			wire.ToolChoice.Function = WireFuncChoice{Name: req.ToolChoice.Name}
		}
	}

	return wire
}

func (o *OpenRouterProvider) fromWire(resp *ChatCompletionResponse) *domain.ChatResponse {
	out := &domain.ChatResponse{Model: resp.Model}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		out.Content = choice.Message.Content
		out.StopReason = choice.FinishReason

		if len(choice.Message.ToolCalls) > 0 {
			out.ToolCalls = make([]domain.ToolCall, len(choice.Message.ToolCalls))
			for i, tc := range choice.Message.ToolCalls {
				out.ToolCalls[i] = domain.ToolCall{
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				}
			}
		}
	}

	if resp.Usage != nil {
		out.Usage = domain.Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
			Cost:         resp.Usage.Cost,
		}
	}

	return out
}
