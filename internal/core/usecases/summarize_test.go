package usecases

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
	"github.com/lin-br/go-linai-tools/internal/core/ports/outbound"
)

// fakeProvider is a test double for outbound.Provider used by the summarize
// tests. It captures the ChatRequest passed to ChatStream and either returns a
// connection error (streamErr) or emits the configured events on a channel.
//
// When streamCh is pre-set, it is returned as-is (the test controls emission).
// Otherwise the fake spawns a goroutine that emits events in order, with an
// optional delay between events, respecting context cancellation by closing
// the channel — mirroring the MP1 streaming contract.
type fakeProvider struct {
	capturedReq *domain.ChatRequest
	streamErr   error
	streamCh    chan domain.StreamEvent
	events      []domain.StreamEvent
	delay       time.Duration
	chatResp    *domain.ChatResponse
	chatErr     error
	chatGotCtx  context.Context
	chatGotReq  *domain.ChatRequest
	chatCalls   int
}

func (f *fakeProvider) Chat(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	f.chatGotCtx = ctx
	f.chatGotReq = req
	f.chatCalls++
	if f.chatErr != nil {
		return nil, f.chatErr
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return f.chatResp, nil
}

func (f *fakeProvider) ChatStream(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamEvent, error) {
	f.capturedReq = req
	if f.streamErr != nil {
		return nil, f.streamErr
	}
	if f.streamCh != nil {
		return f.streamCh, nil
	}
	ch := make(chan domain.StreamEvent)
	go func() {
		defer close(ch)
		for _, ev := range f.events {
			if f.delay > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(f.delay):
				}
			}
			select {
			case <-ctx.Done():
				return
			case ch <- ev:
			}
		}
	}()
	return ch, nil
}

// Compile-time check: fakeProvider satisfies outbound.Provider.
var _ outbound.Provider = (*fakeProvider)(nil)

// 9.2 — Request building: Stream builds a streaming ChatRequest with the right
// model, system prompt, Stream flag, and a single user message.
func TestSummarizeStreamBuildsRequest(t *testing.T) {
	fake := &fakeProvider{
		events: []domain.StreamEvent{{Type: domain.StreamEventTypeFinish}},
	}
	uc := NewSummarizeUseCase(fake)

	err := uc.Stream(context.Background(), "anthropic/claude-sonnet-4-20250514", "Summarize concisely", "This is a long text to summarize", &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}

	req := fake.capturedReq
	if req == nil {
		t.Fatal("provider did not receive a request")
	}
	if !req.Stream {
		t.Error("req.Stream = false, want true")
	}
	if req.Model != "anthropic/claude-sonnet-4-20250514" {
		t.Errorf("req.Model = %q, want anthropic/claude-sonnet-4-20250514", req.Model)
	}
	if req.System != "Summarize concisely" {
		t.Errorf("req.System = %q, want %q", req.System, "Summarize concisely")
	}
	if len(req.Messages) != 1 {
		t.Fatalf("len(req.Messages) = %d, want 1", len(req.Messages))
	}
	msg := req.Messages[0]
	if msg.Role != domain.MessageRoleUser {
		t.Errorf("msg.Role = %q, want %q", msg.Role, domain.MessageRoleUser)
	}
	if msg.Content != "This is a long text to summarize" {
		t.Errorf("msg.Content = %q, want the input text", msg.Content)
	}
	// Optional fields must be zero-valued.
	if len(req.Tools) != 0 || req.ToolChoice != nil || req.MaxTokens != 0 || req.Temperature != nil || req.TopP != nil {
		t.Error("optional fields should be zero-valued")
	}
}

// 9.3 — Text deltas are written in order and flushed.
func TestSummarizeStreamWritesDeltasInOrder(t *testing.T) {
	fake := &fakeProvider{
		events: []domain.StreamEvent{
			{Type: domain.StreamEventTypeText, Delta: "Hello"},
			{Type: domain.StreamEventTypeText, Delta: " world"},
		},
	}
	uc := NewSummarizeUseCase(fake)

	var buf bytes.Buffer
	if err := uc.Stream(context.Background(), "m", "s", "in", &buf); err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}
	if got := buf.String(); got != "Hello world" {
		t.Errorf("output = %q, want %q", got, "Hello world")
	}
}

