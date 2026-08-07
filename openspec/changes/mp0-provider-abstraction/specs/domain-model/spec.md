## ADDED Requirements

### Requirement: ChatRequest domain type

The system SHALL define a `ChatRequest` struct in `internal/core/domain` with fields: `Model string`, `Messages []Message`, `System string`, `Tools []Tool`, `ToolChoice *ToolChoice`, `MaxTokens int64`, `Temperature *float64`, `TopP *float64`, `Stream bool`.

#### Scenario: Minimal chat request

- **WHEN** a `ChatRequest` is constructed with only `Model` and `Messages`
- **THEN** all optional fields (`System`, `Tools`, `ToolChoice`, `MaxTokens`, `Temperature`, `TopP`, `Stream`) SHALL be zero-valued and omittable from serialization

### Requirement: ChatResponse domain type

The system SHALL define a `ChatResponse` struct with fields: `Content string`, `ToolCalls []ToolCall`, `StopReason string`, `Usage Usage`, `Model string`.

#### Scenario: Text-only response

- **WHEN** the model returns a text response with no tool calls
- **THEN** `ChatResponse.Content` SHALL contain the concatenated text, `ToolCalls` SHALL be nil, and `StopReason` SHALL be "stop"

### Requirement: StreamEvent domain type

The system SHALL define a `StreamEvent` struct with fields: `Type StreamEventType`, `Delta string`, `StopReason string`, `Usage *Usage`, `Err error`. The `StreamEventType` constants SHALL include at minimum: `StreamEventTypeText`, `StreamEventTypeFinish`, `StreamEventTypeError`.

#### Scenario: Text delta event

- **WHEN** the provider receives a streaming chunk with text content
- **THEN** a `StreamEvent` with `Type=StreamEventTypeText` and `Delta` containing the incremental text SHALL be sent on the channel

#### Scenario: Finish event

- **WHEN** the stream completes normally
- **THEN** a `StreamEvent` with `Type=StreamEventTypeFinish`, `StopReason` set to the provider's stop reason, and `Usage` populated (if available) SHALL be sent, followed by channel closure

#### Scenario: Error event

- **WHEN** an error occurs mid-stream
- **THEN** a `StreamEvent` with `Type=StreamEventTypeError` and `Err` set SHALL be sent, followed by channel closure

### Requirement: Message domain type

The system SHALL define a `Message` struct with fields: `Role string`, `Content string`, `ToolCalls []ToolCall`, `ToolCallID string`. Roles SHALL include "user", "assistant", "system", "tool".

#### Scenario: User message

- **WHEN** a user message is constructed with `Role="user"` and `Content="Hello"`
- **THEN** `ToolCalls` and `ToolCallID` SHALL be zero-valued

#### Scenario: Tool result message

- **WHEN** a tool result message is constructed with `Role="tool"`, `Content="42"`, `ToolCallID="call_123"`
- **THEN** the message represents a tool execution result linked to the original tool call

### Requirement: Tool and ToolChoice domain types

The system SHALL define a `Tool` struct with fields: `Name string`, `Description string`, `InputSchema map[string]any`. The system SHALL define a `ToolChoice` struct with fields: `Type string`, `Name string`. `ToolChoice.Type` SHALL accept "auto", "none", "tool".

#### Scenario: Forced tool choice

- **WHEN** `ToolChoice{Type: "tool", Name: "extract_data"}` is set on a `ChatRequest`
- **THEN** the model SHALL be forced to call the named tool

### Requirement: ToolCall domain type

The system SHALL define a `ToolCall` struct with fields: `ID string`, `Name string`, `Arguments string`. `Arguments` SHALL be a JSON string (not a parsed object) to remain provider-agnostic.

#### Scenario: Tool call from model

- **WHEN** the model returns a tool call with `Name="extract_data"` and `Arguments='{"key":"value"}'`
- **THEN** the `ToolCall` SHALL preserve the arguments as a raw JSON string

### Requirement: Usage domain type

The system SHALL define a `Usage` struct with fields: `InputTokens int64`, `OutputTokens int64`, `TotalTokens int64`, `Cost *float64`. `Cost` SHALL be optional (pointer).

#### Scenario: Usage with cost

- **WHEN** the provider returns usage data including cost
- **THEN** `Usage.Cost` SHALL be non-nil and `TotalTokens` SHALL equal `InputTokens + OutputTokens`
