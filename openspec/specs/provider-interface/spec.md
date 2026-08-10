# Purpose

TBD

# Requirements

## Requirement: Provider interface defines Chat and ChatStream

The system SHALL define a `Provider` interface in `internal/core/ports/outbound` with two methods: `Chat(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error)` and `ChatStream(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamEvent, error)`. Both methods MUST accept `context.Context` as the first parameter.

### Scenario: Non-streaming chat request

- **WHEN** a caller invokes `Provider.Chat(ctx, req)` with a valid `ChatRequest`
- **THEN** the provider SHALL return a `*domain.ChatResponse` containing the model's full response, or an error if the request failed

### Scenario: Streaming chat request

- **WHEN** a caller invokes `Provider.ChatStream(ctx, req)` with a valid `ChatRequest`
- **THEN** the provider SHALL return a receive-only channel of `domain.StreamEvent` and a nil error, or an error if the initial connection failed

### Scenario: Stream channel closes on completion

- **WHEN** the stream is fully consumed or an error occurs mid-stream
- **THEN** the provider SHALL close the channel after sending any final error event

### Scenario: Context cancellation aborts request

- **WHEN** the context passed to `Chat` or `ChatStream` is cancelled
- **THEN** the provider SHALL abort the in-flight HTTP request and return an error

## Requirement: Entrypoint interface accepts context.Context

The system SHALL update the `Entrypoint` interface in `internal/core/ports/inbound` to accept `context.Context` as the first parameter of `StartAgent`.

### Scenario: Agent starts with context

- **WHEN** a caller invokes `StartAgent(ctx, in, out)`
- **THEN** the agent SHALL propagate the context to all downstream use case calls
