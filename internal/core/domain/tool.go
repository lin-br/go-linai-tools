package domain

// Tool describes a callable tool available to the model.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// ToolChoice controls whether and how the model uses tools.
type ToolChoice struct {
	Type string
	Name string
}

// Common tool choice types.
const (
	ToolChoiceAuto = "auto"
	ToolChoiceNone = "none"
	ToolChoiceTool = "tool"
)

// ToolCall represents a tool invocation requested by the model.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}
