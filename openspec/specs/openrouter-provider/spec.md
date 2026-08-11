# Purpose

TBD

# Requirements

## Requirement: OpenRouterProvider implements Provider interface

The system SHALL implement `OpenRouterProvider` in `internal/adapters/driven/http_clients` that satisfies the `outbound.Provider` interface. It SHALL use the OpenAI Chat Completions wire format (`POST https://openrouter.ai/api/v1/chat/completions`).

### Scenario: Non-streaming chat

- **WHEN** `Chat(ctx, req)` is called with a valid `ChatRequest`
- **THEN** the provider SHALL build an OpenAI-compatible request body, POST to `/api/v1/chat/completions`, parse the `ChatCompletionResponse`, and return a `*domain.ChatResponse`

### Scenario: ChatStream returns a live streaming channel

- **WHEN** `ChatStream(ctx, req)` is called with a valid `ChatRequest`
- **THEN** the provider SHALL set `stream: true` on the wire request, POST to `/api/v1/chat/completions`, and return a `<-chan domain.StreamEvent` that emits `StreamEvent` values as SSE chunks arrive from OpenRouter

### Scenario: ChatStream initial connection failure

- **WHEN** the initial HTTP request to OpenRouter fails (network error, non-2xx status)
- **THEN** `ChatStream` SHALL return a nil channel and a non-nil error, and SHALL NOT launch a streaming goroutine

### Scenario: ChatStream finishes on [DONE]

- **WHEN** the SSE stream sends `data: [DONE]`
- **THEN** the provider SHALL close the event channel after emitting all pending events, signaling stream completion to the consumer

## Requirement: OpenRouter authentication headers

The `OpenRouterProvider` SHALL include the `Authorization: Bearer {api_key}` header, the `HTTP-Referer` header, and the `X-OpenRouter-Title` header on every request.

### Scenario: Request includes auth headers

- **WHEN** any request is sent to OpenRouter
- **THEN** the HTTP request SHALL contain `Authorization`, `HTTP-Referer`, and `X-OpenRouter-Title` headers

## Requirement: Domain-to-wire translation

The `OpenRouterProvider` SHALL translate `domain.ChatRequest` to an OpenAI-compatible `ChatCompletionRequest` wire type, and translate the OpenAI `ChatCompletionResponse` back to `domain.ChatResponse`.

### Scenario: Multi-turn conversation

- **WHEN** a `ChatRequest` with multiple `Messages` (user, assistant, user) is sent
- **THEN** the wire request SHALL preserve the full message array with roles and content

### Scenario: System prompt translation

- **WHEN** a `ChatRequest` has a non-empty `System` field
- **THEN** the wire request SHALL include a system message as the first element of the `messages` array

### Scenario: Tool definition translation

- **WHEN** a `ChatRequest` has `Tools` set
- **THEN** each `domain.Tool` SHALL be translated to an OpenAI `tools[]` entry with `type: "function"` and a `function` object containing `name`, `description`, and `parameters` (from `InputSchema`)

### Scenario: Tool choice translation

- **WHEN** a `ChatRequest` has `ToolChoice{Type: "tool", Name: "extract_data"}`
- **THEN** the wire request SHALL set `tool_choice` to `{"type": "function", "function": {"name": "extract_data"}}`

### Scenario: Tool call response parsing

- **WHEN** the response contains `choices[0].message.tool_calls`
- **THEN** the provider SHALL parse each tool call into a `domain.ToolCall` with `ID`, `Name`, and `Arguments` (the raw JSON string from `function.arguments`)

## Requirement: Usage parsing

The `OpenRouterProvider` SHALL parse the `usage` field from the OpenRouter response into `domain.Usage`, including `Cost` when the `cost` field is present.

### Scenario: Usage with cost

- **WHEN** the response includes `usage.cost`
- **THEN** `domain.Usage.Cost` SHALL be non-nil

### Scenario: Usage without cost

- **WHEN** the response does not include `usage.cost`
- **THEN** `domain.Usage.Cost` SHALL be nil

## Requirement: HTTP request uses context

The `OpenRouterProvider` SHALL construct HTTP requests using `http.NewRequestWithContext(ctx, ...)`, not `http.NewRequest(...)`. The HTTP client SHALL NOT have a hardcoded timeout — timeouts are managed via `context.Context` (MP2 adds retry/timeout wrappers).

### Scenario: Context cancellation aborts HTTP

- **WHEN** the context is cancelled while an HTTP request is in flight
- **THEN** the request SHALL be aborted and an error returned

## Requirement: Error handling for HTTP errors

The `OpenRouterProvider` SHALL return a descriptive error when the HTTP response status is not 2xx. The error SHALL include the status code and the response body for debugging.

### Scenario: 401 unauthorized

- **WHEN** OpenRouter returns HTTP 401
- **THEN** the provider SHALL return an error containing the status code and response body

### Scenario: 429 rate limit

- **WHEN** OpenRouter returns HTTP 429
- **THEN** the provider SHALL return an error that can be identified as retryable by the retry wrapper (MP2)
