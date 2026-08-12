## Context

By the end of Phase 1 the monorepo has a clean `outbound.Provider` interface (`Chat`, `ChatStream`) and a working `OpenRouterProvider` that speaks the OpenAI-compatible wire format. Phase 4 uses that foundation to develop model-picking intuition: the same `domain.ChatRequest` must run against multiple providers, measured with the same metrics, without changing application code.

The existing `anthropic_request.go` / `anthropic_response.go` types were preserved for this exact moment. They now become the wire format for a direct `AnthropicProvider`. OpenAI, Gemini, and Bedrock each get dedicated wire files under `internal/providers/` so that every provider is self-contained and independently testable. A thin `Provider` wrapper and `New(kind string)` factory live at the package root so callers only need a provider name string.

The benchmark CLI is a new `cmd/model-roulette/` binary. It is intentionally not an agent loop; it is a batch runner that iterates prompts and models, measures wall-clock latency, reads token usage, estimates cost from a static price table, and writes one CSV row per run.

## Goals / Non-Goals

**Goals:**
- Provide a generic provider factory in `internal/providers/` that returns implementations of `outbound.Provider` by name.
- Implement `AnthropicProvider` using the Anthropic Messages API over `net/http`.
- Implement `OpenAIProvider`, `GeminiProvider`, and `BedrockProvider` over `net/http`.
- Preserve the existing `outbound.Provider` interface contract: `Chat(ctx, *ChatRequest) (*ChatResponse, error)` and `ChatStream(ctx, *ChatRequest) (<-chan StreamEvent, error)`.
- Build `cmd/model-roulette` with ten fixed prompt categories and a CSV output.
- Write `docs/model-selection.md` as a living model-picker cheat sheet.
- Keep all providers testable behind the shared interface using fakes and `httptest.Server`.

**Non-Goals:**
- Replacing `OpenRouterProvider` or moving it into `internal/providers/`.
- Adding streaming for all providers in MP20. Each provider MUST implement `Chat`; `ChatStream` returns a clear error (`ErrStreamingNotImplemented`) for direct providers that do not yet support streaming. OpenRouter continues to support streaming.
- Full agent-loop integration — Phase 3 already provides that; Phase 4 only benchmarks single-turn prompts.
- Dynamic pricing via live API. Cost is estimated from a static per-model price table in Go.
- UI — the benchmark output is CSV + markdown only.
- Bedrock streaming, tool calling, or guardrails. MP20 covers text `Chat` only.

## Decisions

### D1: Provider package location — `internal/providers/` separate from adapters

The new provider implementations live in `internal/providers/`, not under `internal/adapters/driven/http_clients/`. The existing `OpenRouterProvider` remains in `http_clients` as-is.

**Why:** `internal/providers/` is a sibling domain package focused on provider selection. The hexagonal adapter layer already owns the OpenRouter-specific OpenAI-compatible client; adding direct-provider code there would mix generic factory logic with adapter code. Keeping `OpenRouterProvider` where it is preserves backwards compatibility and makes the factory a thin router that can delegate to either the adapter or a direct provider.

**Alternative considered:** Move `OpenRouterProvider` into `internal/providers/` and unify everything. Rejected — it touches tested Phase 1 code for no functional gain and creates unnecessary churn.

### D2: All direct providers satisfy the existing `outbound.Provider` interface

`AnthropicProvider`, `OpenAIProvider`, `GeminiProvider`, and `BedrockProvider` each implement `Chat` and `ChatStream`.

**Why:** The benchmark and any future use case can switch models by changing a string. No per-provider request/response types leak into the benchmark code.

**Trade-off accepted:** The shared domain model is OpenAI-ish (`system`, `messages`, `tools`, `tool_choice`). Each provider adapter maps that into its native wire shape. Anthropic needs a `system` array and assistant/user roles; Gemini needs a `contents` array; Bedrock needs `messages` with inference parameters. The translation code is localized to each provider file.

### D3: Streaming is optional at the provider level

`ChatStream` is required by the interface but direct providers in Phase 4 are not required to stream. They return `(<-chan StreamEvent, error)` where the error is a typed `ErrStreamingNotImplemented`. OpenRouter already streams and remains unchanged.

**Why:** Phase 4 benchmarks use `Chat` (non-streaming) so that usage and cost are reported in one response. Forcing every provider to implement SSE reassembly in MP20 would delay the benchmark without adding learning value. Returning a typed error keeps the interface honest and lets callers fall back to `Chat`.

### D4: Cost estimation from a static price table

The benchmark CLI holds a `map[string]ModelPricing` keyed by `model` string with `{InputPer1M, OutputPer1M float64}`. Cost = `(prompt_tokens * InputPer1M + completion_tokens * OutputPer1M) / 1e6`.

**Why:** Live pricing APIs add credentials, rate limits, and network calls to a benchmark that should be deterministic and offline-friendly. A static table is sufficient for Phase 4 and is trivial to update. Prices are USD per million tokens.

**Alternative considered:** Query OpenRouter `/api/v1/models` or provider pricing endpoints at runtime. Rejected — adds external dependency to a metric that only needs approximate correctness for model comparison.

### D5: Quality score is a manual 1–5 rating filled after the run

The CSV column `quality_1_to_5` is written as empty (`""`) by the benchmark runner. The user inspects outputs and fills the value in a spreadsheet or via a future helper.

**Why:** Automated LLM-as-judge would require another model call, cost, and judge design. Phase 4 is about developing human model-picking intuition. The CSV schema reserves the column; the value is intentionally human-rated.

### D6: Ten fixed prompt categories

Prompts are hard-coded in `cmd/model-roulette/prompts.go` as a `[]BenchmarkPrompt` slice. Categories match the roadmap: classification, summarization, structured extraction, creative, code, reasoning, translation, RAG response, tool selection, refusal boundary.

**Why:** Fixed prompts make runs comparable across models and users. The categories cover the most common production LLM tasks and map directly to the model-selection cheat sheet.

### D7: No external CLI framework

`cmd/model-roulette` uses the standard library `flag` package, consistent with existing CLIs.

**Why:** The roadmap explicitly avoids heavy frameworks in learning code. The CLI has a handful of flags; `flag` is enough.

### D8: Bedrock authentication via environment credentials

`BedrockProvider` reads `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN` (optional), and `AWS_REGION` from the environment. It builds the SigV4 signature using `crypto/hmac`, `crypto/sha256`, and the standard library.

**Why:** AWS SDK would hide the wire protocol, which is the Phase 4 learning goal. The SigV4 implementation is ~80 lines and demonstrates how Bedrock requests are signed. Environment credentials are the standard AWS convention.

## Risks / Trade-offs

- **[Direct providers may diverge from the shared domain model]** → Mitigation: unit tests in each provider package use `httptest.Server` to assert wire-format mapping. Fakes for `outbound.Provider` keep the benchmark tests isolated from network calls.
- **[Static pricing becomes stale]** → Mitigation: `docs/model-selection.md` and the price table are marked "refresh every 3 months"; the CSV still records exact token counts, so cost can be recomputed later.
- **[Bedrock SigV4 signing is error-prone]** → Mitigation: add an integration test behind a `//go:build integration` tag that calls a real Bedrock model only when credentials are present. Unit tests sign a known request and assert the canonical signature matches.
- **[Gemini API changes field names frequently]** → Mitigation: keep wire structs minimal (`GenerateContentRequest`, `GenerateContentResponse`) and tests assert unmarshalling of the current documented shape.
- **[Quality rating is manual and subjective]** → Mitigation: documented as a non-goal; the CSV reserves the column and the cheat sheet explains the rubric.
