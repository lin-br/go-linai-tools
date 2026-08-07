## 1. Domain Types

- [ ] 1.1 Create `internal/core/domain/message.go` — `Message{Role, Content, ToolCalls, ToolCallID}` with role constants
- [ ] 1.2 Create `internal/core/domain/tool.go` — `Tool{Name, Description, InputSchema}`, `ToolChoice{Type, Name}`, `ToolCall{ID, Name, Arguments}`
- [ ] 1.3 Create `internal/core/domain/usage.go` — `Usage{InputTokens, OutputTokens, TotalTokens, Cost}`
- [ ] 1.4 Create `internal/core/domain/chat_request.go` — `ChatRequest{Model, Messages, System, Tools, ToolChoice, MaxTokens, Temperature, TopP, Stream}`
- [ ] 1.5 Create `internal/core/domain/chat_response.go` — `ChatResponse{Content, ToolCalls, StopReason, Usage, Model}`
- [ ] 1.6 Create `internal/core/domain/stream_event.go` — `StreamEvent{Type, Delta, StopReason, Usage, Err}` with `StreamEventType` constants
- [ ] 1.7 Remove old `domain.Request` / `domain.Response` types (replace in all usages)

## 2. Ports

- [ ] 2.1 Replace `outbound.ProviderModelHandler` with `Provider{Chat(ctx, *ChatRequest) (*ChatResponse, error); ChatStream(ctx, *ChatRequest) (<-chan StreamEvent, error)}` in `internal/core/ports/outbound/provider.go`
- [ ] 2.2 Update `inbound.Entrypoint` to `StartAgent(ctx context.Context, in io.Reader, out io.Writer)` in `internal/core/ports/inbound/entrypoint.go`

## 3. OpenRouter Wire Types (OpenAI Chat Completions format)

- [ ] 3.1 Create `internal/adapters/driven/http_clients/openai_wire.go` — `ChatCompletionRequest`, `WireMessage`, `WireTool`, `WireFunction`, `WireToolCall`, `WireFuncCall`
- [ ] 3.2 Add response types to `openai_wire.go` — `ChatCompletionResponse`, `WireChoice`, `WireUsage`
- [ ] 3.3 Add streaming chunk types to `openai_wire.go` — `ChatCompletionChunk`, `WireChunkChoice`, `WireDelta`

## 4. OpenRouterProvider

- [ ] 4.1 Rewrite `internal/adapters/driven/http_clients/openrouter.go` — rename `OpenRouterClient` to `OpenRouterProvider`, implement `Provider` interface
- [ ] 4.2 Implement `toWire(*domain.ChatRequest) *ChatCompletionRequest` — domain-to-wire translation (messages, system prompt, tools, tool_choice, params)
- [ ] 4.3 Implement `fromWire(*ChatCompletionResponse) *domain.ChatResponse` — wire-to-domain translation (content, tool_calls, stop_reason, usage, cost)
- [ ] 4.4 Implement `Chat(ctx, *domain.ChatRequest) (*domain.ChatResponse, error)` — build request with `http.NewRequestWithContext`, POST to `/api/v1/chat/completions`, parse response
- [ ] 4.5 Implement `ChatStream(ctx, *domain.ChatRequest) (<-chan StreamEvent, error)` — stub that returns `errors.New("streaming not implemented yet")` (wired in MP1)
- [ ] 4.6 Add error handling for non-2xx responses — return error with status code and response body
- [ ] 4.7 Remove the old `makePayload` / `makeRequest` methods and the hardcoded 5-minute timeout

## 5. Config

- [ ] 5.1 Add `Provider string` field to `Config` struct with YAML tag `provider`
- [ ] 5.2 Update `configs.yaml` — add `provider: openrouter` (default), keep existing fields
- [ ] 5.3 Add validation in `LoadConfigs` — error on unknown provider value; require `openrouter_api_key` when provider is `openrouter`

## 6. Use Case

- [ ] 6.1 Refactor `DoSendMessageUseCase.Send` to accept `ctx context.Context` as first parameter and use the new `Provider` interface
- [ ] 6.2 Update `Send` to build a `*domain.ChatRequest` (with `Model`, `Messages` from the input message) and call `provider.Chat(ctx, req)`

## 7. Driving Adapter

- [ ] 7.1 Update `cli.go` `StartAgent` to accept `context.Context` and propagate it to `useCase.Send(ctx, text)`

## 8. Entrypoints

- [ ] 8.1 Update `main.go` — wire `context.Background()`, new `OpenRouterProvider`, updated use case
- [ ] 8.2 Update `cmd/cli/main.go` — wire `context.Background()`, new `OpenRouterProvider`, updated use case
- [ ] 8.3 Add provider selection in wiring — construct `OpenRouterProvider` when `config.Provider == "openrouter"`, error otherwise

## 9. Verification

- [ ] 9.1 Run `go build ./...` — all packages compile
- [ ] 9.2 Run `go vet ./...` — no warnings
- [ ] 9.3 Manually test `go run .` — one-shot prompt works end-to-end through new abstraction
- [ ] 9.4 Manually test `go run ./cmd/cli` — interactive loop works through new abstraction
