package usecases

import (
	"bufio"
	"context"
	"io"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
	"github.com/lin-br/go-linai-tools/internal/core/ports/outbound"
)

// DefaultSummarizeSystemPrompt is the focused summarization directive used when
// the caller does not supply a -system flag. It asks for a concise summary that
// captures key points, decisions, and action items, with no preamble.
const DefaultSummarizeSystemPrompt = "Summarize the following text concisely. Capture key points, decisions, and action items. Be direct — no preamble."

// SummarizeUseCase builds a streaming ChatRequest from a single user message
// and writes text deltas from the provider's StreamEvent channel to an
// io.Writer as they arrive. It holds only an outbound.Provider — model and
// system prompt are resolved by the caller and passed to Stream, keeping the
// use case config-agnostic and testable.
type SummarizeUseCase struct {
	provider outbound.Provider
}

// NewSummarizeUseCase constructs a SummarizeUseCase that delegates chat
// operations to the given provider. The provider may be a raw Provider or a
// RetryProvider decorator; both satisfy the outbound.Provider interface.
func NewSummarizeUseCase(provider outbound.Provider) *SummarizeUseCase {
	return &SummarizeUseCase{provider: provider}
}

// Stream builds a streaming ChatRequest with the given model, system prompt,
// and single user message (input), then ranges over the provider's event
// channel, writing text deltas to out via a bufio.Writer that is flushed after
// each delta so tokens appear immediately.
//
// Error handling follows two paths, per the MP4 design:
//   - Initial connection error: ChatStream returns (nil, err). Stream returns
//     the error immediately without writing to out.
//   - Mid-stream error: a StreamEvent{Type: Error} arrives after some deltas.
//     Already-flushed deltas remain on out; Stream returns the event's Err.
//
// The context is passed unmodified to provider.ChatStream. If it is cancelled
// mid-stream, the provider closes the channel (per the MP1 contract) and
// Stream returns without writing further deltas.
func (uc *SummarizeUseCase) Stream(ctx context.Context, model, systemPrompt, input string, out io.Writer) error {
	req := &domain.ChatRequest{
		Model:  model,
		System: systemPrompt,
		Stream: true,
		Messages: []domain.Message{
			{Role: domain.MessageRoleUser, Content: input},
		},
	}

	ch, err := uc.provider.ChatStream(ctx, req)
	if err != nil {
		return err
	}

	w := bufio.NewWriter(out)
	for event := range ch {
		switch event.Type {
		case domain.StreamEventTypeText:
			if event.Delta == "" {
				continue
			}
			if _, werr := w.WriteString(event.Delta); werr != nil {
				return werr
			}
			if ferr := w.Flush(); ferr != nil {
				return ferr
			}
		case domain.StreamEventTypeFinish:
			// Nothing to write for finish events; continue ranging.
		case domain.StreamEventTypeError:
			return event.Err
		}
	}
	return nil
}
