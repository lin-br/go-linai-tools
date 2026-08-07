## Context

The repo has a hexagonal-ish layout with a single provider (`OpenRouterClient`) that mismatches its wire format (OpenAI body → Anthropic endpoint). Domain types are 2-field structs. No `context.Context` anywhere. Phase 1 needs streaming, retries, structured outputs, and three CLIs — all blocked by the lack of a proper abstraction layer.

OpenRouter's main API is the OpenAI Chat Completions format (`POST /api/v1/chat/completions`). It normalizes request/response schemas across all underlying providers. Streaming uses SSE with `data: {json}\n\n` lines terminated by `data: [DONE]`. The existing `anthropic_request.go` / `anthropic_response.go` types are complete and correct for the Anthropic Messages API but are not used by the OpenRouter adapter.

## Goals / Non-Goals

**Goals:**
- Define a provider-agnostic `Provider` interface with `Chat` and `ChatStream` methods, both taking `context.Context`.
- Define rich domain types that can express multi-turn conversations, tools, tool choices, usage, and streaming events.
- Implement `OpenRouterProvider` using the OpenAI Chat Completions wire format.
- Evolve config to support a provider selector and per-provider credential blocks.
- Propagate `context.Context` through all layers.
- Keep both existing entrypoints (`main.go`, `cmd/cli/main.go`) working end-to-end through the new abstraction.

**Non-Goals:**
- Implement streaming SSE parsing (MP1).
- Implement retry/backoff (MP2).
- Implement structured outputs / tool use parsing (MP3).
- Build any of the three CLIs (MP4–MP6).
- Implement `AnthropicProvider` or `BedrockProvider` — only leave the door open.
- Add external dependencies (SDKs, retry libraries). `net/http` + `encoding/json` only.

## Decisions

### D1: OpenAI Chat Completions as OpenRouter wire format

Use `POST /api/v1/chat/completions` with the OpenAI request/response schema.

**Why:** It is OpenRouter's main API. It normalizes across all underlying providers (Anthropic, OpenAI, Google, etc.), so the response shape is always `choices[].message` regardless of which model is called. SSE streaming is simple: `data: {chunk}\n\n` lines, each with `choices[].delta.content`, terminated by `data: [DONE]`.

**Alternative considered:** Anthropic Messages format (`/api/v1/messages`) — would reuse the existing `anthropic_*.go` types and give Bedrock format compatibility for free. Rejected because the user explicitly chose the main API, and the OpenAI format is more universally normalized across OpenRouter's provider pool.

### D2: Domain model is provider-agnostic

Domain types are neither OpenAI-shaped nor Anthropic-shaped. Each provider adapter translates to/from its wire format.

```
ChatRequest ──▶ OpenRouterProvider.toWire() ──▶ ChatCompletionRequest
                                                        │
                                          POST /v1/chat/completions
                                                        ▼
ChatResponse ◀── OpenRouterProvider.fromWire() ◀── ChatCompletionResponse
```

**Key types:**
- `Message{Role, Content, ToolCalls, ToolCallID}` — supports multi-turn, tool results.
- `ChatRequest{Model, Messages, System, Tools, ToolChoice, MaxTokens, Temperature, TopP, Stream}` — full request surface.
- `ChatResponse{Content, ToolCalls, StopReason, Usage, Model}` — full response surface.
- `StreamEvent{Type, Delta, StopReason, Usage, Err}` — streaming events via channel.
- `Tool{Name, Description, InputSchema}` — JSON Schema for tool input.
- `ToolChoice{Type, Name}` — auto/none/forced.
- `ToolCall{ID, Name, Arguments}` — Arguments is a JSON string (provider-agnostic).
- `Usage{InputTokens, OutputTokens, TotalTokens, Cost}` — cost optional.

### D3: Provider interface — channel-based streaming

```go
type Provider interface {
    Chat(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error)
    ChatStream(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamEvent, error)
}
```

**Why channel over iterator:** Channels are the idiomatic Go pattern for streaming. They integrate naturally with `context.Context` cancellation (close on ctx.Done), `select` statements, and fan-out. The provider closes the channel when the stream is complete or on error (error sent as final `StreamEvent` before closing).

**ChatStream contract:** The provider returns a channel immediately (non-blocking). The caller reads events until the channel is closed. If the initial HTTP request fails, `ChatStream` returns an error immediately (no channel). If the stream fails mid-way, an error `StreamEvent` is sent, then the channel is closed.

### D4: Config with provider selector

```yaml
provider: openrouter          # "openrouter" | "anthropic" | "bedrock"
openrouter_api_key: ${OPENROUTER_API_KEY}
# anthropic_api_key: ${ANTHROPIC_API_KEY}    # future
# bedrock_region: ${BEDROCK_REGION}          # future
models:
  default: ${DEFAULT_MODEL}
  pro: ${PRO_MODEL}
  free: ${FREE_MODEL}
```

The `provider` field selects which adapter to construct at wiring time. Default is `openrouter`. Unknown values produce a clear error. This is forward-compatible — adding Anthropic/Bedrock later requires only a new adapter + config field, no schema change.

### D5: Context propagation

`context.Context` is the first parameter of every method in the call chain:
- `Provider.Chat(ctx, req)` / `Provider.ChatStream(ctx, req)`
- `DoSendMessageUseCase.Send(ctx, message)`
- `Entrypoint.StartAgent(ctx, in, out)`

**Why:** Enables MP1 (stream cancellation), MP2 (timeout via `context.WithTimeout`), and is idiomatic Go. The HTTP request in `OpenRouterProvider` uses `http.NewRequestWithContext(ctx, ...)`, so context cancellation aborts in-flight requests.

### D6: Preserve existing Anthropic types

`anthropic_request.go` and `anthropic_response.go` stay in the `http_clients` package untouched. They are complete and correct for a future `AnthropicProvider`. When that provider is built (post-Phase 1), they move to a dedicated `anthropic` adapter package. No premature refactoring.

## Risks / Trade-offs

- **[Breaking change to domain types]** → All existing code using `domain.Request`/`domain.Response` must be updated. Since the codebase is small (2 entrypoints, 1 use case, 1 adapter), the blast radius is contained.
- **[ToolCall.Arguments is a JSON string, not parsed]** → Keeps the domain provider-agnostic (OpenAI sends string, Anthropic sends object). MP3 will add a generic `Extract[T]` helper that parses the string into a typed struct. Trade-off: callers must parse, but the interface is clean.
- **[No tool call streaming in StreamEvent]** → Phase 1 CLIs that use tool use (`extract`, `spec-to-code`) use non-streaming `Chat`. The `summarize` CLI uses streaming but only needs text deltas. Tool call streaming can be added later if needed. Trade-off: simpler StreamEvent now.
- **[Config path is hardcoded]** → `configs.yaml` path is `./internal/configs/configs.yaml`. Not addressed here — it works for the learning project. Could be made configurable via flag/env in the future.
