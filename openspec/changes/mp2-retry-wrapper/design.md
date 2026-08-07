## Context

MP0 defines the `Provider` interface (`Chat`, `ChatStream` with `context.Context`) and `OpenRouterProvider` using the OpenAI Chat Completions wire format. MP0's spec requires that non-2xx responses produce an error carrying the status code and body, but does not yet define a typed error — it returns a generic error string. There is no retry, no backoff, and no way for a caller to programmatically distinguish a retryable 429 from a permanent 400 without parsing strings.

OpenRouter proxies many upstream providers (Anthropic, OpenAI, Google). Transient failures are common: 429 rate limits, 529 Anthropic overload, 5xx upstream errors, and network-level timeouts/resets. A single transient failure should not kill a request when a short backoff would succeed. Phase 1's roadmap explicitly calls for "Retry wrapper with exponential backoff + `context.WithTimeout` (handles 429/529/400)" as deliverable #4.

The project constraint is strict: `net/http` + `encoding/json` + stdlib only. No `cenkalti/backoff`, no retry libraries. This is a learning project — hand-rolling the retry loop is the point.

## Goals / Non-Goals

**Goals:**
- Implement a `RetryProvider` decorator that wraps any `Provider` and implements the same `Provider` interface (transparent to callers, composable).
- Exponential backoff with jitter that respects `context.Context` cancellation/deadline during waits.
- Define a `ProviderError` type carrying the HTTP status code so retryability is decided by type assertion, not string parsing.
- A default `IsRetryable` predicate covering 429 / 529 / 5xx / network errors, with the ability to override it.
- Wrap both `Chat` and `ChatStream`, with `ChatStream` retrying only on the initial connection error.
- Configurable via functional options (`MaxRetries`, `BaseDelay`, `MaxDelay`, `IsRetryable`).

**Non-Goals:**
- Retry on mid-stream failures (partial output already consumed — caller owns the stream).
- Circuit breaking, metrics, or observability hooks (Phase 3+ concern).
- Retry-After header parsing (noted as an open question; default backoff is sufficient for Phase 1).
- Wrapping non-OpenRouter providers — the decorator is generic, but only OpenRouter exists in Phase 1.
- Changing the `Provider` interface signature — the decorator is transparent.

## Decisions

### D1: Decorator pattern over middleware functions

`RetryProvider{inner Provider, opts RetryOptions}` implements `Provider`. Callers construct `NewRetryProvider(openRouterProvider, opts...)` and use it wherever a `Provider` is expected.

**Why over middleware funcs:** The `Provider` interface has two methods. A struct that satisfies the interface keeps the wiring site unchanged — `useCase.Send` receives a `Provider` and does not know whether retries are enabled. It also composes: a future logging/metrics wrapper can wrap `RetryProvider` the same way.

**Alternative considered:** A `WithRetry(provider, opts)` free function returning `Provider`. Functionally identical, but a named type (`*RetryProvider`) is easier to type-assert in tests and debugging. Chose the struct.

### D2: ProviderError lives in the outbound ports package

`ProviderError{StatusCode int, Body string, Err error}` is defined in `internal/core/ports/outbound/provider_error.go`. It implements `error` (via `Error() string`) and (optionally) `Unwrap() error`.

**Why the ports package and not the retry adapter:** `ProviderError` is a contract between providers and their consumers. Both `OpenRouterProvider` (MP0) and `RetryProvider` (MP2) reference it. Putting it in the retry package would create a dependency from the provider adapter → retry adapter, which is backwards. The port package is the natural shared home.

**Cross-cutting contract:** MP0's `OpenRouterProvider` MUST return `*ProviderError` for non-2xx HTTP responses. MP0 is specced but not implemented, so this is coordinated: when MP0 is implemented, its error-handling code returns `&ProviderError{StatusCode: resp.StatusCode, Body: body}`. The retry predicate type-asserts `errors.As(err, &providerErr)` to read `StatusCode`. Network errors (no HTTP response) are NOT `*ProviderError` — they are retryable by default (see D4).

### D3: Exponential backoff with full jitter

```
capped   = min(baseDelay * 2^attempt, maxDelay)
delay    = rand.Int63n(capped + 1)   // full jitter: [0, capped]
```

Attempt is zero-indexed: after the 1st failure, `attempt=0` gives a delay in `[0, baseDelay]`; after the 2nd, `[0, 2*baseDelay]`; etc., capped at `maxDelay`.

**Why full jitter over "equal jitter" (`delay/2 + rand(0, delay/2)`):** Full jitter (`rand(0, capped)`) gives the best thundering-herd protection under high concurrency — AWS's classic exponential-backoff paper recommends it. Equal jitter is a middle ground that preserves some determinism; for a learning project the simplest effective formula is full jitter. `math/rand` is sufficient; no crypto-grade randomness needed for backoff spread.

**Default values:** `MaxRetries=3`, `BaseDelay=1s`, `MaxDelay=30s`. These are sensible for LLM APIs (429s usually clear in a few seconds; 30s cap prevents pathological waits).

### D4: Retryable predicate — status-code classification

