package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
	"github.com/lin-br/go-linai-tools/internal/core/tools"
)

// validPlanArgs is a JSON string matching the CodePlan schema, used as the tool
// call arguments in success-case tests.
const validPlanArgs = `{
  "summary": "User authentication system",
  "language": "go",
  "files": [
    {
      "path": "internal/core/domain/user.go",
      "description": "User domain types",
      "types": [
        {
          "name": "User",
          "fields": [
            {"name": "ID", "type": "string"},
            {"name": "Email", "type": "string"}
          ]
        }
      ],
      "functions": [
        {
          "name": "NewUser",
          "signature": "func NewUser(email string) *User"
        }
      ]
    }
  ]
}`

// 5.1 — Plan success path: fake Provider returns a valid CodePlan tool call;
// assert typed *CodePlan returned with populated fields.
func TestSpecToCodeUseCaseSuccess(t *testing.T) {
	fake := &fakeProvider{
		chatResp: &domain.ChatResponse{
			ToolCalls: []domain.ToolCall{
				{Name: "generate_code_plan", Arguments: validPlanArgs},
			},
		},
	}
	uc := NewSpecToCodeUseCase(fake, "anthropic/claude-sonnet-4-20250514", "go")

	plan, err := uc.Plan(context.Background(), "Add user authentication with login and register")
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if plan == nil {
		t.Fatal("Plan returned nil result")
	}
	if plan.Summary != "User authentication system" {
		t.Errorf("Summary = %q, want %q", plan.Summary, "User authentication system")
	}
	if plan.Language != "go" {
		t.Errorf("Language = %q, want go", plan.Language)
	}
	if len(plan.Files) != 1 {
		t.Fatalf("Files len = %d, want 1", len(plan.Files))
	}
	if plan.Files[0].Path != "internal/core/domain/user.go" {
		t.Errorf("Files[0].Path = %q", plan.Files[0].Path)
	}
	if len(plan.Files[0].Types) != 1 {
		t.Fatalf("Files[0].Types len = %d, want 1", len(plan.Files[0].Types))
	}
	if plan.Files[0].Types[0].Name != "User" {
		t.Errorf("Files[0].Types[0].Name = %q, want User", plan.Files[0].Types[0].Name)
	}
	if len(plan.Files[0].Types[0].Fields) != 2 {
		t.Fatalf("Fields len = %d, want 2", len(plan.Files[0].Types[0].Fields))
	}
	if plan.Files[0].Types[0].Fields[0].Name != "ID" {
		t.Errorf("Fields[0].Name = %q, want ID", plan.Files[0].Types[0].Fields[0].Name)
	}
	if len(plan.Files[0].Functions) != 1 {
		t.Fatalf("Functions len = %d, want 1", len(plan.Files[0].Functions))
	}
	if plan.Files[0].Functions[0].Name != "NewUser" {
		t.Errorf("Functions[0].Name = %q, want NewUser", plan.Files[0].Functions[0].Name)
	}

	// Verify the system prompt was formatted with the language.
	if fake.chatGotReq == nil {
		t.Fatal("provider did not receive a request")
	}
	if fake.chatGotReq.System == "" {
		t.Error("system prompt is empty")
	}
}

// 5.2 — Plan propagates ErrNoToolCall when provider returns no tool calls.
func TestSpecToCodeUseCaseErrNoToolCall(t *testing.T) {
	fake := &fakeProvider{
		chatResp: &domain.ChatResponse{},
	}
	uc := NewSpecToCodeUseCase(fake, "model", "go")

	plan, err := uc.Plan(context.Background(), "input")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if plan != nil {
		t.Errorf("expected nil plan on error, got %+v", plan)
	}
	if !errors.Is(err, tools.ErrNoToolCall) {
		t.Errorf("errors.Is(err, ErrNoToolCall) = false, want true (err=%v)", err)
	}
}

// 5.3 — Plan propagates ErrUnmarshalFailed when tool call arguments are invalid
// JSON.
func TestSpecToCodeUseCaseErrUnmarshalFailed(t *testing.T) {
	fake := &fakeProvider{
		chatResp: &domain.ChatResponse{
			ToolCalls: []domain.ToolCall{
				{Name: "generate_code_plan", Arguments: "not valid json"},
			},
		},
	}
	uc := NewSpecToCodeUseCase(fake, "model", "go")

	plan, err := uc.Plan(context.Background(), "input")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if plan != nil {
		t.Errorf("expected nil plan on error, got %+v", plan)
	}
	if !errors.Is(err, tools.ErrUnmarshalFailed) {
		t.Errorf("errors.Is(err, ErrUnmarshalFailed) = false, want true (err=%v)", err)
	}
}

// 5.4 — Plan propagates provider errors directly.
func TestSpecToCodeUseCaseProviderError(t *testing.T) {
	providerErr := errors.New("provider boom")
	fake := &fakeProvider{
		chatErr: providerErr,
	}
	uc := NewSpecToCodeUseCase(fake, "model", "go")

	plan, err := uc.Plan(context.Background(), "input")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if plan != nil {
		t.Errorf("expected nil plan on error, got %+v", plan)
	}
	if !errors.Is(err, providerErr) {
		t.Errorf("provider error not propagated: got %v", err)
	}
}

// 5.5 — Plan passes ctx through to provider.Chat; cancelled context returns
// provider error (ctx.Err()).
func TestSpecToCodeUseCaseContextPropagation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	fake := &fakeProvider{
		chatResp: &domain.ChatResponse{
			ToolCalls: []domain.ToolCall{
				{Name: "generate_code_plan", Arguments: `{}`},
			},
		},
	}
	uc := NewSpecToCodeUseCase(fake, "model", "go")

	_, err := uc.Plan(ctx, "input")
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if fake.chatGotCtx != ctx {
		t.Error("provider did not receive the same context passed to Plan")
	}
}

// Verify the system prompt constant contains the required phrases.
func TestSpecToCodeSystemPromptContainsRequiredPhrases(t *testing.T) {
	if !strings.Contains(specToCodeSystemPrompt, "software architect") {
		t.Error("system prompt must contain 'software architect'")
	}
	if !strings.Contains(specToCodeSystemPrompt, "Do not write implementation code") {
		t.Error("system prompt must contain 'Do not write implementation code'")
	}
	if !strings.Contains(specToCodeSystemPrompt, "Target language: %s") {
		t.Error("system prompt must contain the Target language placeholder")
	}
}
