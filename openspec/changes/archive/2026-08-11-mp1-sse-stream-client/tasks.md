## 1. SSE SplitFunc

- [x] 1.1 Create `internal/adapters/driven/http_clients/sse_stream.go` — package declaration and imports (`bufio`, `bytes`, `context`, `encoding/json`, `fmt`, `io`, `net/http`)
- [x] 1.2 Implement `sseSplitFunc(data []byte, atEOF bool) (advance int, token []byte, err error)` — split on `\n\n` boundaries, strip `data: ` prefix, skip comment lines (starting with `:`), return `0, nil, nil` for partial data
- [x] 1.3 Implement `newSSEScanner(r io.Reader) *bufio.Scanner` — create scanner, call `scanner.Buffer()` with 64KB initial / 1MB max, call `scanner.Split(sseSplitFunc)`

## 2. Chunk-to-StreamEvent Translation

- [x] 2.1 Implement `translateChunk(data []byte) (*domain.StreamEvent, error)` — unmarshal `data` as `ChatCompletionChunk`, map `delta.content` → Text event, `finish_reason` → Finish event, `usage` → Usage on Finish
- [x] 2.2 Handle empty content delta (role-only first chunk) — return nil event, no emission
- [x] 2.3 Handle `usage` on a separate chunk after finish — stash usage or emit standalone usage event if Finish already sent

## 3. Streaming Goroutine

- [x] 3.1 Implement `streamLoop(ctx context.Context, body io.ReadCloser, ch chan<- domain.StreamEvent)` — `defer close(ch)`, `defer body.Close()`, create SSE scanner, loop `scanner.Scan()`
- [x] 3.2 In the scan loop, use `select` on `ctx.Done()` before sending each event — on cancellation, return without sending an error event
- [x] 3.3 Detect `data: [DONE]` — break the scan loop (do not parse as JSON, do not emit event)
- [x] 3.4 After scan loop, check `scanner.Err()` — if non-nil, send `StreamEvent{Type: Error, Err: err}` before the deferred close fires

## 4. Wire into OpenRouterProvider.ChatStream

- [x] 4.1 Rewrite `OpenRouterProvider.ChatStream` — build wire request with `stream: true`, send HTTP request with `http.NewRequestWithContext(ctx, ...)`, check for non-2xx / connection errors (return nil channel + error)
- [x] 4.2 On success, create unbuffered `chan domain.StreamEvent`, launch `go streamLoop(ctx, resp.Body, ch)`, return `ch, nil`
- [x] 4.3 Remove the MP0 stub (`errors.New("streaming not implemented yet")`)
- [x] 4.4 Ensure `ChatRequest.Stream = true` is set in `toWire()` when `ChatStream` is the entry point (or override locally in `ChatStream`)

## 5. Verification

- [x] 5.1 Run `go build ./...` — all packages compile
- [x] 5.2 Run `go vet ./...` — no warnings
- [x] 5.3 Manually test streaming with `go run .` or `go run ./cmd/cli` — confirm tokens print incrementally (requires a real `OPENROUTER_API_KEY` and a model that supports streaming)
- [x] 5.4 Test context cancellation — start a stream, Ctrl+C or cancel context, confirm the program exits cleanly without goroutine leak