// 9.4 — Empty delta is skipped (nothing written).
func TestSummarizeStreamSkipsEmptyDelta(t *testing.T) {
	fake := &fakeProvider{
		events: []domain.StreamEvent{
			{Type: domain.StreamEventTypeText, Delta: ""},
		},
	}
	uc := NewSummarizeUseCase(fake)

	var buf bytes.Buffer
	if err := uc.Stream(context.Background(), "m", "s", "in", &buf); err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("output = %q, want empty", buf.String())
	}
}

// 9.5 — Finish event writes nothing; only the text delta appears.
func TestSummarizeStreamFinishWritesNothing(t *testing.T) {
	fake := &fakeProvider{
		events: []domain.StreamEvent{
			{Type: domain.StreamEventTypeText, Delta: "summary"},
			{Type: domain.StreamEventTypeFinish, StopReason: "stop"},
		},
	}
	uc := NewSummarizeUseCase(fake)

	var buf bytes.Buffer
	if err := uc.Stream(context.Background(), "m", "s", "in", &buf); err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}
	if got := buf.String(); got != "summary" {
		t.Errorf("output = %q, want %q", got, "summary")
	}
}

// 9.6 — Initial connection error: ChatStream returns (nil, err); Stream returns
// the error and writes nothing.
func TestSummarizeStreamInitialConnectionError(t *testing.T) {
	connErr := errors.New("401 unauthorized")
	fake := &fakeProvider{streamErr: connErr}
	uc := NewSummarizeUseCase(fake)

	var buf bytes.Buffer
	err := uc.Stream(context.Background(), "m", "s", "in", &buf)
	if !errors.Is(err, connErr) {
		t.Errorf("Stream err = %v, want %v", err, connErr)
	}
	if buf.Len() != 0 {
		t.Errorf("writer = %q, want empty", buf.String())
	}
}

// 9.7 — Mid-stream error: partial delta is written, then the error is returned.
func TestSummarizeStreamMidStreamError(t *testing.T) {
	streamErr := errors.New("connection reset")
	fake := &fakeProvider{
		events: []domain.StreamEvent{
			{Type: domain.StreamEventTypeText, Delta: "Summar"},
			{Type: domain.StreamEventTypeError, Err: streamErr},
		},
	}
	uc := NewSummarizeUseCase(fake)

	var buf bytes.Buffer
	err := uc.Stream(context.Background(), "m", "s", "in", &buf)
	if !errors.Is(err, streamErr) {
		t.Errorf("Stream err = %v, want %v", err, streamErr)
	}
	if got := buf.String(); got != "Summar" {
		t.Errorf("output = %q, want %q", got, "Summar")
	}
}

// 9.8 — Context cancellation: Stream returns without deadlock and writes no
// further deltas after cancellation.
func TestSummarizeStreamContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	fake := &fakeProvider{
		events: []domain.StreamEvent{
			{Type: domain.StreamEventTypeText, Delta: "first"},
			{Type: domain.StreamEventTypeText, Delta: "second"},
		},
		delay: 50 * time.Millisecond,
	}
	uc := NewSummarizeUseCase(fake)

	var buf bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- uc.Stream(ctx, "m", "s", "in", &buf)
	}()

	// Let the first event through, then cancel before the second arrives.
	time.Sleep(75 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Stream returned — no deadlock.
	case <-time.After(5 * time.Second):
		t.Fatal("Stream did not return after context cancellation (deadlock)")
	}

	if strings.Contains(buf.String(), "second") {
		t.Errorf("deltas written after cancellation: %q", buf.String())
	}
}

// DefaultSummarizeSystemPrompt is non-empty and directive.
func TestDefaultSummarizeSystemPromptNonEmpty(t *testing.T) {
	if DefaultSummarizeSystemPrompt == "" {
		t.Error("DefaultSummarizeSystemPrompt is empty")
	}
}
