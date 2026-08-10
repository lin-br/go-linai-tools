## Why

The current `OpenRouterClient` posts an OpenAI-shaped payload to the Anthropic Messages endpoint (`/api/v1/messages`) — a wire-format mismatch flagged in AGENTS.md. The domain types (`Request{Model, Message}`, `Response{Message}`) are too thin to express multi-turn conversations, tools, streaming, or usage. There is no `context.Context` propagation, no provider-agnostic abstraction, and the outbound port (`ProviderModelHandler`) is coupled to a single provider. Phase 1 needs streaming, retries, structured outputs, and three CLIs — none of which can land on the current foundation.

## What Changes

- **BREAKING**: Replace `domain.Request{Model, Message}` / `domain.Response{Message}` with richer, provider-agnostic types: `ChatRequest`, `ChatResponse`, `StreamEvent`, `Message`, `Tool`, `ToolChoice`, `Usage`.
- **BREAKING**: Replace `outbound.ProviderModelHandler` interface with a new `Provider` interface containing `Chat(ctx, *ChatRequest) (*ChatResponse, error)` and `ChatStream(ctx, *ChatRequest) (<-chan StreamEvent, error)`.
- **BREAKING**: Replace `inbound.Entrypoint` interface to accept `context.Context`.
- Refactor `OpenRouterClient` → `OpenRouterProvider` using the OpenAI Chat Completions format (`POST /api/v1/chat/completions`), the main OpenRouter API.
- Add OpenAI-compatible wire types (`ChatCompletionRequest`, `ChatCompletionResponse`, `ChatCompletionChunk`) in the OpenRouter adapter package.
- Evolve `configs.Config` to support a `provider` field (`openrouter` | `anthropic` | `bedrock`) and per-provider configuration blocks. Keep OpenRouter as default.
- Keep existing `anthropic_request.go` / `anthropic_response.go` types in place for future `AnthropicProvider` — they are not used by the OpenRouter adapter.
- Update `DoSendMessageUseCase` to use `context.Context` and the new `Provider` interface.
- Update both entrypoints (`main.go`, `cmd/cli/main.go`) to wire through the new types.
- Propagate `context.Context` through all layers (use cases, adapters, ports).

## Capabilities

### New Capabilities

- `provider-interface`: The `Provider` interface contract — `Chat` and `ChatStream` methods with `context.Context`, the driving/driven port boundary that all providers implement.
- `domain-model`: Provider-agnostic domain types for chat requests, responses, streaming events, messages, tools, tool choices, and usage.
- `openrouter-provider`: OpenRouter adapter implementation using the OpenAI Chat Completions wire format, including request building, response parsing, error handling, and SSE chunk parsing infrastructure (actual streaming client wired in MP1).
- `config-providers`: Configuration structure supporting multiple AI providers with a selector field and per-provider credential blocks.

### Modified Capabilities

(No existing specs to modify — this is the first OpenSpec change in the repo.)

## Impact

- **`internal/core/domain/`** — `message.go` replaced with richer types (new files: `chat_request.go`, `chat_response.go`, `stream_event.go`, `message.go`, `tool.go`, `usage.go`).
- **`internal/core/ports/outbound/`** — `ProviderModelHandler` replaced with `Provider` interface.
- **`internal/core/ports/inbound/`** — `Entrypoint` updated to accept `context.Context`.
- **`internal/core/usecases/`** — `DoSendMessageUseCase` refactored to use `Provider` interface and `context.Context`.
- **`internal/adapters/driven/http_clients/`** — `openrouter.go` refactored to `OpenRouterProvider`; new OpenAI wire types added; existing `anthropic_*.go` files untouched.
- **`internal/adapters/driving/`** — `cli.go` updated for context-aware use case.
- **`internal/configs/`** — `configs.go` and `configs.yaml` extended with provider selection.
- **`main.go`, `cmd/cli/main.go`** — wiring updated.
- **No new external dependencies** — still `net/http` + `encoding/json` only.
