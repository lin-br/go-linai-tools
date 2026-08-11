# Purpose

TBD

# Requirements

## Requirement: SSE parser uses bufio.Scanner with custom SplitFunc

The system SHALL implement an SSE parser that reads from an `io.Reader` (the HTTP response body) using `bufio.Scanner` with a custom `SplitFunc`. The SplitFunc SHALL split the byte stream on `\n\n` boundaries (double newline), strip the `data: ` prefix from each event, and return the JSON payload as a single token. The scanner buffer SHALL be configured with `scanner.Buffer()` to support tokens up to 1MB.

### Scenario: Single SSE event

- **WHEN** the response body contains `data: {"id":"gen-1","choices":[]}\n\n`
- **THEN** the scanner SHALL produce one token containing `{"id":"gen-1","choices":[]}`

### Scenario: Multiple SSE events in one read

- **WHEN** the response body contains multiple `data: {json}\n\n` blocks in a single read
- **THEN** the scanner SHALL produce one token per `data:` block, in order

### Scenario: Partial event across reads

- **WHEN** a `data: {json}\n\n` block is split across multiple `io.Reader` reads
- **THEN** the SplitFunc SHALL return `0, nil, nil` to request more data, and the scanner SHALL reassemble the complete event on the next read

### Scenario: Large event exceeds default buffer

- **WHEN** an SSE event payload exceeds the default 64KB scanner buffer
- **THEN** the scanner SHALL NOT error, because `scanner.Buffer()` was called with a 1MB maximum

## Requirement: data: [DONE] terminator handling

The system SHALL detect the `data: [DONE]` sentinel and treat it as the end of the SSE stream. When `[DONE]` is received, the parser SHALL stop scanning and close the event channel. The `[DONE]` line SHALL NOT be parsed as JSON.

### Scenario: Normal stream termination

- **WHEN** the stream sends `data: [DONE]\n\n`
- **THEN** the parser SHALL stop reading, and the event channel SHALL be closed

### Scenario: [DONE] after finish event

- **WHEN** a chunk with `finish_reason` is followed by `data: [DONE]`
- **THEN** the Finish event SHALL be emitted first, then the channel SHALL be closed on `[DONE]`

## Requirement: Comment lines are ignored

The system SHALL ignore SSE comment lines — lines starting with `:` (colon). These include OpenRouter keepalive comments such as `: OPENROUTER PROCESSING`. Comment lines SHALL NOT produce tokens, events, or errors.

### Scenario: Keepalive comment between data events

- **WHEN** the stream contains `: OPENROUTER PROCESSING\n\n` between two `data:` events
- **THEN** the comment SHALL be skipped, and only the `data:` events SHALL produce tokens

### Scenario: Comment line attached to a data event

- **WHEN** the stream contains `: comment\ndata: {json}\n\n`
- **THEN** the comment SHALL be stripped and the `data:` event SHALL produce a token

## Requirement: Chunk-to-StreamEvent translation

The system SHALL parse each SSE `data:` token (except `[DONE]`) as a `ChatCompletionChunk` wire type and translate it to zero or more `domain.StreamEvent` values according to these rules: if `choices[0].delta.content` is non-empty, emit a `StreamEvent{Type: StreamEventTypeText, Delta: content}`; if `choices[0].finish_reason` is non-nil, emit a `StreamEvent{Type: StreamEventTypeFinish, StopReason: finish_reason}`; if `usage` is non-nil, set the `Usage` field on the Finish event (or emit a standalone usage event if the Finish event was already sent).

### Scenario: Text delta chunk

- **WHEN** a chunk has `delta.content = "Hello"` and `finish_reason = null`
- **THEN** a `StreamEvent{Type: StreamEventTypeText, Delta: "Hello"}` SHALL be emitted

### Scenario: Empty content delta (role-only first chunk)

- **WHEN** a chunk has `delta.role = "assistant"` and `delta.content = ""` and `finish_reason = null`
- **THEN** no `StreamEvent` SHALL be emitted for this chunk

