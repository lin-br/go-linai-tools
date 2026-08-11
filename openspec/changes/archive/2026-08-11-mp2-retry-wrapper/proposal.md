## Why

LLM APIs fail transiently — 429 rate limits, 529 overload (Anthropic-specific, proxied by OpenRouter), 5xx server errors, and network blips. Today the `Provider` interface (MP0) surfaces raw errors with no recovery strategy; a single 429 kills the request. Phase 1's roadmap lists a retry wrapper with exponential backoff + `context.WithTimeout` as a core deliverable, and the three CLIs (MP4–MP6) need resilient providers to be usable on real traffic. There is currently no way to inspect an HTTP status code on a provider error, no backoff, and no retry loop.

## What Changes

- Add a `RetryProvider` decorator that wraps any `Provider` and implements the same `Provider` interface — transparent to callers and composable with other wrappers.
- Add exponential backoff with jitter: `delay = baseDelay * 2^attempt`, capped at `maxDelay`, with full jitter `delay = rand(0, delay)` applied to the capped value to avoid thundering herd.
- Add a `RetryOptions` struct (`MaxRetries`, `BaseDelay`, `MaxDelay`, `IsRetryable`) and a functional-options constructor `NewRetryProvider(inner Provider, opts ...RetryOption) *RetryProvider`.
- Add a `ProviderError` type carrying `StatusCode int`, `Body string`, and wrapped `Err error` so the retry layer can inspect HTTP status codes without string parsing.
- Add a default `IsRetryable` predicate: retryable on 429, 529, 5xx, and network/context errors; NOT retryable on 4xx client errors (except 429/529) which fail immediately.
- Integrate `context.Context` into backoff waits — `select` on `ctx.Done()` against a `time.Timer` so cancellation/deadline aborts pending retries immediately and returns the context error.
- Wrap both `Chat` and `ChatStream`. For `ChatStream`, retry ONLY on the initial connection error (non-nil error returned alongside a nil channel). If the stream starts successfully and fails mid-way, do NOT retry — partial output has already been consumed.
- **Cross-cutting contract**: MP0's `OpenRouterProvider` MUST return `*ProviderError` for non-2xx HTTP responses so the default retry predicate can inspect the status code.

## Capabilities

### New Capabilities

- `retry-wrapper`: The `RetryProvider` decorator, exponential backoff with jitter, the `ProviderError` error type and retryable-error predicate, context-aware backoff waits, and functional-options configuration.

### Modified Capabilities

(No existing specs in `openspec/specs/` to modify — MP0 has not been archived yet. The cross-cutting requirement that `OpenRouterProvider` returns `*ProviderError` for HTTP errors is captured as a requirement inside the `retry-wrapper` spec and called out in the design, so it is enforced when both microphases are implemented together.)

## Impact

- **New files**:
  - `internal/core/ports/outbound/provider_error.go` — the `ProviderError` type (shared port contract).
  - `internal/adapters/driven/retry/retry_provider.go` — `RetryProvider`, `RetryOptions`, functional options (`WithMaxRetries`, `WithBaseDelay`, `WithMaxDelay`, `WithRetryable`), backoff calculation, default `IsRetryable`, and `Chat`/`ChatStream` wrappers.
- **MP0 dependency**: `OpenRouterProvider` (MP0) MUST return `*ProviderError` for non-2xx HTTP responses. MP0's error-handling requirement is refined by this contract; the two microphases are coordinated during implementation since neither is implemented yet.
- **Wiring**: Entry points (`main.go`, `cmd/cli/main.go`) MAY wrap `OpenRouterProvider` in `RetryProvider` at construction time once MP0 lands.
- **No new external dependencies** — `math/rand`, `time`, `context`, and `errors` from the standard library only. No `cenkalti/backoff` or retry libraries, per the roadmap constraint.
