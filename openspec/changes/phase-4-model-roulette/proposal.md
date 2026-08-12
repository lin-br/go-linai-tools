## Why

Phase 1 taught the wire protocol through a single OpenRouter client; Phase 4 turns that into provider-agnostic intuition. The roadmap requires benchmarking the same prompts across multiple providers to learn latency, cost, and quality trade-offs. Today the monorepo only has `OpenRouterProvider`; there is no generic factory, no direct provider implementations, and no benchmark harness. Phase 4 closes that gap by adding a provider registry and a CLI that runs ten prompt categories head-to-head, producing a cost/quality CSV and a permanent model-selection cheat sheet.

## What Changes

- Add `internal/providers/` as the new provider layer: a `Provider` interface wrapper around the existing `outbound.Provider`, provider-specific configuration, and a factory `New(kind string)`.
- Add `AnthropicProvider` (direct Messages API over `net/http`), `OpenAIProvider` (direct Completions API), `GeminiProvider` (Google AI Studio generateContent), and `BedrockProvider` (AWS Bedrock InvokeModel / Converse, `us-east-1` default). All satisfy `outbound.Provider`.
- Add `cmd/model-roulette/` benchmark CLI with ten fixed prompt categories: classification, summarization, structured extraction, creative, code, reasoning, translation, RAG response, tool selection, and refusal boundary.
- Emit CSV with columns `prompt_id, model, latency_ms, prompt_tokens, completion_tokens, cost_usd, quality_1_to_5`.
- Add `docs/model-selection.md` — the permanent model-picker cheat sheet refreshed every three months.
- No breaking changes. `OpenRouterProvider` remains unchanged and continues to live in `internal/adapters/driven/http_clients/`.

## Capabilities

### New Capabilities

- `mp19-multi-provider`: Generic provider factory + configuration + `AnthropicProvider` (direct Messages API, `net/http`).
- `mp20-provider-implementations`: `OpenAIProvider`, `GeminiProvider`, and `BedrockProvider`, all wrapping direct HTTP calls and satisfying `outbound.Provider`.
- `mp21-benchmark-runner`: `cmd/model-roulette` benchmark CLI with ten prompt categories, model list via flag/config, and per-prompt timed execution.
- `mp22-csv-cheat-sheet`: CSV result output and `docs/model-selection.md` model-selection guide.

### Modified Capabilities

- (No existing specs are modified. The existing `OpenRouterProvider` and `outbound.Provider` interface are consumed unchanged.)

## Impact

- **New files:**
  - `internal/providers/provider.go` — provider abstraction and factory.
  - `internal/providers/config.go` — provider-specific config parsing.
  - `internal/providers/anthropic.go` — `AnthropicProvider`.
  - `internal/providers/openai.go` — `OpenAIProvider`.
  - `internal/providers/gemini.go` — `GeminiProvider`.
  - `internal/providers/bedrock.go` — `BedrockProvider`.
  - `internal/providers/anthropic_request.go` / `anthropic_response.go` — domain-specific wire types for Anthropic.
  - `internal/providers/openai_wire.go` / `openai_response.go` — OpenAI wire types.
  - `internal/providers/gemini_wire.go` — Gemini wire types.
  - `cmd/model-roulette/main.go` — benchmark CLI entry point.
  - `cmd/model-roulette/runner.go` — benchmark orchestration.
  - `cmd/model-roulette/prompts.go` — the ten prompt definitions.
  - `docs/model-selection.md` — cheat sheet.
- **No new external SDKs** — all providers use `net/http` + `encoding/json`. AWS request signing for Bedrock uses the standard library plus `crypto/hmac`/`sha256`.
- **No breaking changes** — purely additive.
- **Enables** Phase 4 close: interface + three impls + CSV + concept page + LinkedIn post #4.
