## ADDED Requirements

### Requirement: RetryProvider implements the Provider interface

The system SHALL define a `RetryProvider` struct in `internal/adapters/driven/retry` that wraps an inner `Provider` and itself satisfies the `outbound.Provider` interface. The `RetryProvider` SHALL expose both `Chat(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error)` and `ChatStream(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamEvent, error)`, delegating to the inner provider on each attempt.

#### Scenario: RetryProvider is usable wherever a Provider is expected

- **WHEN** a `RetryProvider` is constructed around an inner `Provider` and passed to a consumer expecting `outbound.Provider`
- **THEN** the consumer SHALL be able to call `Chat` and `ChatStream` without knowing retries are enabled

#### Scenario: Successful first attempt makes no extra calls

- **WHEN** the inner provider's `Chat` succeeds on the first attempt
- **THEN** `RetryProvider.Chat` SHALL return the inner provider's response immediately and SHALL NOT invoke the inner provider again

### Requirement: RetryOptions and functional-options constructor

The system SHALL define a `RetryOptions` struct with fields `MaxRetries int`, `BaseDelay time.Duration`, `MaxDelay time.Duration`, and `IsRetryable func(error) bool`. The system SHALL provide a constructor `NewRetryProvider(inner Provider, opts ...RetryOption) *RetryProvider` where `RetryOption` is `func(*RetryOptions)`. When no options are supplied, the constructor SHALL apply defaults: `MaxRetries=3`, `BaseDelay=1s`, `MaxDelay=30s`, and the default `IsRetryable` predicate.

#### Scenario: Defaults applied with no options

- **WHEN** `NewRetryProvider(inner)` is called with zero options
- **THEN** the resulting `RetryProvider` SHALL use `MaxRetries=3`, `BaseDelay=1s`, `MaxDelay=30s`, and the default retryable predicate

#### Scenario: Override via functional options

- **WHEN** `NewRetryProvider(inner, WithMaxRetries(5), WithBaseDelay(500*time.Millisecond))` is called
- **THEN** the resulting `RetryProvider` SHALL use `MaxRetries=5` and `BaseDelay=500ms` while retaining defaults for unspecified options

#### Scenario: Custom retryable predicate

- **WHEN** `NewRetryProvider(inner, WithRetryable(func(error) bool { return false }))` is called
- **THEN** the resulting `RetryProvider` SHALL never retry, returning the first error immediately

### Requirement: ProviderError type carries HTTP status code

The system SHALL define a `ProviderError` struct in `internal/core/ports/outbound` with fields `StatusCode int`, `Body string`, and `Err error`. `ProviderError` SHALL implement the `error` interface (an `Error() string` method that includes the status code and body) and SHALL support `errors.As` / `errors.Is` unwrapping via an `Unwrap() error` method that returns `Err`.

#### Scenario: Inspectable status code

- **WHEN** a caller receives an error from a provider and performs `errors.As(err, &providerErr)`
- **THEN** `providerErr.StatusCode` SHALL expose the HTTP status code so retryability can be decided without string parsing

#### Scenario: Wrapped inner error

- **WHEN** `ProviderError.Err` is non-nil
- **THEN** `errors.Unwrap` / `errors.Is` against the inner error SHALL succeed

### Requirement: Providers return ProviderError for HTTP errors

Any `Provider` implementation that issues HTTP requests SHALL return a `*ProviderError` for non-2xx HTTP responses, with `StatusCode` set to the HTTP status code and `Body` set to the response body. This is a cross-cutting contract: MP0's `OpenRouterProvider` MUST return `*ProviderError` for HTTP errors so the retry predicate can inspect the status code.

#### Scenario: OpenRouter 429 surfaces as ProviderError

- **WHEN** `OpenRouterProvider.Chat` receives an HTTP 429 response
- **THEN** the returned error SHALL be a `*ProviderError` with `StatusCode=429`

#### Scenario: OpenRouter 400 surfaces as ProviderError

- **WHEN** `OpenRouterProvider.Chat` receives an HTTP 400 response
- **THEN** the returned error SHALL be a `*ProviderError` with `StatusCode=400`

### Requirement: Default retryable error predicate

The system SHALL provide a default `IsRetryable` predicate that classifies errors as follows: an error that type-asserts to `*ProviderError` is retryable when `StatusCode` is 429, 529, or any 5xx value (500–599); it is NOT retryable for other 4xx values (400, 401, 403, 404, 422, etc.). An error that is NOT a `*ProviderError` (a network/transport error) is retryable by default. An error for which `errors.Is(err, context.Canceled)` or `errors.Is(err, context.DeadlineExceeded)` is true is NOT retryable.

#### Scenario: 429 is retryable

- **WHEN** the default predicate receives a `*ProviderError` with `StatusCode=429`
- **THEN** it SHALL return true

#### Scenario: 529 is retryable

- **WHEN** the default predicate receives a `*ProviderError` with `StatusCode=529`
- **THEN** it SHALL return true

#### Scenario: 500 is retryable

- **WHEN** the default predicate receives a `*ProviderError` with `StatusCode=500`
- **THEN** it SHALL return true

#### Scenario: 400 is not retryable

