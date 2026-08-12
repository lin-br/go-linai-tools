package usecases

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
	"github.com/lin-br/go-linai-tools/internal/core/tools"
)

// validExtractArgs is a JSON string matching the ExtractionResult schema,
// used as the tool call arguments in success-case tests.
const validExtractArgs = `{
  "summary": "Meeting with John about budget",
  "entities": [{"name": "John", "type": "person"}],
  "action_items": ["Follow up with John"],
  "dates": ["2024-03-15"],
  "amounts": ["$500"]
}`

// 5.1 — ExtractUseCase.Extract with a fake Provider returning a valid tool
// call; verify the returned *ExtractionResult fields.
func TestExtractUseCaseSuccess(t *testing.T) {
	fake := &fakeProvider{
		chatResp: &domain.ChatResponse{
			ToolCalls: []domain.ToolCall{
				{Name: "extract_structured_data", Arguments: validExtractArgs},
			},
		},
	}
	uc := NewExtractUseCase(fake)

	result, err := uc.Extract(context.Background(), "anthropic/claude-sonnet-4-20250514", "Meeting with John on 2024-03-15 about $500 budget")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Extract returned nil result")
	}
	if result.Summary != "Meeting with John about budget" {
		t.Errorf("Summary = %q, want %q", result.Summary, "Meeting with John about budget")
	}
	if len(result.Entities) != 1 {
		t.Fatalf("Entities len = %d, want 1", len(result.Entities))
	}
	if result.Entities[0].Name != "John" {
		t.Errorf("Entities[0].Name = %q, want John", result.Entities[0].Name)
	}
	if result.Entities[0].Type != "person" {
		t.Errorf("Entities[0].Type = %q, want person", result.Entities[0].Type)
	}
	if len(result.ActionItems) != 1 || result.ActionItems[0] != "Follow up with John" {
		t.Errorf("ActionItems = %v, want [Follow up with John]", result.ActionItems)
	}
	if len(result.Dates) != 1 || result.Dates[0] != "2024-03-15" {
		t.Errorf("Dates = %v, want [2024-03-15]", result.Dates)
	}
	if len(result.Amounts) != 1 || result.Amounts[0] != "$500" {
		t.Errorf("Amounts = %v, want [$500]", result.Amounts)
	}
}

// 5.2 — ExtractUseCase.Extract error paths: provider error, ErrNoToolCall,
// ErrUnmarshalFailed (verify errors.Is).
func TestExtractUseCaseErrors(t *testing.T) {
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
			name:         "no tool call",
			chatResp:     &domain.ChatResponse{},
			chatErr:      nil,
			wantSentinel: tools.ErrNoToolCall,
		},
		{
			name: "unmarshal failed",
			chatResp: &domain.ChatResponse{
				ToolCalls: []domain.ToolCall{
					{Name: "extract_structured_data", Arguments: "not valid json"},
				},
			},
			wantSentinel: tools.ErrUnmarshalFailed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeProvider{chatResp: tt.chatResp, chatErr: tt.chatErr}
			uc := NewExtractUseCase(fake)

			result, err := uc.Extract(context.Background(), "model", "input")
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

// 5.3 — NewExtractUseCase generates the schema once and reuses it across
// calls. Two Extract calls should carry identical tool definitions.
func TestExtractUseCaseSchemaReused(t *testing.T) {
	fake := &fakeProvider{
		chatResp: &domain.ChatResponse{
			ToolCalls: []domain.ToolCall{
				{Name: "extract_structured_data", Arguments: `{}`},
			},
		},
	}
	uc := NewExtractUseCase(fake)

	if _, err := uc.Extract(context.Background(), "model", "first input"); err != nil {
		t.Fatalf("first Extract error: %v", err)
	}
	firstReq := fake.chatGotReq

	if _, err := uc.Extract(context.Background(), "model", "second input"); err != nil {
		t.Fatalf("second Extract error: %v", err)
	}
	secondReq := fake.chatGotReq

	if fake.chatCalls != 2 {
		t.Errorf("chatCalls = %d, want 2", fake.chatCalls)
	}
	if firstReq == nil || secondReq == nil {
		t.Fatal("provider did not receive requests")
	}
	if len(firstReq.Tools) != 1 || len(secondReq.Tools) != 1 {
		t.Fatalf("Tools len: first=%d second=%d, want 1", len(firstReq.Tools), len(secondReq.Tools))
	}
	if firstReq.Tools[0].Name != secondReq.Tools[0].Name {
		t.Errorf("tool Name differs: %q vs %q", firstReq.Tools[0].Name, secondReq.Tools[0].Name)
	}
	if firstReq.Tools[0].Description != secondReq.Tools[0].Description {
		t.Errorf("tool Description differs: %q vs %q", firstReq.Tools[0].Description, secondReq.Tools[0].Description)
	}
	if !reflect.DeepEqual(firstReq.Tools[0].InputSchema, secondReq.Tools[0].InputSchema) {
		t.Errorf("InputSchema differs across calls: first=%v second=%v", firstReq.Tools[0].InputSchema, secondReq.Tools[0].InputSchema)
	}
	if firstReq.Tools[0].Name != "extract_structured_data" {
		t.Errorf("tool Name = %q, want extract_structured_data", firstReq.Tools[0].Name)
	}
}

// 5.4 — Context propagation: a cancelled context is passed to the provider
// and the provider error (ctx.Err()) is returned.
func TestExtractUseCaseContextPropagation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	fake := &fakeProvider{
		chatResp: &domain.ChatResponse{
			ToolCalls: []domain.ToolCall{
				{Name: "extract_structured_data", Arguments: `{}`},
			},
		},
	}
	uc := NewExtractUseCase(fake)

	_, err := uc.Extract(ctx, "model", "input")
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if fake.chatGotCtx != ctx {
		t.Error("provider did not receive the same context passed to Extract")
	}
}

// DefaultExtractSystemPrompt is non-empty and directive.
func TestDefaultExtractSystemPromptNonEmpty(t *testing.T) {
	if DefaultExtractSystemPrompt == "" {
		t.Error("DefaultExtractSystemPrompt is empty")
	}
}
