package http_clients

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
)

const (
	sseInitialBuffer = 64 * 1024
	sseMaxBuffer     = 1 * 1024 * 1024
)

// sseSplitFunc tokenizes an OpenAI-style Server-Sent Event stream.
// It splits on "\n\n" boundaries, skips comment lines (starting with ':'),
// strips the "data: " prefix from the remaining line, and returns the JSON
// payload as the token. If a boundary is not yet available, it returns
// (0, nil, nil) so bufio.Scanner requests more data.
func sseSplitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {
	idx := bytes.Index(data, []byte("\n\n"))
	if idx == -1 {
		return 0, nil, nil
	}

	block := data[:idx]
	advance = idx + 2

	for _, line := range bytes.Split(block, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		if line[0] == ':' {
			continue
		}
		if bytes.HasPrefix(line, []byte("data: ")) {
			return advance, line[len("data: "):], nil
		}
	}

	// Comment-only or empty block: advance past it and let the scanner keep going.
	return advance, nil, nil
}

// newSSEScanner creates a bufio.Scanner configured for SSE tokens up to 1MB.
func newSSEScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, sseInitialBuffer), sseMaxBuffer)
	scanner.Split(sseSplitFunc)
	return scanner
}

// translateChunk parses a single SSE data payload into a domain StreamEvent.
// It returns nil when the chunk carries no content, finish_reason, or usage.
func translateChunk(data []byte) (*domain.StreamEvent, error) {
	var chunk ChatCompletionChunk
	if err := json.Unmarshal(data, &chunk); err != nil {
		return nil, fmt.Errorf("invalid chunk: %w", err)
	}

	if len(chunk.Choices) == 0 {
		return nil, nil
	}

	choice := chunk.Choices[0]

	if choice.FinishReason != nil && *choice.FinishReason != "" {
		event := &domain.StreamEvent{
			Type:       domain.StreamEventTypeFinish,
			StopReason: *choice.FinishReason,
		}
		if chunk.Usage != nil {
			event.Usage = wireUsageToDomain(chunk.Usage)
		}
		return event, nil
	}

	if choice.Delta.Content != "" {
		return &domain.StreamEvent{
			Type:  domain.StreamEventTypeText,
			Delta: choice.Delta.Content,
		}, nil
	}

	return nil, nil
}

// extractUsage parses a chunk just enough to extract its usage field.
// It is used for the edge case where usage arrives on a separate chunk.
func extractUsage(data []byte) *domain.Usage {
	var chunk ChatCompletionChunk
	if err := json.Unmarshal(data, &chunk); err != nil {
		return nil
	}
	return wireUsageToDomain(chunk.Usage)
}

func wireUsageToDomain(u *WireUsage) *domain.Usage {
	if u == nil {
		return nil
	}
	return &domain.Usage{
		InputTokens:  u.PromptTokens,
		OutputTokens: u.CompletionTokens,
		TotalTokens:  u.TotalTokens,
		Cost:         u.Cost,
	}
}

// streamLoop reads SSE events from body and sends them on ch.
// It closes both body and ch when it returns, either on completion, error,
// or context cancellation. Error events are sent for parse/read failures;
// cancellation produces no error event.
func streamLoop(ctx context.Context, body io.ReadCloser, ch chan<- domain.StreamEvent) {
	defer close(ch)

	var closeOnce sync.Once
	closeBody := func() { closeOnce.Do(func() { body.Close() }) }
	defer closeBody()

	// Abort the HTTP read if the caller cancels the context. The defer above
	// ensures we do not leak this goroutine when streamLoop exits normally.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			closeBody()
		case <-stop:
		}
	}()

	scanner := newSSEScanner(body)
	var pendingUsage *domain.Usage
	finishSent := false

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		data := scanner.Bytes()
		if bytes.Equal(data, []byte("[DONE]")) {
			return
		}

		event, err := translateChunk(data)
		if err != nil {
			select {
			case <-ctx.Done():
			case ch <- domain.StreamEvent{Type: domain.StreamEventTypeError, Err: err}:
			}
			return
		}

		if event != nil {
			if event.Type == domain.StreamEventTypeFinish && event.Usage == nil && pendingUsage != nil {
				event.Usage = pendingUsage
				pendingUsage = nil
			}

			select {
			case <-ctx.Done():
				return
			case ch <- *event:
			}

			if event.Type == domain.StreamEventTypeFinish {
				finishSent = true
			}
			continue
		}

		if usage := extractUsage(data); usage != nil {
			if finishSent {
				select {
				case <-ctx.Done():
					return
				case ch <- domain.StreamEvent{Type: domain.StreamEventTypeFinish, Usage: usage}:
				}
			} else {
				pendingUsage = usage
			}
		}
	}

	if err := scanner.Err(); err != nil {
		select {
		case <-ctx.Done():
		case ch <- domain.StreamEvent{Type: domain.StreamEventTypeError, Err: err}:
		}
	}
}