- **WHEN** the default predicate receives a `*ProviderError` with `StatusCode=400`
- **THEN** it SHALL return false

#### Scenario: Network error is retryable

- **WHEN** the default predicate receives a `*url.Error` (transport-level failure with no HTTP response)
- **THEN** it SHALL return true

#### Scenario: Context cancellation is not retryable

- **WHEN** the default predicate receives `context.Canceled`
- **THEN** it SHALL return false

### Requirement: Exponential backoff with jitter

The system SHALL compute the backoff delay before each retry attempt as `capped = min(baseDelay * 2^attempt, maxDelay)` followed by full jitter `delay = rand(0, capped)`, where `attempt` is zero-indexed (the delay after the first failure uses `attempt=0`). The delay SHALL never exceed `maxDelay`. Jitter SHALL use `math/rand`.

#### Scenario: Delay grows exponentially up to the cap

- **WHEN** `BaseDelay=1s`, `MaxDelay=30s`, and the first attempt fails
- **THEN** the delay before the second attempt SHALL be in the range `[0, 1s]`

#### Scenario: Cap is respected

- **WHEN** `BaseDelay=1s`, `MaxDelay=30s`, and `attempt=10` (would naively yield 1024s)
- **THEN** the capped base SHALL be `30s` and the delay SHALL be in the range `[0, 30s]`

### Requirement: Context-aware backoff waits

The system SHALL wait for the computed backoff delay using a `time.Timer` and a `select` on `ctx.Done()`. If the context is cancelled or expires during the wait, the timer SHALL be stopped and the retry loop SHALL abort immediately without making another attempt.

#### Scenario: Context cancelled during backoff

- **WHEN** the context is cancelled while waiting between retry attempts
- **THEN** `RetryProvider` SHALL stop the timer and return immediately without invoking the inner provider again

#### Scenario: Error returned on context cancellation during backoff

- **WHEN** the context is cancelled during a backoff wait and a previous attempt produced an error
- **THEN** `RetryProvider` SHALL return the last provider error if non-nil, otherwise `ctx.Err()`

### Requirement: Chat retry behavior

`RetryProvider.Chat` SHALL invoke the inner provider's `Chat`. If the result is an error and `IsRetryable(err)` returns true and the attempt count has not exceeded `MaxRetries`, it SHALL wait the backoff delay (context-aware) and retry. If `IsRetryable(err)` returns false, it SHALL return the error immediately without retrying. After exhausting `MaxRetries`, it SHALL return the last error.

#### Scenario: Retry then succeed

- **WHEN** the inner `Chat` returns a retryable error on attempts 1 and 2, then succeeds on attempt 3
- **THEN** `RetryProvider.Chat` SHALL return the successful response from attempt 3

#### Scenario: Non-retryable error fails immediately

- **WHEN** the inner `Chat` returns a `*ProviderError` with `StatusCode=400`
- **THEN** `RetryProvider.Chat` SHALL return that error immediately without any backoff or retry

#### Scenario: Exhausted retries return last error

- **WHEN** the inner `Chat` returns a retryable error on every attempt up to `MaxRetries`
- **THEN** `RetryProvider.Chat` SHALL return the last error after the final attempt and SHALL NOT retry further

#### Scenario: Zero MaxRetries means no retries

- **WHEN** `MaxRetries=0` and the inner `Chat` returns a retryable error
- **THEN** `RetryProvider.Chat` SHALL return the error immediately with no retry attempts

### Requirement: ChatStream retries only on initial connection error

`RetryProvider.ChatStream` SHALL invoke the inner provider's `ChatStream`. If the inner provider returns a non-nil error (and a nil channel), `RetryProvider.ChatStream` SHALL treat it as a retryable initial connection failure and retry per the same backoff and `IsRetryable` rules as `Chat`. If the inner provider returns a non-nil channel and a nil error, the stream has started and `RetryProvider.ChatStream` SHALL return the channel immediately without retrying, even if the stream fails mid-way.

#### Scenario: Initial connection error is retried

- **WHEN** the inner `ChatStream` returns `(nil, err)` with a retryable `err` on the first attempt, then returns `(ch, nil)` on the second
- **THEN** `RetryProvider.ChatStream` SHALL retry and return the channel from the second attempt

#### Scenario: Mid-stream failure is not retried

- **WHEN** the inner `ChatStream` returns `(ch, nil)` (stream started) and the stream later fails with an error event on the channel
- **THEN** `RetryProvider.ChatStream` SHALL NOT retry and the caller SHALL receive the error event on the channel as-is

#### Scenario: Non-retryable initial error fails immediately

- **WHEN** the inner `ChatStream` returns `(nil, err)` where `err` is a `*ProviderError` with `StatusCode=400`
- **THEN** `RetryProvider.ChatStream` SHALL return the error immediately without retrying

### Requirement: No external retry dependencies

The retry implementation SHALL use only the Go standard library (`context`, `time`, `math/rand`, `errors`, `net/http` types). The system SHALL NOT introduce `cenkalti/backoff` or any third-party retry/backoff library.

#### Scenario: Standard library only

- **WHEN** the `internal/adapters/driven/retry` package is built
- **THEN** it SHALL compile using only stdlib imports plus the project's own `domain` and `outbound` packages
