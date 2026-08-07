## Context

MP0 defined the full type surface: `Provider.ChatStream(ctx, *ChatRequest) (<-chan StreamEvent, error)`, `StreamEvent{Type, Delta, StopReason, Usage, Err}`, `ChatCompletionChunk{ID, Choices[]WireChunkChoice, Usage}`, and `OpenRouterProvider` with a `ChatStream` stub returning `errors.New("streaming not implemented yet")`. The wire types for streaming chunks are already in `openai_wire.go`. No new domain types or interface changes are needed — this microphase fills in the implementation behind the existing contract.

OpenRouter streaming uses the OpenAI Chat Completions SSE format: the HTTP response body is a stream of `data: {json}\n\n` events, each containing a `ChatCompletionChunk`. The stream terminates with `data: [DONE]`. OpenRouter may intersperse keepalive comment lines starting with `:`. The request must set `"stream": true` in the JSON body (the `ChatRequest.Stream` field maps to this).

## Goals / Non-Goals

**Goals:**
- Implement a hand-rolled SSE parser using `bufio.Scanner` with a custom `SplitFunc` — no external SSE library.
- Parse `data: {json}` lines as `ChatCompletionChunk`, translate to `domain.StreamEvent`, send on a channel.
- Handle `data: [DONE]`, ignore comment lines, propagate errors as `StreamEvent{Type: Error}`.
- Wire the parser into `OpenRouterProvider.ChatStream` via a goroutine that owns the response body lifecycle.
- Support `context.Context` cancellation — stop reading, close body, close channel.

**Non-Goals:**
- Retry/backoff for stream connections (MP2).
- Reconnection / resume-from-last-event (not needed for Phase 1 CLIs).
- Tool call streaming (Phase 1 CLIs that use tools call non-streaming `Chat`).
- Streaming for providers other than OpenRouter.
- Unit tests (addressed in implementation, not in the spec).

## Decisions

### D1: bufio.Scanner with custom SplitFunc

Use `bufio.Scanner` with a custom `SplitFunc` that splits the SSE byte stream on `\n\n` boundaries, strips the `data: ` prefix, and returns each event payload as a token.

**Why:** The roadmap explicitly requires "SSE parsing via `bufio.Scanner`, `data:` prefix." A `SplitFunc` is the idiomatic Go way to tokenize a byte stream without buffering the entire body into memory. `bufio.Scanner` handles partial reads transparently — the `SplitFunc` returns `0, nil, nil` when more data is needed, and the scanner fetches more from the underlying `io.Reader`.

**Buffer sizing:** SSE events from OpenRouter are small (a few hundred bytes per chunk), but the scanner's default 64KB max token length could be exceeded by a very long content delta. Call `scanner.Buffer()` with an initial 64KB and a max of 1MB to be safe.

**Alternative considered:** `bufio.Reader.ReadString('\n')` in a loop, accumulating lines until a blank line signals event boundary. Rejected because `Scanner` with `SplitFunc` is cleaner — the tokenization logic is isolated in one function, and the main loop only deals with complete events.

### D2: Goroutine + channel ownership pattern

`ChatStream` makes the HTTP request synchronously (so it can return an error if the connection fails), then launches a goroutine that owns the response body. The goroutine reads SSE events, sends `StreamEvent`s on the channel, and closes the channel when done.

```
ChatStream(ctx, req)
  ├─ build request (stream: true)
  ├─ http.Do(req) ─── on error → return nil, err
  ├─ ch := make(chan StreamEvent)
  ├─ go streamLoop(ctx, resp.Body, ch)
  └─ return ch, nil

streamLoop(ctx, body, ch)
  ├─ defer close(ch)
  ├─ defer body.Close()
  ├─ scanner := newSSEScanner(body)
  ├─ for scanner.Scan() {
  │     select {
  │     case <-ctx.Done(): return
  │     default:
  │       event := translateChunk(scanner.Bytes())
  │       if event != nil { ch <- event }
  │     }
  │  }
  └─ if scanner.Err() != nil → ch <- StreamEvent{Type: Error, Err: ...}
```

