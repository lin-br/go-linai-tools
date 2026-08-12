# Purpose

TBD

# Requirements

## Requirement: ToolSchema type

The system SHALL define a `ToolSchema` struct in `internal/core/tools` with fields: `Name string`, `Description string`, `InputSchema map[string]any`. `InputSchema` SHALL be a JSON Schema object compatible with `domain.Tool.InputSchema`. A `ToolSchema` MAY be constructed by hand or produced by `SchemaFromStruct`.

### Scenario: Hand-built tool schema

- **WHEN** a caller constructs `ToolSchema{Name: "extract_person", Description: "Extract a person", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}}}}`
- **THEN** the `ToolSchema` SHALL be usable directly by `BuildToolRequest` without further processing

## Requirement: SchemaFromStruct generates JSON Schema from Go structs

The system SHALL provide `SchemaFromStruct(v any) (map[string]any, error)` in `internal/core/tools`. It SHALL use `reflect` to walk the exported fields of the struct and produce a JSON Schema object with `type: "object"`, `properties`, and `required`. Field names SHALL be derived from `json` struct tags (the first comma-separated part); fields without a `json` tag SHALL use the Go field name. Fields with `omitempty` in their `json` tag SHALL be excluded from the `required` array; all other fields SHALL be included.

### Scenario: Simple struct with required and optional fields

- **WHEN** `SchemaFromStruct` is called with a struct having fields `Name string \`json:"name"\`` and `Age int \`json:"age,omitempty"\``
- **THEN** the returned schema SHALL have `type: "object"`, `properties` containing `name` (`{type: "string"}`) and `age` (`{type: "number"}`), and `required` containing only `["name"]`

### Scenario: Field without json tag uses Go field name

- **WHEN** `SchemaFromStruct` is called with a struct field `Title string` (no `json` tag)
- **THEN** the property key in the schema SHALL be `Title` (the Go field name)

### Scenario: Go type to JSON Schema type mapping

- **WHEN** `SchemaFromStruct` encounters a `string` field
- **THEN** the property type SHALL be `"string"`
- **WHEN** `SchemaFromStruct` encounters an `int`, `int64`, `int32`, `float64`, or `float32` field
- **THEN** the property type SHALL be `"number"`
- **WHEN** `SchemaFromStruct` encounters a `bool` field
- **THEN** the property type SHALL be `"boolean"`

### Scenario: Unexported fields are ignored

- **WHEN** `SchemaFromStruct` encounters a field where `reflect` reports it as unexported
- **THEN** the field SHALL be excluded from `properties` and `required`

### Scenario: Non-struct input returns error

- **WHEN** `SchemaFromStruct` is called with a non-struct value (e.g. a `string` or `int`)
- **THEN** it SHALL return an error

## Requirement: BuildToolRequest constructs forced-tool ChatRequest

The system SHALL provide `BuildToolRequest(model, system, input string, schema ToolSchema) *domain.ChatRequest` in `internal/core/tools`. The returned `ChatRequest` SHALL contain: `Model` set to the `model` argument, `System` set to the `system` argument, a single `Messages` entry with `Role: "user"` and `Content` set to `input`, a single `Tools` entry built from the `ToolSchema` (translated to `domain.Tool{Name, Description, InputSchema}`), and `ToolChoice` set to `&domain.ToolChoice{Type: "tool", Name: schema.Name}`.

### Scenario: Forced tool choice request

- **WHEN** `BuildToolRequest("anthropic/claude-sonnet-4-20250514", "Extract data", "John is 30", schema)` is called
- **THEN** the returned `ChatRequest` SHALL have `ToolChoice.Type` equal to `"tool"` and `ToolChoice.Name` equal to `schema.Name`, forcing the model to call that specific tool

### Scenario: Single tool definition

- **WHEN** `BuildToolRequest` builds the request
- **THEN** `ChatRequest.Tools` SHALL contain exactly one `domain.Tool` whose `Name`, `Description`, and `InputSchema` match the `ToolSchema`

