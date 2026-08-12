package usecases

import (
	"context"

	"github.com/lin-br/go-linai-tools/internal/core/ports/outbound"
	"github.com/lin-br/go-linai-tools/internal/core/tools"
)

// DefaultExtractSystemPrompt instructs the model to extract structured
// information and to use empty arrays or empty strings for fields with no
// data, never guessing.
const DefaultExtractSystemPrompt = "Extract structured information from the following text. Fill in all fields. If a field has no data, use an empty array or empty string — do not guess."

// extractToolName is the forced tool name the model must call to return
// structured data. It matches the ToolSchema.Name used by ExtractUseCase.
const extractToolName = "extract_structured_data"

// extractToolDescription is the human-readable description attached to the
// extraction tool definition sent to the provider.
const extractToolDescription = "Extract structured information (summary, entities, action items, dates, amounts) from the input text."

// buildExtractSchema constructs the JSON Schema for the extraction tool.
//
// This is hand-built rather than generated via tools.SchemaFromStruct because
// SchemaFromStruct is deliberately shallow (MP3 D3): []Entity and []string
// both produce {type: "array"} with no item schema, leaving the model unable
// to infer what the arrays should contain. The hand-built schema describes
// array items explicitly — entities as objects with name+type, the rest as
// arrays of strings — so the model fills in the correct shape.
func buildExtractSchema() tools.ToolSchema {
	return tools.ToolSchema{
		Name:        extractToolName,
		Description: extractToolDescription,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"summary": map[string]any{
					"type":        "string",
					"description": "A concise summary of the input text",
				},
				"entities": map[string]any{
					"type":        "array",
					"description": "Named entities (people, organizations, locations, etc.) found in the text",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{
								"type":        "string",
								"description": "The name of the entity",
							},
							"type": map[string]any{
								"type":        "string",
								"description": "The category of the entity (e.g. person, organization, location)",
							},
						},
						"required": []string{"name", "type"},
					},
				},
				"action_items": map[string]any{
					"type":        "array",
					"description": "Actionable tasks or next steps mentioned in the text",
					"items": map[string]any{
						"type": "string",
					},
				},
				"dates": map[string]any{
					"type":        "array",
					"description": "Dates mentioned in the text (any format)",
					"items": map[string]any{
						"type": "string",
					},
				},
				"amounts": map[string]any{
					"type":        "array",
					"description": "Monetary amounts or quantities mentioned in the text",
					"items": map[string]any{
						"type": "string",
					},
				},
			},
			"required": []string{"summary", "entities", "action_items", "dates", "amounts"},
		},
	}
}

// ExtractUseCase wraps tools.Extract[ExtractionResult] with a predefined
// ToolSchema. It holds an outbound.Provider (typically wrapped with
// RetryProvider at wiring time) and reuses the same schema for every Extract
// call.
type ExtractUseCase struct {
	provider outbound.Provider
	schema   tools.ToolSchema
}

// NewExtractUseCase constructs an ExtractUseCase that delegates to the given
// provider. The tool schema is built once via buildExtractSchema and reused
// for all subsequent Extract calls.
func NewExtractUseCase(provider outbound.Provider) *ExtractUseCase {
	return &ExtractUseCase{
		provider: provider,
		schema:   buildExtractSchema(),
	}
}

// Extract calls tools.Extract[ExtractionResult] with the predefined schema and
// DefaultExtractSystemPrompt. The context is passed unmodified to the provider
// via tools.Extract. Returns the typed result or an error from the provider /
// tools.Extract (ErrNoToolCall, ErrUnmarshalFailed, etc.).
func (uc *ExtractUseCase) Extract(ctx context.Context, model, input string) (*ExtractionResult, error) {
	return tools.Extract[ExtractionResult](ctx, uc.provider, model, DefaultExtractSystemPrompt, input, uc.schema)
}
