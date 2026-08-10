package domain

// ChatRequest is a provider-agnostic chat completion request.
type ChatRequest struct {
	Model       string
	Messages    []Message
	System      string
	Tools       []Tool
	ToolChoice  *ToolChoice
	MaxTokens   int64
	Temperature *float64
	TopP        *float64
	Stream      bool
}