The default `IsRetryable(err error) bool`:
1. If `errors.As(err, &providerErr)` succeeds → check `providerErr.StatusCode`:
   - 429 → retryable (rate limit)
   - 529 → retryable (Anthropic overload, proxied by OpenRouter)
   - 5xx (500–599) → retryable (server error)
   - other 4xx (400, 401, 403, 404, 422) → NOT retryable (client error, retrying won't help)
2. If NOT a `*ProviderError` → retryable (assume network error: timeout, connection reset, DNS). Network failures are overwhelmingly transient; retrying is the safe default.
3. If `errors.Is(err, context.Canceled)` or `errors.Is(err, context.DeadlineExceeded)` → NOT retryable. Context cancellation is intentional; do not fight it.

**Why default-retryable for non-ProviderError:** The only non-`ProviderError` failures from `OpenRouterProvider` are transport-level (`net/http` returns `*url.Error` on timeout, connection refused, etc.). These are transient. If a future provider returns a non-retryable non-HTTP error, it should wrap it in `*ProviderError` with a 4xx code, or the caller supplies a custom `IsRetryable`.

**Override:** `WithRetryable(func(error) bool)` replaces the default. This lets a caller mark 400 as retryable (rare) or disable retries entirely (`func(error) bool { return false }`).

### D5: ChatStream retries only on the initial connection error

`ChatStream` returns `(<-chan StreamEvent, error)`. Per MP0's contract:
- If the initial HTTP request fails → returns `(nil, err)`.
- If the stream starts → returns `(ch, nil)`; subsequent failures arrive as `StreamEvent{Type: Error}` on the channel.

`RetryProvider.ChatStream` retries ONLY in the first case (non-nil error, nil channel). Once a channel is returned, the stream is live — partial output may have been consumed by the caller, and replaying the request would duplicate tokens. Mid-stream failures are the caller's problem.

**Why not replay the stream:** There is no way to know how much the caller consumed. Replaying from the start produces duplicate content. The caller can choose to re-`ChatStream` manually if it hasn't committed any output.

### D6: Context-aware backoff via timer + select

```go
timer := time.NewTimer(delay)
select {
case <-timer.C:
    // proceed to next attempt
case <-ctx.Done():
    timer.Stop()
    return ctx.Err()   // or the last provider error — see below
}
```

**Which error to return on context cancellation mid-backoff:** Return the last provider error, not `ctx.Err()`, when the context is cancelled DURING a backoff wait but the cancellation was a deadline (not an explicit cancel). Rationale: the caller wants to know WHY the operation failed (e.g., "429 rate limit" is more useful than "context deadline exceeded" when the deadline was caused by the retries themselves). However, if the context was explicitly cancelled (`context.Canceled`), return `ctx.Err()` because the caller chose to abort.

Simplified rule for Phase 1: on `ctx.Done()` during backoff, return the last provider error if non-nil, else `ctx.Err()`. This keeps it predictable. (Refinement: distinguish `Canceled` vs `DeadlineExceeded` — defer to Phase 3 if it matters.)

### D7: Functional options for construction

```go
type RetryOption func(*RetryOptions)

func NewRetryProvider(inner Provider, opts ...RetryOption) *RetryProvider
```

Options: `WithMaxRetries(int)`, `WithBaseDelay(time.Duration)`, `WithMaxDelay(time.Duration)`, `WithRetryable(func(error) bool)`.

**Why functional options over a config struct constructor:** The roadmap project uses functional options nowhere yet, but this is the idiomatic Go pattern for extensible constructors (3+ optional params, future-proof). It also lets callers write `NewRetryProvider(p)` for pure defaults with zero ceremony. Aligns with the `golang-functional-options` skill convention.

**Zero-value safety:** `NewRetryProvider` applies defaults after user options, so a caller passing no options gets `MaxRetries=3, BaseDelay=1s, MaxDelay=30s, IsRetryable=default`. This means the zero-value `RetryOptions` is never used directly — the constructor fills it.

## Risks / Trade-offs

- **[ProviderError is a cross-cutting contract with MP0]** → MP0 is specced but not implemented. If MP0 is implemented without returning `*ProviderError`, the default predicate cannot classify HTTP errors and falls through to "retryable" for all errors — 400s get retried wastefully. Mitigation: the `retry-wrapper` spec includes a requirement that providers return `*ProviderError` for HTTP errors; coordinate implementation of MP0 + MP2 together.
- **[No Retry-After header parsing]** → OpenRouter sends `Retry-After` on 429s. Ignoring it means backoff may be too short (retry before the window clears) or too long (cap higher than needed). Mitigation: Phase 1 default backoff is sufficient for learning; parse `Retry-After` in a later microphase if real traffic warrants. Noted as an open question.
- **[Full jitter can produce near-zero delays]** → `rand(0, capped)` occasionally returns ~0, causing a tight retry. Mitigation: this is intentional under high concurrency (spreads load); for a single-user CLI it's negligible. A floor could be added later.
- **[Mid-stream failures are not retried]** → A stream that starts then drops loses all progress. Mitigation: documented behavior; the caller can re-invoke `ChatStream`. Phase 3's agent loop may add stream-resume logic.
- **[math/rand is not seeded by default in older Go]** → Go 1.20+ auto-seeds the global rand source; the project uses Go 1.26, so no `rand.Seed` needed. If a local `rand.Rand` is used for testability, document it.

## Open Questions

- **Retry-After header:** Should the default predicate read `Retry-After` from 429 responses and use it as the delay floor? Deferred — Phase 1 uses fixed exponential backoff. If added, `ProviderError` would need a `Headers` field or a dedicated `RetryAfter` field.
- **Error returned on context cancellation:** Return last provider error vs `ctx.Err()`. Phase 1 rule: last provider error if non-nil, else `ctx.Err()`. Refine in Phase 3 if observability needs the distinction.
- **RetryProvider location:** `internal/adapters/driven/retry/` vs `internal/core/ports/outbound/` (as a port-level wrapper). Chose the adapter dir because it's a concrete implementation, not a port contract. Open to revisiting if other wrappers cluster.
