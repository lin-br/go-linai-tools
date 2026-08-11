## 1. ProviderError Type (Port Contract)

- [x] 1.1 Create `internal/core/ports/outbound/provider_error.go` — `ProviderError{StatusCode int, Body string, Err error}` with `Error() string` (includes status code + body) and `Unwrap() error` returning `Err`
- [x] 1.2 Verify `errors.As(err, &providerErr)` and `errors.Is` against the wrapped `Err` work as expected (compile-time interface check: `var _ error = (*ProviderError)(nil)`)

## 2. RetryOptions and Functional Options

- [x] 2.1 Create `internal/adapters/driven/retry/retry_provider.go` — `RetryOptions{MaxRetries int, BaseDelay time.Duration, MaxDelay time.Duration, IsRetryable func(error) bool}` struct
- [x] 2.2 Define `RetryOption` type alias `func(*RetryOptions)` and the `NewRetryProvider(inner Provider, opts ...RetryOption) *RetryProvider` constructor
- [x] 2.3 Implement functional option helpers: `WithMaxRetries(int)`, `WithBaseDelay(time.Duration)`, `WithMaxDelay(time.Duration)`, `WithRetryable(func(error) bool)`
- [x] 2.4 Apply defaults in `NewRetryProvider` after user options: `MaxRetries=3`, `BaseDelay=1s`, `MaxDelay=30s`, `IsRetryable=defaultIsRetryable` (zero-value `IsRetryable` nil is overwritten with the default)

## 3. Default Retryable Predicate

- [x] 3.1 Implement `defaultIsRetryable(err error) bool` — `errors.As` into `*ProviderError`; if found, retryable on 429/529/5xx (500–599), not retryable on other 4xx
- [x] 3.2 Handle non-`*ProviderError` errors — return true (assume network/transport transient), unless `errors.Is(err, context.Canceled)` or `errors.Is(err, context.DeadlineExceeded)` → return false

## 4. Exponential Backoff with Jitter

- [x] 4.1 Implement `backoffDelay(attempt int, base, max time.Duration) time.Duration` — `capped = min(base * 2^attempt, max)`, then full jitter `delay = rand(0, capped)` via `math/rand`
- [x] 4.2 Ensure the delay never exceeds `max` and handles `capped=0` safely (no `rand.Int63n(0)` panic)

## 5. Context-Aware Backoff Wait

- [x] 5.1 Implement `wait(ctx context.Context, delay time.Duration) error` — `time.NewTimer(delay)` + `select` on `timer.C` vs `ctx.Done()`; stop timer on ctx cancellation
- [x] 5.2 Return `nil` when the timer fires (proceed to next attempt); return the appropriate error when `ctx.Done()` fires (caller decides what to surface)

## 6. RetryProvider.Chat Wrapper

- [x] 6.1 Implement `RetryProvider.Chat(ctx, req)` — loop over attempts (0..MaxRetries): call `inner.Chat`, return immediately on success or non-retryable error
- [x] 6.2 On a retryable error, call `wait(ctx, backoffDelay(attempt, opts.BaseDelay, opts.MaxDelay))`; if `wait` returns a context error, abort and return the last provider error if non-nil, else `ctx.Err()`
- [x] 6.3 After exhausting `MaxRetries`, return the last error without further backoff

## 7. RetryProvider.ChatStream Wrapper

- [x] 7.1 Implement `RetryProvider.ChatStream(ctx, req)` — loop: call `inner.ChatStream`; if it returns `(nil, err)`, treat as initial connection failure and retry per the same rules as `Chat`
- [x] 7.2 If `inner.ChatStream` returns `(ch, nil)` (stream started), return `(ch, nil)` immediately — do NOT retry mid-stream failures
- [x] 7.3 If the initial error is non-retryable, return it immediately without backoff

## 8. MP0 Coordination (Cross-Cutting Contract)

- [x] 8.1 Update MP0's `OpenRouterProvider` error handling to return `*outbound.ProviderError{StatusCode: resp.StatusCode, Body: body}` for non-2xx responses (coordinate with MP0 implementation; this is required for the default predicate to classify HTTP errors)
- [x] 8.2 Confirm MP0's `ChatStream` returns `(nil, err)` on initial connection failure and `(ch, nil)` once the stream starts — `RetryProvider.ChatStream` depends on this contract

## 9. Wiring (Optional)

- [x] 9.1 In `main.go` and `cmd/cli/main.go`, optionally wrap `OpenRouterProvider` in `retry.NewRetryProvider(provider)` at construction time once MP0 is implemented
- [x] 9.2 Keep wiring behind the MP0 land — if MP0 is not yet implemented, `RetryProvider` is usable in isolation via tests

## 10. Verification

- [x] 10.1 Run `go build ./...` — all packages compile (including `internal/adapters/driven/retry` and `internal/core/ports/outbound`)
- [x] 10.2 Run `go vet ./...` — no warnings
- [x] 10.3 Write table-driven tests for `defaultIsRetryable` covering 429, 529, 500, 400, 401, network error, `context.Canceled`, `context.DeadlineExceeded`
- [x] 10.4 Write a test for `RetryProvider.Chat` using a fake `Provider` that fails N times then succeeds — assert exact attempt count and that backoff was waited
- [x] 10.5 Write a test asserting non-retryable errors (400) return immediately with zero retries
- [x] 10.6 Write a test asserting context cancellation during backoff aborts the loop and returns the last error
- [x] 10.7 Write a test for `RetryProvider.ChatStream` — initial connection error retried, mid-stream failure NOT retried
- [x] 10.8 Run `go test ./...` — all tests pass
