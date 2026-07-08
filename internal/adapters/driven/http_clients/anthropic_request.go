package http_clients

// MessagesRequest is the top-level request body for the Messages API.
type MessagesRequest struct {
	Model        string             `json:"model"`
	Messages     []Message          `json:"messages"`
	MaxTokens    int64              `json:"max_tokens"`
	System       []TextContentBlock `json:"system,omitempty"`
	Temperature  *float64           `json:"temperature,omitempty"`
	Thinking     *ThinkingConfig    `json:"thinking,omitempty"`
	ToolChoice   *ToolChoice        `json:"tool_choice,omitempty"`
	Tools        []ToolUnion        `json:"tools,omitempty"`
	TopK         *int64             `json:"top_k,omitempty"`
	TopP         *float64           `json:"top_p,omitempty"`
	StopSequences []string          `json:"stop_sequences,omitempty"`
	Metadata     *Metadata          `json:"metadata,omitempty"`
	ServiceTier  string             `json:"service_tier,omitempty"`
}

// Message represents a single message in the conversation.
type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// TextContentBlock is a text content block used in messages and system prompts.
type TextContentBlock struct {
	Type         string        `json:"type"`
	Text         string        `json:"text"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// ImageContentBlock is an image content block with a base64-encoded source.
type ImageContentBlock struct {
	Type         string        `json:"type"`
	Source       ImageSource   `json:"source"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// ImageSource holds the base64-encoded image data.
type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// ToolUseContentBlock represents a tool_use block in the conversation.
type ToolUseContentBlock struct {
	Type  string         `json:"type"`
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

// ToolResultContentBlock represents a tool_result block in the conversation.
type ToolResultContentBlock struct {
	Type      string      `json:"type"`
	ToolUseID string      `json:"tool_use_id"`
	Content   interface{} `json:"content,omitempty"`
	IsError   bool        `json:"is_error,omitempty"`
}

// ThinkingContentBlock represents a thinking block in the conversation.
type ThinkingContentBlock struct {
	Type      string `json:"type"`
	Thinking  string `json:"thinking"`
	Signature string `json:"signature"`
}

// DocumentContentBlock represents a document content block (PDF or plain text).
type DocumentContentBlock struct {
	Type         string        `json:"type"`
	Source       DocumentSource `json:"source"`
	Context      string        `json:"context,omitempty"`
	Title        string        `json:"title,omitempty"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// DocumentSource holds the base64-encoded document data.
type DocumentSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// ToolUnion is a union type for tool definitions (custom or server-side).
type ToolUnion struct {
	Type              string          `json:"type"`
	Name              string          `json:"name,omitempty"`
	Description       string          `json:"description,omitempty"`
	InputSchema       *ToolInputSchema `json:"input_schema,omitempty"`
	CacheControl      *CacheControl   `json:"cache_control,omitempty"`
	DeferLoading      bool            `json:"defer_loading,omitempty"`
	MaxContentTokens  *int64          `json:"max_content_tokens,omitempty"`
	MaxUses           *int64          `json:"max_uses,omitempty"`
	AllowedDomains    []string        `json:"allowed_domains,omitempty"`
	BlockedDomains    []string        `json:"blocked_domains,omitempty"`
	UserLocation      *UserLocation   `json:"user_location,omitempty"`
}

// ToolInputSchema defines the JSON Schema for a custom tool's input.
type ToolInputSchema struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties,omitempty"`
	Required   []string       `json:"required,omitempty"`
}

// UserLocation provides approximate geographic context for web search tools.
type UserLocation struct {
	Type     string `json:"type"`
	City     string `json:"city,omitempty"`
	Country  string `json:"country,omitempty"`
	Region   string `json:"region,omitempty"`
	Timezone string `json:"timezone,omitempty"`
}

// ThinkingConfig configures extended thinking.
type ThinkingConfig struct {
	Type         string `json:"type"`
	BudgetTokens int64  `json:"budget_tokens"`
}

// ToolChoice specifies how the model should use tools.
type ToolChoice struct {
	Type   string `json:"type"`
	Name   string `json:"name,omitempty"`
	DisableParallelToolUse bool `json:"disable_parallel_tool_use,omitempty"`
}

// CacheControl marks a content block for prompt caching.
type CacheControl struct {
	Type string `json:"type"`
}

// Metadata holds optional request metadata.
type Metadata struct {
	UserID string `json:"user_id,omitempty"`
}

// MessageRole constants
const (
	MessageRoleUser      = "user"
	MessageRoleAssistant = "assistant"
)

// ContentBlockType constants
const (
	ContentTypeText            = "text"
	ContentTypeImage           = "image"
	ContentTypeToolUse         = "tool_use"
	ContentTypeToolResult      = "tool_result"
	ContentTypeThinking        = "thinking"
	ContentTypeDocument        = "document"
)

// ToolType constants
const (
	ToolTypeCustom              = "custom"
	ToolTypeWebSearch20250305   = "web_search_20250305"
	ToolTypeWebFetch20250910    = "web_fetch_20250910"
	ToolTypeCodeExecution20250825 = "code_execution_20250825"
	ToolTypeTextEditor20250728  = "text_editor_20250728"
)

// ThinkingType constants
const (
	ThinkingTypeEnabled  = "enabled"
	ThinkingTypeDisabled = "disabled"
)

// ToolChoiceType constants
const (
	ToolChoiceAuto     = "auto"
	ToolChoiceAny      = "any"
	ToolChoiceTool     = "tool"
	ToolChoiceNone     = "none"
)

// Service tier constants
const (
	ServiceTierAuto         = "auto"
	ServiceTierStandardOnly = "standard_only"
)

// Image media type constants
const (
	ImageMediaTypeJPEG = "image/jpeg"
	ImageMediaTypePNG  = "image/png"
	ImageMediaTypeGIF  = "image/gif"
	ImageMediaTypeWebP = "image/webp"
)

// Document media type constants
const (
	DocumentMediaTypePDF  = "application/pdf"
	DocumentMediaTypeText = "text/plain"
)
