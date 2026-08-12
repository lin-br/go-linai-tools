## ADDED Requirements

### Requirement: Provider package exports a factory and config types

The system SHALL create an `internal/providers/` package. The package SHALL export a `Provider` interface that embeds `outbound.Provider` so that every returned implementation satisfies `Chat(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error)` and `ChatStream(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamEvent, error)`. The package SHALL export `func New(kind string, cfg Config) (Provider, error)` and a `Config` struct with fields for provider kind, API key, default model, and region (used only by Bedrock).

#### Scenario: Factory returns Anthropic provider
- **WHEN** `New("anthropic", cfg)` is called with a non-empty Anthropic API key
- **THEN** it SHALL return a non-nil `*AnthropicProvider` and a nil error

#### Scenario: Factory returns error for unknown kind
- **WHEN** `New("unknown", cfg)` is called
- **THEN** it SHALL return a nil provider and an error whose message contains "unknown provider kind"

#### Scenario: All factory providers satisfy the interface
- **WHEN** `New` returns a provider for any supported kind
- **THEN** the returned value SHALL be assignable to the `Provider` interface

### Requirement: Config validates required credentials

The system SHALL define a `Config` struct in `internal/providers/config.go` with at least `Provider string`, `APIKey string`, `Model string`, and `Region string`. The factory SHALL validate that the active provider's required credential is present: Anthropic/OpenAI/Gemini require a non-empty `APIKey`; Bedrock requires non-empty `Region`. Missing credentials SHALL produce a wrapped error using `fmt.Errorf("...: %w", err)`.

#### Scenario: Missing Anthropic API key
- **WHEN** `New("anthropic", Config{APIKey: ""})` is called
- **THEN** it SHALL return an error wrapping `ErrMissingCredential`

#### Scenario: Bedrock requires region
- **WHEN** `New("bedrock", Config{APIKey: "x", Region: ""})` is called
- **THEN** it SHALL return an error wrapping `ErrMissingCredential`

#### Scenario: OpenAI and Gemini require API key
- **WHEN** `New("openai", Config{APIKey: ""})` or `New("gemini", Config{APIKey: ""})` is called
- **THEN** it SHALL return an error wrapping `ErrMissingCredential`

### Requirement: AnthropicProvider implements direct Messages API Chat

The system SHALL define `AnthropicProvider` in `internal/providers/anthropic.go`. It SHALL implement `Chat` by POSTing to `https://api.anthropic.com/v1/messages` with `Content-Type: application/json` and `x-api-key` headers. It SHALL translate `domain.ChatRequest` into the Anthropic `MessagesRequest` shape: system prompt inside the `System []TextContentBlock` field; user messages in the `Messages []Message` field; tools mapped to Anthropic `ToolUnion` with `InputSchema`; `ToolChoice` mapped when non-nil. It SHALL translate the `MessageResponse` back into `domain.ChatResponse`, copying `Content` text, `ToolCalls` from tool_use blocks, and usage fields.

#### Scenario: Non-streaming chat returns text
- **WHEN** `AnthropicProvider.Chat(ctx, &domain.ChatRequest{System: "say hi", Messages: []domain.Message{{Role: "user", Content: "hello"}}})` is called and the mock server returns a `MessageResponse` with one text block
- **THEN** it SHALL return a `*domain.ChatResponse` with `Content == "hi"` and a nil error

#### Scenario: Tool choice forces a tool call
- **WHEN** `AnthropicProvider.Chat(ctx, req)` is called with a non-nil `ToolChoice` of type `domain.ToolChoiceTool`
- **THEN** the outgoing `MessagesRequest.ToolChoice` SHALL be set with `Type: "tool"` and `Name: req.ToolChoice.Name`

#### Scenario: Error on non-2xx response
- **WHEN** the mock server returns HTTP 401
- **THEN** `AnthropicProvider.Chat` SHALL return a non-nil error wrapping the status code and response body

### Requirement: AnthropicProvider implements ChatStream with typed not-implemented error

The system SHALL implement `AnthropicProvider.ChatStream` so that it returns a nil channel and an error wrapping the sentinel `ErrStreamingNotImplemented`. This preserves the `outbound.Provider` interface contract while documenting that Phase 4 direct providers do not stream.

#### Scenario: ChatStream returns typed error
- **WHEN** `AnthropicProvider.ChatStream(ctx, req)` is called
- **THEN** it SHALL return `nil, ErrStreamingNotImplemented` and `errors.Is(err, ErrStreamingNotImplemented)` SHALL be true

### Requirement: AnthropicProvider uses existing domain types only

The system SHALL reuse `domain.ChatRequest`, `domain.ChatResponse`, `domain.Message`, `domain.Tool`, `domain.ToolCall`, `domain.ToolChoice`, and `domain.Usage` in `AnthropicProvider` signatures and internal mapping. It SHALL define Anthropic-specific wire structs in `internal/providers/anthropic_request.go` and `internal/providers/anthropic_response.go` if needed, but those structs SHALL only be used inside the provider package.

#### Scenario: No leakage of wire types outside the package
- **WHEN** `cmd/model-roulette` imports `internal/providers`
- **THEN** it SHALL NOT need to import any `anthropic_*.go` wire type

### Requirement: Table-driven unit tests for the factory and AnthropicProvider

The system SHALL add `internal/providers/providers_test.go` and `internal/providers/anthropic_test.go` using table-driven subtests. Tests SHALL cover factory success and error paths, `Chat` response mapping, non-2xx handling, and `ChatStream` not-implemented behavior. HTTP tests SHALL use `net/http/httptest`.

#### Scenario: Factory table tests
- **WHEN** running `go test ./internal/providers/...`
- **THEN** table-driven tests for `New` SHALL pass for each supported kind and each missing-credential case

#### Scenario: Anthropic chat table tests
- **WHEN** running `go test ./internal/providers/...`
- **THEN** table-driven tests for `AnthropicProvider.Chat` SHALL pass for text responses, tool responses, and HTTP errors
