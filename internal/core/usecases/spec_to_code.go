package usecases

import (
	"context"
	"fmt"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
	"github.com/lin-br/go-linai-tools/internal/core/ports/outbound"
	"github.com/lin-br/go-linai-tools/internal/core/tools"
)

// specToCodeSystemPrompt is the system prompt that positions the model as a
// software architect producing a structured code plan — not implementation
// code. The %s placeholder is replaced with the target language hint.
const specToCodeSystemPrompt = "You are a software architect. Given a feature description, produce a structured code plan. Identify the files to create, the types to define, and the functions to implement. Be specific about paths, signatures, and field types. Do not write implementation code — only the plan. Target language: %s."

// specToCodeToolName is the forced tool name the model must call to return the
// structured code plan.
const specToCodeToolName = "generate_code_plan"

// specToCodeToolDescription is the human-readable description attached to the
// tool definition sent to the provider.
const specToCodeToolDescription = "Generate a structured code plan (files, types, functions with signatures) from a feature description."

// SpecToCodeUseCase wraps tools.Extract[CodePlan] behind a Plan method. It
// holds the provider (typically wrapped with RetryProvider), the resolved
// model identifier, and the target language hint injected into the system
// prompt. The tool schema is built per call via SchemaFromStruct.
type SpecToCodeUseCase struct {
	provider outbound.Provider
	model    string
	lang     string
}

// NewSpecToCodeUseCase constructs a SpecToCodeUseCase with the given provider,
// resolved model, and target language hint. The language is injected into the
// system prompt on each Plan call.
func NewSpecToCodeUseCase(provider outbound.Provider, model, lang string) *SpecToCodeUseCase {
	return &SpecToCodeUseCase{
		provider: provider,
		model:    model,
		lang:     lang,
	}
}

// Plan builds a ToolSchema for CodePlan via SchemaFromStruct, formats the
// system prompt with the target language, and calls tools.Extract[CodePlan].
// The context is passed unmodified to the provider via tools.Extract. Errors
// from Extract (ErrNoToolCall, ErrToolNameMismatch, ErrUnmarshalFailed,
// provider errors) are propagated directly without wrapping.
func (uc *SpecToCodeUseCase) Plan(ctx context.Context, input string) (*domain.CodePlan, error) {
	inputSchema, err := tools.SchemaFromStruct(&domain.CodePlan{})
	if err != nil {
		return nil, fmt.Errorf("spec_to_code: failed to build schema: %w", err)
	}

	schema := tools.ToolSchema{
		Name:        specToCodeToolName,
		Description: specToCodeToolDescription,
		InputSchema: inputSchema,
	}

	system := fmt.Sprintf(specToCodeSystemPrompt, uc.lang)
	return tools.Extract[domain.CodePlan](ctx, uc.provider, uc.model, system, input, schema)
}
