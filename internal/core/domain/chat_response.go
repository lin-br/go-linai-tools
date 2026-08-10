package domain

// ChatResponse is a provider-agnostic chat completion response.
type ChatResponse struct {
	Content    string
	ToolCalls  []ToolCall
	StopReason string
	Usage      Usage
	Model      string
}