### Scenario: Finish chunk with usage

- **WHEN** a chunk has `finish_reason = "stop"` and `usage = {prompt_tokens: 10, completion_tokens: 2, total_tokens: 12}`
- **THEN** a `StreamEvent{Type: StreamEventTypeFinish, StopReason: "stop", Usage: &Usage{...}}` SHALL be emitted

### Scenario: Finish chunk without usage

- **WHEN** a chunk has `finish_reason = "stop"` and `usage` is nil
- **THEN** a `StreamEvent{Type: StreamEventTypeFinish, StopReason: "stop", Usage: nil}` SHALL be emitted

### Scenario: Usage on a separate chunk after finish

- **WHEN** a chunk with `finish_reason = "stop"` is followed by a chunk with `usage` set but no `finish_reason` and no `content`
- **THEN** the usage SHALL be attached to the Finish event if not yet sent, otherwise emitted as a standalone event with `Usage` populated

## Requirement: Streaming goroutine owns response body lifecycle

The system SHALL launch a background goroutine from `ChatStream` that owns the HTTP response body. The goroutine SHALL read SSE events, send `StreamEvent`s on the channel, and close both the response body and the channel when done. The channel SHALL be unbuffered.

### Scenario: Normal completion

- **WHEN** the stream is fully consumed (all chunks read, `[DONE]` received)
- **THEN** the goroutine SHALL close the response body and close the channel

### Scenario: Channel closes after last event

- **WHEN** the consumer is ranging over the channel
- **THEN** the range loop SHALL terminate when the goroutine closes the channel after stream completion

## Requirement: Error events on parse or stream failure

The system SHALL send a `StreamEvent{Type: StreamEventTypeError, Err: err}` on the channel when JSON parsing of a `data:` token fails or when the underlying `io.Reader` returns an error (other than `io.EOF`). After sending the error event, the goroutine SHALL close the channel.

### Scenario: Malformed JSON in data line

- **WHEN** a `data:` token contains invalid JSON (e.g., `data: {broken`)
- **THEN** a `StreamEvent{Type: StreamEventTypeError, Err: <json error>}` SHALL be sent, and the channel SHALL be closed

### Scenario: Network error mid-stream

- **WHEN** the response body read returns a non-EOF error (e.g., connection reset)
- **THEN** a `StreamEvent{Type: StreamEventTypeError, Err: <read error>}` SHALL be sent, and the channel SHALL be closed

### Scenario: Scanner error after partial reads

- **WHEN** `scanner.Err()` returns a non-nil error after the scan loop exits
- **THEN** a `StreamEvent{Type: StreamEventTypeError, Err: <scanner error>}` SHALL be sent before closing the channel

## Requirement: Context cancellation stops streaming

The system SHALL monitor `context.Context` cancellation during streaming. When `ctx.Done()` fires, the goroutine SHALL stop reading from the response body, close the body (to abort the HTTP connection), and close the channel. The goroutine SHALL NOT send an error event on cancellation — channel closure signals termination.

### Scenario: Context cancelled mid-stream

- **WHEN** the context is cancelled while the goroutine is blocked sending an event or reading from the body
- **THEN** the goroutine SHALL close the response body, close the channel, and return without sending an error event

### Scenario: Consumer stops reading and cancels context

- **WHEN** the consumer stops ranging over the channel and calls `ctx.Cancel()`
- **THEN** the goroutine SHALL unblock, close the body, close the channel, and exit without leaking

## Requirement: No external SSE dependency

The system SHALL implement SSE parsing using only the Go standard library (`bufio`, `bytes`, `context`, `encoding/json`, `io`, `net/http`). No third-party SSE client library SHALL be imported.

### Scenario: Dependency check

- **WHEN** `go.mod` is inspected after this change
- **THEN** no SSE-related third-party import (e.g., `r3labs/sse`, `donutloop/sse`) SHALL be present
