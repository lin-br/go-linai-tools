package domain

// Message represents a single message in a conversation.
type Message struct {
	Role        string
	Content     string
	ToolCalls   []ToolCall
	ToolCallID  string
}

// Common message roles.
const (
	MessageRoleUser      = "user"
	MessageRoleAssistant = "assistant"
	MessageRoleSystem    = "system"
	MessageRoleTool      = "tool"
)