### Scenario: User message contains input text

- **WHEN** `BuildToolRequest` is called with `input` set to `"John is 30"`
- **THEN** `ChatRequest.Messages` SHALL contain one message with `Role: "user"` and `Content: "John is 30"`

## Requirement: Extract generic function

The system SHALL provide a generic function `Extract[T any](ctx context.Context, p outbound.Provider, model, system, input string, schema ToolSchema) (*T, error)` in `internal/core/tools`. It SHALL call `BuildToolRequest` to construct the request, invoke `p.Chat(ctx, req)`, locate the first `ToolCall` in the response whose `Name` equals `schema.Name`, `json.Unmarshal` the `ToolCall.Arguments` string into a new `T`, and return a pointer to it.

### Scenario: Successful extraction

- **WHEN** `Extract[Person](ctx, provider, model, "Extract a person", "John is 30", schema)` is called and the provider returns a `ToolCall` with `Name: "extract_person"` and `Arguments: '{"name":"John","age":30}'`
- **THEN** the function SHALL return a `*Person` with `Name: "John"` and `Age: 30`, and a nil error

### Scenario: Provider error propagated

- **WHEN** `provider.Chat` returns a non-nil error
- **THEN** `Extract` SHALL return a nil `*T` and the error from the provider, without attempting to parse a response

## Requirement: Extract error handling for missing tool call

The system SHALL define an `ErrNoToolCall` error. When the provider returns a `ChatResponse` with no `ToolCalls` (nil or empty), `Extract` SHALL return a nil `*T` and `ErrNoToolCall`.

### Scenario: Model returns text instead of tool call

- **WHEN** the provider response has `ToolCalls` that is nil or empty
- **THEN** `Extract` SHALL return `ErrNoToolCall` so callers can detect that the model did not produce structured data

## Requirement: Extract error handling for tool name mismatch

The system SHALL define an `ErrToolNameMismatch` error. When the response contains one or more `ToolCalls` but none has a `Name` equal to `schema.Name`, `Extract` SHALL return a nil `*T` and `ErrToolNameMismatch`.

### Scenario: Tool call with wrong name

- **WHEN** the provider returns a `ToolCall` with `Name: "other_tool"` but `schema.Name` is `"extract_person"`
- **THEN** `Extract` SHALL return `ErrToolNameMismatch`

## Requirement: Extract error handling for unmarshal failure

The system SHALL define an `ErrUnmarshalFailed` error. When the matching `ToolCall.Arguments` string cannot be `json.Unmarshal`ed into `T`, `Extract` SHALL return a nil `*T` and `ErrUnmarshalFailed` wrapping the underlying error. Callers SHALL be able to use `errors.Is(err, ErrUnmarshalFailed)` to detect this case while the wrapped error preserves the detail.

### Scenario: Malformed JSON arguments

- **WHEN** the matching `ToolCall.Arguments` is `"not valid json"` and `T` is a struct
- **THEN** `Extract` SHALL return `ErrUnmarshalFailed` and `errors.Is(err, ErrUnmarshalFailed)` SHALL be true

### Scenario: Arguments missing expected fields

- **WHEN** the matching `ToolCall.Arguments` is `"{}"` and `T` has required fields
- **THEN** `json.Unmarshal` SHALL succeed (zero-value fields) and `Extract` SHALL return a `*T` with zero-valued fields and a nil error — validation of semantic completeness is the caller's responsibility

## Requirement: Context propagation in Extract

The `Extract` function SHALL pass the `context.Context` argument to `provider.Chat` without modification. If the context is cancelled or expired before the provider returns, `Extract` SHALL propagate the provider's error.

### Scenario: Cancelled context aborts extraction

- **WHEN** `Extract` is called with an already-cancelled context
- **THEN** `provider.Chat` SHALL receive the cancelled context and `Extract` SHALL return the provider's error
