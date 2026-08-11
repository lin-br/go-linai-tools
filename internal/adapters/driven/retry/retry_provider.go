// Package retry provides a RetryProvider decorator that wraps an outbound.Provider
// with exponential backoff and jitter, retrying transient failures (429, 529, 5xx,
// network errors) while failing fast on permanent client errors (4xx).
//
// The decorator is transparent: it implements the same outbound.Provider interface,
// so callers can swap a raw provider for a retry-wrapped one without code changes.
// Only the Go standard library is used (context, time, math/rand, errors).
package retry

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
	"github.com/lin-br/go-linai-tools/internal/core/ports/outbound"
)

// Compile-time interface check: RetryProvider satisfies outbound.Provider.
var _ outbound.Provider = (*RetryProvider)(nil)

// RetryOptions configures the retry behavior of a RetryProvider.
//
// MaxRetries is the number of retry attempts after the initial call (0 means
// no retries — the initial call runs once and any error is returned as-is).
// BaseDelay is the delay before the first retry; subsequent delays double.
// MaxDelay caps the computed delay regardless of attempt count.
// IsRetryable decides whether a given error should trigger a retry.
type RetryOptions struct {
	MaxRetries  int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	IsRetryable func(error) bool
}

// RetryOption configures a RetryProvider at construction time.
type RetryOption func(*RetryOptions)

// WithMaxRetries overrides the default retry count.
func WithMaxRetries(n int) RetryOption {
	return func(o *RetryOptions) { o.MaxRetries = n }
}

// WithBaseDelay overrides the base (pre-jitter) backoff delay.
func WithBaseDelay(d time.Duration) RetryOption {
	return func(o *RetryOptions) { o.BaseDelay = d }
}

// WithMaxDelay overrides the maximum backoff delay cap.
func WithMaxDelay(d time.Duration) RetryOption {
	return func(o *RetryOptions) { o.MaxDelay = d }
}

// WithRetryable replaces the default retryable-error predicate.
func WithRetryable(fn func(error) bool) RetryOption {
	return func(o *RetryOptions) { o.IsRetryable = fn }
}

// RetryProvider wraps an inner Provider and retries transient failures with
// exponential backoff and full jitter. It implements outbound.Provider.
type RetryProvider struct {
	inner outbound.Provider
	opts  RetryOptions
}

// NewRetryProvider constructs a RetryProvider around inner, applying user
// options first and then filling zero-value fields with sensible defaults
// (MaxRetries=3, BaseDelay=1s, MaxDelay=30s, IsRetryable=defaultIsRetryable).
//
// Passing no options yields a provider tuned for typical LLM API traffic.
func NewRetryProvider(inner outbound.Provider, opts ...RetryOption) *RetryProvider {
	o := RetryOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	if o.MaxRetries == 0 {
		o.MaxRetries = 3
	}
	if o.BaseDelay == 0 {
		o.BaseDelay = 1 * time.Second
	}
	if o.MaxDelay == 0 {
		o.MaxDelay = 30 * time.Second
	}
	if o.IsRetryable == nil {
		o.IsRetryable = defaultIsRetryable
	}
	return &RetryProvider{inner: inner, opts: o}
}

// defaultIsRetryable classifies errors for the retry loop.
//
//   - *outbound.ProviderError with status 429, 529, or any 5xx (500–599) → retryable.
//   - *outbound.ProviderError with other 4xx status (400, 401, 403, …) → not retryable.
//   - Non-ProviderError errors (network/transport) → retryable by default.
//   - context.Canceled and context.DeadlineExceeded → never retryable.
func defaultIsRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var pe *outbound.ProviderError
	if errors.As(err, &pe) {
		switch {
		case pe.StatusCode == 429, pe.StatusCode == 529:
			return true
		case pe.StatusCode >= 500 && pe.StatusCode <= 599:
			return true
		default:
			return false
		}
	}
	return true
}

// backoffDelay computes the delay before the next retry attempt using
// exponential backoff capped at max, followed by full jitter.
//
//	capped = min(base * 2^attempt, max)
//	delay  = rand(0, capped)   // full jitter
//
// The returned delay is always in [0, max]. A zero base or max yields 0.
func backoffDelay(attempt int, base, max time.Duration) time.Duration {
	if base <= 0 || max <= 0 {
		return 0
	}
	capped := base
	for i := 0; i < attempt && capped < max; i++ {
		capped *= 2
		if capped < 0 { // int64 overflow guard
			capped = max
			break
		}
	}
	if capped > max {
		capped = max
	}
	// full jitter: [0, capped] inclusive; +1 prevents rand.Int63n(0) panic.
	return time.Duration(rand.Int63n(int64(capped) + 1))
}

// wait blocks for delay or until ctx is cancelled/expired, whichever comes first.
// It returns nil when the delay elapses (proceed to the next attempt) or the
// context error when ctx is done first.
func wait(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Chat calls the inner provider's Chat, retrying transient failures up to
// MaxRetries times with exponential backoff. Non-retryable errors are returned
// immediately. If the context is cancelled during a backoff wait, the last
// provider error is returned (or ctx.Err() when no provider error exists).
func (r *RetryProvider) Chat(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= r.opts.MaxRetries; attempt++ {
		resp, err := r.inner.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !r.opts.IsRetryable(err) {
			return nil, err
		}
		if attempt < r.opts.MaxRetries {
			if werr := wait(ctx, backoffDelay(attempt, r.opts.BaseDelay, r.opts.MaxDelay)); werr != nil {
				if lastErr != nil {
					return nil, lastErr
				}
				return nil, werr
			}
		}
	}
	return nil, lastErr
}

// ChatStream calls the inner provider's ChatStream. Only the initial connection
// error (non-nil error with nil channel) is retried; once a stream starts
// (non-nil channel), it is returned as-is and mid-stream failures are NOT
// retried, because partial output may already have been consumed.
func (r *RetryProvider) ChatStream(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamEvent, error) {
	var lastErr error
	for attempt := 0; attempt <= r.opts.MaxRetries; attempt++ {
		ch, err := r.inner.ChatStream(ctx, req)
		if err == nil {
			return ch, nil
		}
		lastErr = err
		if !r.opts.IsRetryable(err) {
			return nil, err
		}
		if attempt < r.opts.MaxRetries {
			if werr := wait(ctx, backoffDelay(attempt, r.opts.BaseDelay, r.opts.MaxDelay)); werr != nil {
				if lastErr != nil {
					return nil, lastErr
				}
				return nil, werr
			}
		}
	}
	return nil, lastErr
}
