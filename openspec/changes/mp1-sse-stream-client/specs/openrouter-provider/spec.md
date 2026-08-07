## MODIFIED Requirements

### Requirement: OpenRouterProvider implements Provider interface

The system SHALL implement `OpenRouterProvider` in `internal/adapters/driven/http_clients` that satisfies the `outbound.Provider` interface. It SHALL use the OpenAI Chat Completions wire format (`POST https://openrouter.ai/api/v1/chat/completions`).

#### Scenario: Non-streaming chat

- **WHEN** `Chat(ctx, req)` is called with a valid `ChatRequest`
- **THEN** the provider SHALL build an OpenAI-compatible request body, POST to `/api/v1/chat/completions`, parse the `ChatCompletionResponse`, and return a `*domain.ChatResponse`

#### Scenario: ChatStream returns a live streaming channel

- **WHEN** `ChatStream(ctx, req)` is called with a valid `ChatRequest`
- **THEN** the provider SHALL set `stream: true` on the wire request, POST to `/api/v1/chat/completions`, and return a `<-chan domain.StreamEvent` that emits `StreamEvent` values as SSE chunks arrive from OpenRouter

#### Scenario: ChatStream initial connection failure

- **WHEN** the initial HTTP request to OpenRouter fails (network error, non-2xx status)
- **THEN** `ChatStream` SHALL return a nil channel and a non-nil error, and SHALL NOT launch a streaming goroutine

#### Scenario: ChatStream finishes on [DONE]

- **WHEN** the SSE stream sends `data: [DONE]`
- **THEN** the provider SHALL close the event channel after emitting all pending events, signaling stream completion to the consumer
