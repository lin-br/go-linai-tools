package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
)

type Person struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type fakeProvider struct {
	chatResp *domain.ChatResponse
	chatErr  error
	gotCtx   context.Context
	gotReq   *domain.ChatRequest
}

func (f *fakeProvider) Chat(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	f.gotCtx = ctx
	f.gotReq = req
	if f.chatErr != nil {
		return nil, f.chatErr
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return f.chatResp, nil
}

func (f *fakeProvider) ChatStream(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamEvent, error) {
	return nil, errors.New("ChatStream not implemented")
}

func personSchema() ToolSchema {
	return ToolSchema{
		Name:        "extract_person",
		Description: "Extract a person",
		InputSchema: map[string]any{"type": "object"},
	}
}

func TestBuildToolRequest(t *testing.T) {
	schema := personSchema()
	req := BuildToolRequest("anthropic/claude-sonnet-4-20250514", "Extract data", "John is 30", schema)

	if req.Model != "anthropic/claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want anthropic/claude-sonnet-4-20250514", req.Model)
	}
	if req.System != "Extract data" {
		t.Errorf("System = %q, want Extract data", req.System)
	}
	if len(req.Messages) != 1 {
		t.Fatalf("Messages len = %d, want 1", len(req.Messages))
	}
	if req.Messages[0].Role != domain.MessageRoleUser {
		t.Errorf("Messages[0].Role = %q, want user", req.Messages[0].Role)
	}
	if req.Messages[0].Content != "John is 30" {
		t.Errorf("Messages[0].Content = %q, want John is 30", req.Messages[0].Content)
	}
	if len(req.Tools) != 1 {
		t.Fatalf("Tools len = %d, want 1", len(req.Tools))
	}
	if req.Tools[0].Name != schema.Name {
		t.Errorf("Tools[0].Name = %q, want %q", req.Tools[0].Name, schema.Name)
	}
	if req.Tools[0].Description != schema.Description {
		t.Errorf("Tools[0].Description = %q, want %q", req.Tools[0].Description, schema.Description)
	}
	if req.ToolChoice == nil {
		t.Fatal("ToolChoice is nil")
	}
	if req.ToolChoice.Type != domain.ToolChoiceTool {
		t.Errorf("ToolChoice.Type = %q, want %q", req.ToolChoice.Type, domain.ToolChoiceTool)
	}
	if req.ToolChoice.Name != schema.Name {
		t.Errorf("ToolChoice.Name = %q, want %q", req.ToolChoice.Name, schema.Name)
	}
}

func TestExtractSuccess(t *testing.T) {
	fp := &fakeProvider{
		chatResp: &domain.ChatResponse{
			ToolCalls: []domain.ToolCall{
				{Name: "extract_person", Arguments: `{"name":"John","age":30}`},
			},
		},
	}
	result, err := Extract[Person](context.Background(), fp, "model", "system", "input", personSchema())
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Extract returned nil result")
	}
	if result.Name != "John" {
		t.Errorf("Name = %q, want John", result.Name)
	}
	if result.Age != 30 {
		t.Errorf("Age = %d, want 30", result.Age)
	}
}

func TestExtractEmptyArgumentsZeroValue(t *testing.T) {
	fp := &fakeProvider{
		chatResp: &domain.ChatResponse{
			ToolCalls: []domain.ToolCall{
				{Name: "extract_person", Arguments: "{}"},
			},
		},
	}
	result, err := Extract[Person](context.Background(), fp, "model", "system", "input", personSchema())
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Extract returned nil result")
	}
	if result.Name != "" {
		t.Errorf("Name = %q, want empty", result.Name)
	}
	if result.Age != 0 {
		t.Errorf("Age = %d, want 0", result.Age)
	}
}

func TestExtractErrors(t *testing.T) {
	schema := personSchema()
	providerErr := errors.New("provider boom")
	tests := []struct {
		name         string
		chatResp     *domain.ChatResponse
		chatErr      error
		wantSentinel error
	}{
		{
			name:     "provider error propagated",
			chatResp: nil,
			chatErr:  providerErr,
		},
		{
			name:     "no tool call",
			chatResp: &domain.ChatResponse{},
			chatErr:  nil,
			wantSentinel: ErrNoToolCall,
		},
		{
			name: "tool name mismatch",
			chatResp: &domain.ChatResponse{
				ToolCalls: []domain.ToolCall{
					{Name: "other_tool", Arguments: "{}"},
				},
			},
			wantSentinel: ErrToolNameMismatch,
		},
		{
			name: "unmarshal failed",
			chatResp: &domain.ChatResponse{
				ToolCalls: []domain.ToolCall{
					{Name: "extract_person", Arguments: "not valid json"},
				},
			},
			wantSentinel: ErrUnmarshalFailed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp := &fakeProvider{chatResp: tt.chatResp, chatErr: tt.chatErr}
			result, err := Extract[Person](context.Background(), fp, "model", "system", "input", schema)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if result != nil {
				t.Errorf("expected nil result on error, got %+v", result)
			}
			if tt.wantSentinel != nil && !errors.Is(err, tt.wantSentinel) {
				t.Errorf("errors.Is(err, %v) = false, want true (err=%v)", tt.wantSentinel, err)
			}
			if tt.chatErr != nil && !errors.Is(err, tt.chatErr) {
				t.Errorf("provider error not propagated: got %v", err)
			}
		})
	}
}

func TestExtractContextPropagation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fp := &fakeProvider{}
	_, err := Extract[Person](ctx, fp, "model", "system", "input", personSchema())
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if fp.gotCtx != ctx {
		t.Error("provider did not receive the same context passed to Extract")
	}
}

func TestExtractPassesRequestToProvider(t *testing.T) {
	fp := &fakeProvider{
		chatResp: &domain.ChatResponse{
			ToolCalls: []domain.ToolCall{
				{Name: "extract_person", Arguments: `{"name":"John","age":30}`},
			},
		},
	}
	_, err := Extract[Person](context.Background(), fp, "anthropic/claude-sonnet-4-20250514", "Extract a person", "John is 30", personSchema())
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if fp.gotReq == nil {
		t.Fatal("provider received nil request")
	}
	if fp.gotReq.Model != "anthropic/claude-sonnet-4-20250514" {
		t.Errorf("request Model = %q", fp.gotReq.Model)
	}
	if fp.gotReq.ToolChoice == nil || fp.gotReq.ToolChoice.Type != domain.ToolChoiceTool {
		t.Error("request did not carry forced tool choice")
	}
}
