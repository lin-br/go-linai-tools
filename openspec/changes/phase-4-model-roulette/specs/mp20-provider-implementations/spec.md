## ADDED Requirements

### Requirement: OpenAIProvider implements direct Completions API Chat

The system SHALL define `OpenAIProvider` in `internal/providers/openai.go`. It SHALL implement `Chat` by POSTing to `https://api.openai.com/v1/chat/completions` with `Content-Type: application/json` and `Authorization: Bearer <api_key>` headers. It SHALL use OpenAI-compatible wire types `ChatCompletionRequest`, `ChatCompletionResponse`, and `ChatCompletionChunk`. It SHALL translate `domain.ChatRequest` to the request wire, mapping `System` to a message with role `system`, `Messages` to the `messages` array, tools to OpenAI tool format, and `ToolChoice` to OpenAI tool_choice. It SHALL translate the response back into `domain.ChatResponse`, copying content, stop reason, tool calls, and usage.

#### Scenario: Non-streaming chat returns text
- **WHEN** `OpenAIProvider.Chat(ctx, &domain.ChatRequest{System: "be brief", Messages: []domain.Message{{Role: "user", Content: "hi"}}})` is called and the mock server returns a `ChatCompletionResponse` with one assistant message
- **THEN** it SHALL return a `*domain.ChatResponse` with `Content == "hello"` and a nil error

#### Scenario: Tool calls are mapped from response
- **WHEN** the mock server returns a response with `choices[0].message.tool_calls` containing one function call
- **THEN** the returned `domain.ChatResponse.ToolCalls` SHALL contain one element with matching `Name` and `Arguments`

#### Scenario: Error on non-2xx response
- **WHEN** the mock server returns HTTP 429
- **THEN** `OpenAIProvider.Chat` SHALL return a non-nil error whose message includes the status code and body

### Requirement: OpenAIProvider ChatStream uses SSE

The system SHALL implement `OpenAIProvider.ChatStream` by sending a request with `Stream: true` and parsing `data:` prefixed SSE chunks. It SHALL emit `domain.StreamEvent` values for content deltas, finish reasons, and final usage. It SHALL close the channel and stop reading when the context is cancelled.

#### Scenario: Stream returns text deltas
- **WHEN** `OpenAIProvider.ChatStream(ctx, req)` is called and the mock server returns SSE chunks `data: {"choices":[{"delta":{"content":"hello"}}]}` and `data: [DONE]`
- **THEN** the returned channel SHALL emit at least one `domain.StreamEvent` with `Content == "hello"`

#### Scenario: Context cancellation stops stream
- **WHEN** the context is cancelled mid-stream
- **THEN** the stream goroutine SHALL stop reading and the channel SHALL close without a panic

### Requirement: GeminiProvider implements generateContent Chat

The system SHALL define `GeminiProvider` in `internal/providers/gemini.go`. It SHALL implement `Chat` by POSTing to `https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent?key={api_key}`. It SHALL translate `domain.ChatRequest` into a `GenerateContentRequest` with `contents` containing system and user turns, and `generationConfig` holding `maxOutputTokens` and `temperature`. It SHALL map the `GenerateContentResponse` back into `domain.ChatResponse`, extracting the first candidate's text content and usage metadata when available.

#### Scenario: Non-streaming chat returns text
- **WHEN** `GeminiProvider.Chat(ctx, &domain.ChatRequest{System: "be brief", Messages: []domain.Message{{Role: "user", Content: "hi"}}})` is called and the mock server returns a single-candidate response
- **THEN** it SHALL return a `*domain.ChatResponse` with non-empty `Content` and a nil error

#### Scenario: Model name is interpolated into the URL
- **WHEN** `GeminiProvider.Chat(ctx, &domain.ChatRequest{Model: "gemini-1.5-flash"})` is called
- **THEN** the outgoing HTTP request URL path SHALL contain `models/gemini-1.5-flash:generateContent`

#### Scenario: Error on non-2xx response
- **WHEN** the mock server returns HTTP 400 with a JSON error body
- **THEN** `GeminiProvider.Chat` SHALL return a non-nil error whose message includes the status code

### Requirement: GeminiProvider ChatStream returns typed not-implemented error

The system SHALL implement `GeminiProvider.ChatStream` so that it returns `nil, ErrStreamingNotImplemented`, matching the AnthropicProvider behavior and preserving the shared interface contract.

#### Scenario: ChatStream returns typed error
- **WHEN** `GeminiProvider.ChatStream(ctx, req)` is called
- **THEN** it SHALL return `nil, ErrStreamingNotImplemented` and `errors.Is(err, ErrStreamingNotImplemented)` SHALL be true

### Requirement: BedrockProvider implements InvokeModel Chat

The system SHALL define `BedrockProvider` in `internal/providers/bedrock.go`. It SHALL implement `Chat` by POSTing to `https://bedrock-runtime.{region}.amazonaws.com/model/{modelId}/invoke` with a SigV4-signed request using `Authorization`, `X-Amz-Date`, and `X-Amz-Content-Sha256` headers. It SHALL read credentials from the environment (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`) and `AWS_REGION` from the config. The request body SHALL be the Bedrock Converse/InvokeModel payload; the response SHALL be parsed and mapped to `domain.ChatResponse`.

#### Scenario: Non-streaming chat returns text
- **WHEN** `BedrockProvider.Chat(ctx, &domain.ChatRequest{Model: "anthropic.claude-3-sonnet-20240229-v1:0", Messages: []domain.Message{{Role: "user", Content: "hi"}}})` is called and the mock server returns a Bedrock inference response
- **THEN** it SHALL return a `*domain.ChatResponse` with non-empty `Content` and a nil error

#### Scenario: Request is SigV4 signed
- **WHEN** `BedrockProvider.Chat(ctx, req)` is called
- **THEN** the outgoing request SHALL contain an `Authorization` header starting with `AWS4-HMAC-SHA256` and an `X-Amz-Date` header

#### Scenario: Missing AWS credentials returns error
- **WHEN** `New("bedrock", Config{Region: "us-east-1"})` is called with `AWS_ACCESS_KEY_ID` unset
- **THEN** it SHALL return a non-nil error wrapping `ErrMissingCredential`

### Requirement: BedrockProvider ChatStream returns typed not-implemented error

The system SHALL implement `BedrockProvider.ChatStream` so that it returns `nil, ErrStreamingNotImplemented`.

#### Scenario: ChatStream returns typed error
- **WHEN** `BedrockProvider.ChatStream(ctx, req)` is called
- **THEN** it SHALL return `nil, ErrStreamingNotImplemented` and `errors.Is(err, ErrStreamingNotImplemented)` SHALL be true

### Requirement: All direct providers use table-driven tests

The system SHALL add `internal/providers/openai_test.go`, `internal/providers/gemini_test.go`, and `internal/providers/bedrock_test.go` using table-driven subtests. Each test SHALL use `httptest.Server` and assert wire mapping, error handling, and provider interface contract compliance.

#### Scenario: All provider tests pass
- **WHEN** running `go test ./internal/providers/...`
- **THEN** all table-driven tests for `OpenAIProvider`, `GeminiProvider`, and `BedrockProvider` SHALL pass

### Requirement: Compile-time interface checks

The system SHALL include blank identifier assignments in each provider file: `var _ outbound.Provider = (*<Name>Provider)(nil)`. This ensures the compiler enforces interface satisfaction.

#### Scenario: Build fails if interface breaks
- **WHEN** a provider method signature no longer matches `outbound.Provider`
- **THEN** `go build ./internal/providers/...` SHALL fail at the blank identifier assignment
