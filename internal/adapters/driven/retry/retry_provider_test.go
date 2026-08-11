package retry

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
	"github.com/lin-br/go-linai-tools/internal/core/ports/outbound"
)

// ---------------------------------------------------------------------------
// Fake provider
// ---------------------------------------------------------------------------

// fakeProvider is a test double for outbound.Provider. It returns errors from
// chatErrors/streamErrors in order, then falls back to the success fields.
type fakeProvider struct {
	mu sync.Mutex

	chatCalls   int
	chatErrors  []error
	chatResp    *domain.ChatResponse
	chatTimes   []time.Time

	streamCalls  int
	streamErrors []error
	streamCh     chan domain.StreamEvent

	// called is signaled on every Chat/ChatStream call (non-blocking send).
	called chan struct{}
}

func (f *fakeProvider) Chat(_ context.Context, _ *domain.ChatRequest) (*domain.ChatResponse, error) {
	f.mu.Lock()
	idx := f.chatCalls
	f.chatCalls++
	f.chatTimes = append(f.chatTimes, time.Now())
	errs := f.chatErrors
	f.mu.Unlock()

	f.signal()
	if idx < len(errs) {
		return nil, errs[idx]
	}
	return f.chatResp, nil
}

func (f *fakeProvider) ChatStream(_ context.Context, _ *domain.ChatRequest) (<-chan domain.StreamEvent, error) {
	f.mu.Lock()
	idx := f.streamCalls
	f.streamCalls++
	errs := f.streamErrors
	f.mu.Unlock()

	f.signal()
	if idx < len(errs) {
		return nil, errs[idx]
	}
	return f.streamCh, nil
}

func (f *fakeProvider) signal() {
	if f.called != nil {
		select {
		case f.called <- struct{}{}:
		default:
		}
	}
}

func (f *fakeProvider) chatCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.chatCalls
}

func (f *fakeProvider) streamCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.streamCalls
}

func providerErr(status int) *outbound.ProviderError {
	return &outbound.ProviderError{StatusCode: status, Body: fmt.Sprintf("HTTP %d", status)}
}

// ---------------------------------------------------------------------------
// 10.3 — defaultIsRetryable table-driven tests
// ---------------------------------------------------------------------------

func TestDefaultIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"429 is retryable", providerErr(429), true},
		{"529 is retryable", providerErr(529), true},
		{"500 is retryable", providerErr(500), true},
		{"503 is retryable", providerErr(503), true},
		{"599 is retryable", providerErr(599), true},
		{"400 is not retryable", providerErr(400), false},
		{"401 is not retryable", providerErr(401), false},
		{"403 is not retryable", providerErr(403), false},
		{"404 is not retryable", providerErr(404), false},
		{"422 is not retryable", providerErr(422), false},
		{"network error is retryable", errors.New("connection refused"), true},
		{"context.Canceled is not retryable", context.Canceled, false},
		{"context.DeadlineExceeded is not retryable", context.DeadlineExceeded, false},
		{
			name: "ProviderError wrapping context.Canceled is not retryable",
			err:  &outbound.ProviderError{StatusCode: 429, Err: context.Canceled},
			want: false,
		},
		{"nil error is not retryable", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := defaultIsRetryable(tt.err)
			if got != tt.want {
				t.Errorf("defaultIsRetryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 10.4 — Chat: fail N times then succeed, assert attempt count + backoff waited
// ---------------------------------------------------------------------------

func TestChatRetryThenSucceed(t *testing.T) {
	retryableErr := providerErr(429)
	resp := &domain.ChatResponse{Content: "ok"}

	fake := &fakeProvider{
		chatErrors: []error{retryableErr, retryableErr},
		chatResp:   resp,
	}

	rp := NewRetryProvider(fake,
		WithMaxRetries(3),
		WithBaseDelay(50*time.Millisecond),
		WithMaxDelay(30*time.Second),
	)

	start := time.Now()
	got, err := rp.Chat(context.Background(), &domain.ChatRequest{})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if got.Content != "ok" {
		t.Errorf("Content = %q, want %q", got.Content, "ok")
	}
	if calls := fake.chatCallCount(); calls != 3 {
		t.Errorf("chatCalls = %d, want 3 (2 failures + 1 success)", calls)
	}
	// Two backoff periods (50ms and 100ms caps). Assert at least 1ms total
	// elapsed to confirm backoff actually happened — with 50ms base, the
	// probability of both jittered delays being <0.5ms is negligible.
	if elapsed < time.Millisecond {
		t.Errorf("elapsed = %v, want >= 1ms (backoff was not waited)", elapsed)
	}
}

// ---------------------------------------------------------------------------
// 10.5 — Non-retryable error (400) returns immediately with zero retries
// ---------------------------------------------------------------------------

func TestChatNonRetryableReturnsImmediately(t *testing.T) {
	badRequest := providerErr(400)

	fake := &fakeProvider{
		chatErrors: []error{badRequest},
	}

	rp := NewRetryProvider(fake,
		WithMaxRetries(3),
		WithBaseDelay(50*time.Millisecond),
	)

	start := time.Now()
	_, err := rp.Chat(context.Background(), &domain.ChatRequest{})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var pe *outbound.ProviderError
	if !errors.As(err, &pe) || pe.StatusCode != 400 {
		t.Errorf("err = %v, want *ProviderError(400)", err)
	}
	if calls := fake.chatCallCount(); calls != 1 {
		t.Errorf("chatCalls = %d, want 1 (no retries for non-retryable)", calls)
	}
	if elapsed > 10*time.Millisecond {
		t.Errorf("elapsed = %v, want < 10ms (should return immediately)", elapsed)
	}
}

// ---------------------------------------------------------------------------
// 10.6 — Context cancellation during backoff aborts the loop
// ---------------------------------------------------------------------------

func TestChatContextCancellationDuringBackoff(t *testing.T) {
	retryableErr := providerErr(429)

	fake := &fakeProvider{
		chatErrors: []error{retryableErr}, // always fails
		called:     make(chan struct{}, 10),
	}

	rp := NewRetryProvider(fake,
		WithMaxRetries(5),
		WithBaseDelay(1*time.Second), // long backoff so cancellation lands first
	)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-fake.called // wait for the first Chat call
		cancel()
	}()

	_, err := rp.Chat(ctx, &domain.ChatRequest{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// The last provider error should be returned, not ctx.Err().
	var pe *outbound.ProviderError
	if !errors.As(err, &pe) || pe.StatusCode != 429 {
		t.Errorf("err = %v, want *ProviderError(429) as last error", err)
	}
	if calls := fake.chatCallCount(); calls != 1 {
		t.Errorf("chatCalls = %d, want 1 (cancellation should abort before retry)", calls)
	}
}

// ---------------------------------------------------------------------------
// Chat: exhausted retries return last error
// ---------------------------------------------------------------------------

func TestChatExhaustedRetriesReturnsLastError(t *testing.T) {
	retryableErr := providerErr(503)

	fake := &fakeProvider{
		chatErrors: []error{retryableErr, retryableErr, retryableErr, retryableErr},
	}

	rp := NewRetryProvider(fake,
		WithMaxRetries(3),
		WithBaseDelay(1*time.Millisecond),
	)

	_, err := rp.Chat(context.Background(), &domain.ChatRequest{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// MaxRetries=3 means 4 total attempts (initial + 3 retries).
	if calls := fake.chatCallCount(); calls != 4 {
		t.Errorf("chatCalls = %d, want 4 (initial + 3 retries)", calls)
	}
	var pe *outbound.ProviderError
	if !errors.As(err, &pe) || pe.StatusCode != 503 {
		t.Errorf("err = %v, want *ProviderError(503)", err)
	}
}

// ---------------------------------------------------------------------------
// Chat: zero MaxRetries means no retries
// ---------------------------------------------------------------------------

func TestChatZeroMaxRetriesNoRetries(t *testing.T) {
	retryableErr := providerErr(429)

	fake := &fakeProvider{
		chatErrors: []error{retryableErr},
	}

	// Bypass NewRetryProvider (which defaults MaxRetries=0 → 3) to test
	// the zero-MaxRetries code path directly.
	rp := &RetryProvider{
		inner: fake,
		opts: RetryOptions{
			MaxRetries:  0,
			BaseDelay:   1 * time.Second,
			MaxDelay:    30 * time.Second,
			IsRetryable: defaultIsRetryable,
		},
	}

	_, err := rp.Chat(context.Background(), &domain.ChatRequest{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls := fake.chatCallCount(); calls != 1 {
		t.Errorf("chatCalls = %d, want 1 (no retries with MaxRetries=0)", calls)
	}
}

// ---------------------------------------------------------------------------
// Chat: successful first attempt makes no extra calls
// ---------------------------------------------------------------------------

func TestChatSuccessFirstAttempt(t *testing.T) {
	resp := &domain.ChatResponse{Content: "hello"}
	fake := &fakeProvider{chatResp: resp}

	rp := NewRetryProvider(fake, WithBaseDelay(1*time.Millisecond))

	got, err := rp.Chat(context.Background(), &domain.ChatRequest{})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if got.Content != "hello" {
		t.Errorf("Content = %q, want %q", got.Content, "hello")
	}
	if calls := fake.chatCallCount(); calls != 1 {
		t.Errorf("chatCalls = %d, want 1", calls)
	}
}

// ---------------------------------------------------------------------------
// 10.7 — ChatStream: initial connection error retried, mid-stream failure NOT retried
// ---------------------------------------------------------------------------

func TestChatStreamInitialErrorRetried(t *testing.T) {
	retryableErr := providerErr(429)
	ch := make(chan domain.StreamEvent)

	fake := &fakeProvider{
		streamErrors: []error{retryableErr}, // first attempt fails
		streamCh:     ch,                    // second attempt succeeds
	}

	rp := NewRetryProvider(fake,
		WithMaxRetries(3),
		WithBaseDelay(1*time.Millisecond),
	)

	gotCh, err := rp.ChatStream(context.Background(), &domain.ChatRequest{})
	if err != nil {
		t.Fatalf("ChatStream returned error: %v", err)
	}
	if gotCh == nil {
		t.Fatal("ChatStream returned nil channel")
	}
	if calls := fake.streamCallCount(); calls != 2 {
		t.Errorf("streamCalls = %d, want 2 (1 failure + 1 success)", calls)
	}
}

func TestChatStreamMidStreamFailureNotRetried(t *testing.T) {
	ch := make(chan domain.StreamEvent, 1)

	fake := &fakeProvider{
		streamCh: ch, // stream starts successfully
	}

	rp := NewRetryProvider(fake,
		WithMaxRetries(3),
		WithBaseDelay(1*time.Millisecond),
	)

	gotCh, err := rp.ChatStream(context.Background(), &domain.ChatRequest{})
	if err != nil {
		t.Fatalf("ChatStream returned error: %v", err)
	}
	if gotCh == nil {
		t.Fatal("ChatStream returned nil channel")
	}

	// Simulate a mid-stream failure by sending an error event then closing.
	ch <- domain.StreamEvent{Type: domain.StreamEventTypeError, Err: errors.New("stream dropped")}
	close(ch)

	// Drain the channel to allow the streamLoop goroutine to finish.
	for range gotCh {
	}

	if calls := fake.streamCallCount(); calls != 1 {
		t.Errorf("streamCalls = %d, want 1 (mid-stream failure should NOT retry)", calls)
	}
}

func TestChatStreamNonRetryableErrorReturnsImmediately(t *testing.T) {
	badRequest := providerErr(400)

	fake := &fakeProvider{
		streamErrors: []error{badRequest},
	}

	rp := NewRetryProvider(fake,
		WithMaxRetries(3),
		WithBaseDelay(1*time.Millisecond),
	)

	_, err := rp.ChatStream(context.Background(), &domain.ChatRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var pe *outbound.ProviderError
	if !errors.As(err, &pe) || pe.StatusCode != 400 {
		t.Errorf("err = %v, want *ProviderError(400)", err)
	}
	if calls := fake.streamCallCount(); calls != 1 {
		t.Errorf("streamCalls = %d, want 1 (non-retryable, no retries)", calls)
	}
}

// ---------------------------------------------------------------------------
// backoffDelay — cap and jitter sanity
// ---------------------------------------------------------------------------

func TestBackoffDelayCappedAtMax(t *testing.T) {
	base := 1 * time.Second
	max := 30 * time.Second

	// With a huge attempt, the cap should kick in.
	for i := 0; i < 100; i++ {
		d := backoffDelay(50, base, max)
		if d > max {
			t.Fatalf("backoffDelay(50, %v, %v) = %v, exceeds max", base, max, d)
		}
		if d < 0 {
			t.Fatalf("backoffDelay returned negative: %v", d)
		}
	}
}

func TestBackoffDelayNeverPanicsOnZero(t *testing.T) {
	// capped=0 must not panic on rand.Int63n(0).
	d := backoffDelay(0, 0, 30*time.Second)
	if d != 0 {
		t.Errorf("backoffDelay with base=0 = %v, want 0", d)
	}
	d = backoffDelay(0, 1*time.Second, 0)
	if d != 0 {
		t.Errorf("backoffDelay with max=0 = %v, want 0", d)
	}
}

func TestBackoffDelayWithinExpectedRange(t *testing.T) {
	base := 100 * time.Millisecond
	max := 30 * time.Second

	// attempt=0 → capped=100ms, jitter in [0, 100ms]
	for i := 0; i < 100; i++ {
		d := backoffDelay(0, base, max)
		if d < 0 || d > base {
			t.Fatalf("backoffDelay(0, %v, %v) = %v, want [0, %v]", base, max, d, base)
		}
	}
}

// ---------------------------------------------------------------------------
// wait — context-aware behavior
// ---------------------------------------------------------------------------

func TestWaitReturnsNilOnTimerFire(t *testing.T) {
	if err := wait(context.Background(), 1*time.Millisecond); err != nil {
		t.Errorf("wait with fresh ctx = %v, want nil", err)
	}
}

func TestWaitReturnsContextErrorOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := wait(ctx, 1*time.Second); err != context.Canceled {
		t.Errorf("wait with cancelled ctx = %v, want context.Canceled", err)
	}
}

func TestWaitZeroDelayRespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := wait(ctx, 0); err != context.Canceled {
		t.Errorf("wait(0) with cancelled ctx = %v, want context.Canceled", err)
	}
}

// ---------------------------------------------------------------------------
// Constructor defaults
// ---------------------------------------------------------------------------

func TestNewRetryProviderDefaults(t *testing.T) {
	rp := NewRetryProvider(&fakeProvider{})

	if rp.opts.MaxRetries != 3 {
		t.Errorf("default MaxRetries = %d, want 3", rp.opts.MaxRetries)
	}
	if rp.opts.BaseDelay != 1*time.Second {
		t.Errorf("default BaseDelay = %v, want 1s", rp.opts.BaseDelay)
	}
	if rp.opts.MaxDelay != 30*time.Second {
		t.Errorf("default MaxDelay = %v, want 30s", rp.opts.MaxDelay)
	}
	if rp.opts.IsRetryable == nil {
		t.Error("default IsRetryable is nil")
	}
}

func TestNewRetryProviderOverrides(t *testing.T) {
	custom := func(error) bool { return false }
	rp := NewRetryProvider(&fakeProvider{},
		WithMaxRetries(5),
		WithBaseDelay(500*time.Millisecond),
		WithMaxDelay(10*time.Second),
		WithRetryable(custom),
	)

	if rp.opts.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", rp.opts.MaxRetries)
	}
	if rp.opts.BaseDelay != 500*time.Millisecond {
		t.Errorf("BaseDelay = %v, want 500ms", rp.opts.BaseDelay)
	}
	if rp.opts.MaxDelay != 10*time.Second {
		t.Errorf("MaxDelay = %v, want 10s", rp.opts.MaxDelay)
	}
	// Custom predicate replaces default.
	if rp.opts.IsRetryable(nil) != false {
		t.Error("custom IsRetryable not applied")
	}
}