**Why:** The `Provider.ChatStream` contract (defined in MP0) says: return a channel immediately, close it when done. The goroutine pattern is the only way to satisfy this — the caller gets a channel they can range over, and the provider manages the body lifecycle.

**Channel is unbuffered:** Events should be delivered as soon as they arrive. An unbuffered channel applies backpressure — if the consumer is slow, the goroutine blocks on send, naturally throttling the reader. This is correct for a CLI that prints tokens to stdout.

### D3: Context cancellation via select on send

The goroutine watches `ctx.Done()` at two points: before sending each event (via `select` on `ctx.Done()` vs `ch <- event`), and can also be checked before scanning. When `ctx` is cancelled, the goroutine returns, `defer close(ch)` fires, and `defer body.Close()` aborts the HTTP connection.

**Why:** `http.NewRequestWithContext(ctx, ...)` already makes the HTTP *request* cancellable, but once the response body is being streamed, the scanner blocking on `Read` needs a way to be interrupted. Closing the body (via `defer`) causes the scanner's `Read` to return an error, unblocking the scan loop. The `select` on send prevents the goroutine from blocking forever on a channel whose consumer has given up.

### D4: Chunk-to-StreamEvent translation rules

Each `data: {json}` line (except `[DONE]`) is unmarshalled into `ChatCompletionChunk`. Translation:

| Wire field | StreamEvent |
|---|---|
| `choices[0].delta.content` non-empty | `{Type: Text, Delta: content}` |
| `choices[0].finish_reason` non-nil | `{Type: Finish, StopReason: finish_reason}` |
| `usage` non-nil (on the chunk with `finish_reason`) | Set `Usage` on the Finish event |
| Parse error | `{Type: Error, Err: err}` |

The first chunk typically has `delta.role` set and empty `content` — it is skipped (no Text event for empty content). The final chunk before `[DONE]` has `finish_reason` set and may carry `usage`. `[DONE]` triggers channel closure without emitting an event (the Finish event was already sent from the chunk with `finish_reason`).

**Edge case — usage on separate chunk:** Some providers send `usage` on a chunk that has no `finish_reason` (a separate usage-only chunk after the finish chunk). If a chunk has `usage` but no `finish_reason` and no `content`, emit nothing but stash the usage so it can be attached to the Finish event. If the Finish event was already sent, emit a standalone event with just `Usage`. In practice, OpenRouter attaches `usage` to the same chunk as `finish_reason`, but the parser handles both orderings.

### D5: No external SSE library

Hand-rolled `bufio.Scanner` `SplitFunc` only. No `r3labs/sse`, no `donutloop/sse`.

**Why:** The roadmap's explicit requirement ("SSE parsing via `bufio.Scanner`") is a learning objective — the point is to understand the wire protocol, not to abstract it away. SSE is simple enough (split on `\n\n`, strip `data: `, handle `[DONE]`) that a library would add a dependency for ~40 lines of code.

## Risks / Trade-offs

- **[Scanner buffer overflow on very long events]** → Mitigated by `scanner.Buffer(buf, 1<<20)` (1MB max). OpenRouter chunks are small; this is defensive.
- **[Goroutine leak if consumer abandons channel without cancelling context]** → Mitigated by the `select` on `ctx.Done()` in the send path. If the caller stops reading and doesn't cancel ctx, the goroutine blocks on send forever. This is a known trade-off of unbuffered channels — the contract says "cancel the context when you stop reading."
- **[No reconnection on mid-stream failure]** → Acceptable for Phase 1 CLIs. The `summarize` CLI re-runs on failure. MP2's retry wrapper can wrap the initial connection, but mid-stream retry requires event sequencing (last-event-id) which OpenRouter doesn't guarantee. Out of scope.
- **[Comment lines between data lines]** → The SplitFunc skips lines starting with `:`. OpenRouter sends `: OPENROUTER PROCESSING` keepalives during long model thinking. Correctly ignored.
