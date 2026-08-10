package http_clients

// ChatCompletionRequest is the OpenAI-compatible request body for chat completions.
type ChatCompletionRequest struct {
	Model         string           `json:"model"`
	Messages      []WireMessage    `json:"messages"`
	Tools         []WireTool       `json:"tools,omitempty"`
	ToolChoice    *WireToolChoice  `json:"tool_choice,omitempty"`
	MaxTokens     int64            `json:"max_tokens,omitempty"`
	Temperature   *float64         `json:"temperature,omitempty"`
	TopP          *float64         `json:"top_p,omitempty"`
	Stream        bool             `json:"stream,omitempty"`
}

// WireMessage represents a single message in the OpenAI wire format.
type WireMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCalls  []WireToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

// WireTool is a tool definition in the OpenAI wire format.
type WireTool struct {
	Type     string         `json:"type"`
	Function WireFunction   `json:"function"`
}

// WireFunction describes a callable function.
type WireFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// WireToolChoice controls tool selection in the OpenAI wire format.
type WireToolChoice struct {
	Type     string         `json:"type"`
	Function WireFuncChoice `json:"function,omitempty"`
}

// WireFuncChoice identifies a specific function to call.
type WireFuncChoice struct {
	Name string `json:"name"`
}

// WireToolCall is a tool invocation in the OpenAI wire format.
type WireToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function WireFuncCall `json:"function"`
}

// WireFuncCall holds the function details of a tool call.
type WireFuncCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatCompletionResponse is the OpenAI-compatible non-streaming response.
type ChatCompletionResponse struct {
	ID      string        `json:"id"`
	Model   string        `json:"model"`
	Choices []WireChoice  `json:"choices"`
	Usage   *WireUsage    `json:"usage,omitempty"`
}

// WireChoice represents one completion choice.
type WireChoice struct {
	Index        int          `json:"index"`
	Message      WireMessage  `json:"message"`
	FinishReason string       `json:"finish_reason"`
}

// WireUsage contains token usage and optional cost information.
type WireUsage struct {
	PromptTokens     int64    `json:"prompt_tokens"`
	CompletionTokens int64    `json:"completion_tokens"`
	TotalTokens      int64    `json:"total_tokens"`
	Cost             *float64 `json:"cost,omitempty"`
}

// ChatCompletionChunk is a single Server-Sent Event chunk.
type ChatCompletionChunk struct {
	ID      string             `json:"id"`
	Model   string             `json:"model"`
	Choices []WireChunkChoice  `json:"choices"`
	Usage   *WireUsage         `json:"usage,omitempty"`
}

// WireChunkChoice represents one choice within a streaming chunk.
type WireChunkChoice struct {
	Index        int       `json:"index"`
	Delta        WireDelta `json:"delta"`
	FinishReason *string   `json:"finish_reason,omitempty"`
}

// WireDelta is the incremental content in a streaming chunk.
type WireDelta struct {
	Role     string         `json:"role,omitempty"`
	Content  string         `json:"content,omitempty"`
	ToolCalls []WireToolCall `json:"tool_calls,omitempty"`
}
