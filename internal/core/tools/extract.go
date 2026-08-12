package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
	"github.com/lin-br/go-linai-tools/internal/core/ports/outbound"
)

// BuildToolRequest constructs a *domain.ChatRequest that forces the model to
// call the tool named in schema.Name. The request carries a single user
// message containing input, a single tool definition built from schema, and a
// ToolChoice of {Type: "tool", Name: schema.Name}.
func BuildToolRequest(model, system, input string, schema ToolSchema) *domain.ChatRequest {
	return &domain.ChatRequest{
		Model:  model,
		System: system,
		Messages: []domain.Message{
			{Role: domain.MessageRoleUser, Content: input},
		},
		Tools: []domain.Tool{
			{
				Name:        schema.Name,
				Description: schema.Description,
				InputSchema: schema.InputSchema,
			},
		},
		ToolChoice: &domain.ToolChoice{
			Type: domain.ToolChoiceTool,
			Name: schema.Name,
		},
	}
}

// Extract calls the provider with a forced tool choice and decodes the matching
// tool call's arguments into a new T. It builds the request via
// BuildToolRequest, invokes p.Chat, locates the first ToolCall whose Name
// equals schema.Name, and json.Unmarshal's its Arguments into T.
//
// Errors:
//   - Provider errors from p.Chat are propagated directly.
//   - ErrNoToolCall: the response has no tool calls.
//   - ErrToolNameMismatch: tool calls exist but none matches schema.Name.
//   - ErrUnmarshalFailed: the matching arguments failed to decode into T
//     (wrapped so errors.Is matches the sentinel while preserving detail).
//
// The ctx is passed to p.Chat unmodified.
func Extract[T any](ctx context.Context, p outbound.Provider, model, system, input string, schema ToolSchema) (*T, error) {
	req := BuildToolRequest(model, system, input, schema)
	resp, err := p.Chat(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(resp.ToolCalls) == 0 {
		return nil, ErrNoToolCall
	}

	var match *domain.ToolCall
	for i := range resp.ToolCalls {
		if resp.ToolCalls[i].Name == schema.Name {
			match = &resp.ToolCalls[i]
			break
		}
	}
	if match == nil {
		return nil, ErrToolNameMismatch
	}

	var target T
	if err := json.Unmarshal([]byte(match.Arguments), &target); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnmarshalFailed, err)
	}
	return &target, nil
}
