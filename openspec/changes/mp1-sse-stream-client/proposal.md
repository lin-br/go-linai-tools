## Why

MP0 shipped the `Provider` interface with a `ChatStream` method that returns `errors.New("streaming not implemented yet")`. The `summarize` CLI (MP4) needs real streaming to print tokens as they arrive — without it, every response blocks until completion. Phase 1's deliverable list explicitly calls for a `StreamClient` using SSE parsing via `bufio.Scanner` with the `data:` prefix. This microphase replaces the stub with a hand-rolled SSE parser wired into `OpenRouterProvider.ChatStream`.

## What Changes

- Implement an SSE parser using `bufio.Scanner` with a custom `SplitFunc` that splits on `\n\n` boundaries and strips the `data: ` prefix from each event.
- Parse each `data: {json}` line as a `ChatCompletionChunk` (the wire type defined in MP0's `openai_wire.go`).
- Handle the `data: [DONE]` terminator — stop reading and close the channel.
- Ignore comment lines (lines starting with `:`) — OpenRouter sends keepalive comments like `: OPENROUTER PROCESSING`.
- Translate `ChatCompletionChunk` → `domain.StreamEvent`:
  - `choices[].delta.content` → `StreamEvent{Type: Text, Delta: content}`
  - `finish_reason` non-nil → `StreamEvent{Type: Finish, StopReason: finish_reason}`
  - `usage` non-nil (final chunk before `[DONE]`) → set `Usage` on the Finish event
- Wire the SSE parser into `OpenRouterProvider.ChatStream` via a background goroutine that reads the HTTP response body, sends events on a channel, and closes the channel on completion or error.
- Support `context.Context` cancellation — when `ctx.Done()` fires, close the response body and channel.
- Remove the MP0 `ChatStream` stub.

## Capabilities

### New Capabilities

- `sse-stream-client`: The SSE parsing and streaming infrastructure — `bufio.Scanner` custom `SplitFunc`, chunk-to-`StreamEvent` translation, `[DONE]` handling, comment filtering, context cancellation, and the goroutine channel pattern that `OpenRouterProvider.ChatStream` uses.

### Modified Capabilities

- `openrouter-provider`: The `ChatStream` method changes from a stub returning an error to a full implementation that opens a streaming HTTP request and returns a live `<-chan StreamEvent`.

## Impact

- **`internal/adapters/driven/http_clients/`** — new SSE parser file (e.g., `sse_stream.go`), `OpenRouterProvider.ChatStream` rewritten from stub to full implementation.
- **No domain or ports changes** — `StreamEvent`, `StreamEventType`, `Provider.ChatStream` signature, and `ChatCompletionChunk` wire types are all defined in MP0 and remain untouched.
- **No new external dependencies** — `bufio`, `bytes`, `context`, `encoding/json`, `io`, `net/http` from the standard library only. No SSE library.
- **No config changes** — streaming is triggered by `ChatRequest.Stream = true`, already modeled in MP0.
- **Downstream unblock** — enables MP4 (`summarize` CLI with live token printing) and MP2 (retry wrapper can wrap `ChatStream`).
