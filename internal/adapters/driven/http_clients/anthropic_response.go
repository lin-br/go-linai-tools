package http_clients

// MessageResponse is the top-level response from the Messages API.
type MessageResponse struct {
	ID           string              `json:"id"`
	Container    *Container          `json:"container"`
	Content      []ContentBlock      `json:"content"`
	Model        string              `json:"model"`
	Role         string              `json:"role"`
	StopDetails  *RefusalStopDetails `json:"stop_details"`
	StopReason   *StopReason         `json:"stop_reason,omitempty"`
	StopSequence *string             `json:"stop_sequence"`
	Type         string              `json:"type"`
	Usage        Usage               `json:"usage"`
}

// Container holds information about the code execution container used in the request.
type Container struct {
	ID        string `json:"id"`
	ExpiresAt string `json:"expires_at"`
}

// ContentBlock is a union type for content blocks in the response.
// Only one of the typed fields will be populated, indicated by Type.
type ContentBlock struct {
	Type string `json:"type"`

	// Text
	Text      string     `json:"text,omitempty"`
	Citations []Citation `json:"citations,omitempty"`

	// Thinking / RedactedThinking
	Thinking  string `json:"thinking,omitempty"`
	Signature string `json:"signature,omitempty"`
	Data      string `json:"data,omitempty"`

	// ToolUse / ServerToolUse
	ID     string         `json:"id,omitempty"`
	Name   string         `json:"name,omitempty"`
	Input  map[string]any `json:"input,omitempty"`
	Caller *Caller        `json:"caller,omitempty"`

	// ToolResult
	ToolUseID string `json:"tool_use_id,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`

	// WebSearchResult
	EncryptedContent string `json:"encrypted_content,omitempty"`
	URL              string `json:"url,omitempty"`
	Title            string `json:"title,omitempty"`
	PageAge          string `json:"page_age,omitempty"`

	// WebFetchResult
	RetrievedAt string `json:"retrieved_at,omitempty"`

	// CodeExecutionResult
	ReturnCode      int64                 `json:"return_code,omitempty"`
	Stdout          string                `json:"stdout,omitempty"`
	Stderr          string                `json:"stderr,omitempty"`
	EncryptedStdout string                `json:"encrypted_stdout,omitempty"`
	CodeOutput      []CodeExecutionOutput `json:"content,omitempty"`

	// TextEditorResult
	FileType     string   `json:"file_type,omitempty"`
	NumLines     int64    `json:"num_lines,omitempty"`
	StartLine    int64    `json:"start_line,omitempty"`
	TotalLines   int64    `json:"total_lines,omitempty"`
	IsFileUpdate bool     `json:"is_file_update,omitempty"`
	Lines        []string `json:"lines,omitempty"`
	NewLines     int64    `json:"new_lines,omitempty"`
	NewStart     int64    `json:"new_start,omitempty"`
	OldLines     int64    `json:"old_lines,omitempty"`
	OldStart     int64    `json:"old_start,omitempty"`

	// Error
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`

	// ToolReference / ToolSearchResult
	ToolReferences []ToolReference `json:"tool_references,omitempty"`
	Source         string          `json:"source,omitempty"`
	FileID         string          `json:"file_id,omitempty"`
}

// CodeExecutionOutput represents a file output from code execution.
type CodeExecutionOutput struct {
	FileID string `json:"file_id"`
	Type   string `json:"type"`
}

// ToolReference points to a tool returned by tool search.
type ToolReference struct {
	ToolName string `json:"tool_name"`
	Type     string `json:"type"`
}

// Citation supports text block citations from documents.
type Citation struct {
	Type              string `json:"type"`
	CitedText         string `json:"cited_text"`
	DocumentIndex     int64  `json:"document_index"`
	DocumentTitle     string `json:"document_title"`
	FileID            string `json:"file_id,omitempty"`
	StartCharIndex    int64  `json:"start_char_index,omitempty"`
	EndCharIndex      int64  `json:"end_char_index,omitempty"`
	StartPageNumber   int64  `json:"start_page_number,omitempty"`
	EndPageNumber     int64  `json:"end_page_number,omitempty"`
	StartBlockIndex   int64  `json:"start_block_index,omitempty"`
	EndBlockIndex     int64  `json:"end_block_index,omitempty"`
	SearchResultIndex int64  `json:"search_result_index,omitempty"`
	EncryptedIndex    string `json:"encrypted_index,omitempty"`
	URL               string `json:"url,omitempty"`
	Source            string `json:"source,omitempty"`
	Title             string `json:"title,omitempty"`
}

// Caller represents who invoked a tool. Only one field will be populated.
type Caller struct {
	Type   string `json:"type"`
	ToolID string `json:"tool_id,omitempty"`
}

// Usage contains billing and rate-limit usage information.
type Usage struct {
	CacheCreation            *CacheCreation       `json:"cache_creation,omitempty"`
	CacheCreationInputTokens int64                `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64                `json:"cache_read_input_tokens"`
	InferenceGeo             string               `json:"inference_geo,omitempty"`
	InputTokens              int64                `json:"input_tokens"`
	OutputTokens             int64                `json:"output_tokens"`
	OutputTokensDetails      *OutputTokensDetails `json:"output_tokens_details,omitempty"`
	ServerToolUse            *ServerToolUsage     `json:"server_tool_use,omitempty"`
	ServiceTier              string               `json:"service_tier,omitempty"`
}

// CacheCreation breaks down cached tokens by TTL.
type CacheCreation struct {
	Ephemeral1hInputTokens int64 `json:"ephemeral_1h_input_tokens"`
	Ephemeral5mInputTokens int64 `json:"ephemeral_5m_input_tokens"`
}

// OutputTokensDetails provides a breakdown of output tokens.
type OutputTokensDetails struct {
	ThinkingTokens int64 `json:"thinking_tokens"`
}

// ServerToolUsage tracks server-side tool request counts.
type ServerToolUsage struct {
	WebFetchRequests  int64 `json:"web_fetch_requests"`
	WebSearchRequests int64 `json:"web_search_requests"`
}

// StopReason is the reason the model stopped generating.
type StopReason string

const (
	StopReasonEndTurn      StopReason = "end_turn"
	StopReasonMaxTokens    StopReason = "max_tokens"
	StopReasonStopSequence StopReason = "stop_sequence"
	StopReasonToolUse      StopReason = "tool_use"
	StopReasonPauseTurn    StopReason = "pause_turn"
	StopReasonRefusal      StopReason = "refusal"
)

// RefusalStopDetails holds structured information about a refusal.
type RefusalStopDetails struct {
	Category    string `json:"category"`
	Explanation string `json:"explanation"`
	Type        string `json:"type"`
}
