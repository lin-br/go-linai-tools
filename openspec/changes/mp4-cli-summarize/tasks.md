## 1. SummarizeUseCase — Structure and System Prompt

- [ ] 1.1 Create `internal/core/usecases/summarize.go` — define `SummarizeUseCase` struct holding an `outbound.Provider`
- [ ] 1.2 Define `NewSummarizeUseCase(provider outbound.Provider) *SummarizeUseCase` constructor
- [ ] 1.3 Define package-level constant `DefaultSummarizeSystemPrompt` — focused summarization directive (concise, key points, decisions, action items, no preamble)
- [ ] 1.4 Verify the package imports `domain`, `outbound`, `context`, `io`, `bufio` without import cycles

## 2. SummarizeUseCase.Stream — Request Building

- [ ] 2.1 Implement `Stream(ctx context.Context, model, systemPrompt, input string, out io.Writer) error`
- [ ] 2.2 Construct `*domain.ChatRequest` with `Model`, `System`, `Stream: true`, single `Messages` entry (`Role: "user"`, `Content: input`), and zero-valued optional fields (`Tools`, `ToolChoice`, `MaxTokens`, `Temperature`, `TopP`)
- [ ] 2.3 Call `provider.ChatStream(ctx, req)` and handle the `(nil, err)` initial-connection case — return the error immediately without writing to `out`

## 3. SummarizeUseCase.Stream — Channel Consumption

- [ ] 3.1 Range over the `<-chan domain.StreamEvent` returned by `ChatStream`
- [ ] 3.2 For `StreamEventTypeText` events with non-empty `Delta`, write `Delta` to a `bufio.Writer` wrapping `out` and call `Flush()` after each write
- [ ] 3.3 Skip `StreamEventTypeText` events with empty `Delta` (write nothing)
- [ ] 3.4 Skip `StreamEventTypeFinish` events (write nothing, continue ranging)
- [ ] 3.5 On `StreamEventTypeError`, return the event's `Err` — do not continue ranging; already-flushed deltas remain on `out`
- [ ] 3.6 Return `nil` after the channel closes with no error events encountered

## 4. SummarizeUseCase.Stream — Context Propagation

- [ ] 4.1 Pass `ctx` to `provider.ChatStream` unmodified
- [ ] 4.2 Verify that context cancellation causes the provider to close the channel (per MP1 contract) and that `Stream` returns without writing further deltas

## 5. Summarize CLI — Entry Point and Flags

- [ ] 5.1 Create `cmd/summarize/main.go` with `package main` and `func main()`
- [ ] 5.2 Define `-model string` flag (default empty) and `-system string` flag (default empty) using the `flag` package
- [ ] 5.3 Call `flag.Parse()`

## 6. Summarize CLI — Config and Wiring

- [ ] 6.1 Load config via `configs.LoadConfigs()`; on error, print to stderr and `os.Exit(1)`
- [ ] 6.2 Construct `OpenRouterProvider` from config (MP0)
- [ ] 6.3 Wrap provider in `retry.NewRetryProvider(provider)` (MP2)
- [ ] 6.4 Construct `usecases.NewSummarizeUseCase(retryProvider)`

## 7. Summarize CLI — Model and System Prompt Resolution

- [ ] 7.1 If `-model` flag is set, use it; otherwise resolve via `config.Models.Get()`
- [ ] 7.2 If no model resolved (flag empty and `Models.Get()` returns nil), print error to stderr and exit 1
- [ ] 7.3 If `-system` flag is set, use it; otherwise use `usecases.DefaultSummarizeSystemPrompt`

## 8. Summarize CLI — stdin, Context, and Streaming

- [ ] 8.1 Read all of stdin via `io.ReadAll(os.Stdin)` into a string
- [ ] 8.2 Create context via `signal.NotifyContext(context.Background(), os.Interrupt)` with `defer stop()`
- [ ] 8.3 Call `useCase.Stream(ctx, model, systemPrompt, input, os.Stdout)`
- [ ] 8.4 On error return from `Stream`, print the error to `os.Stderr` and `os.Exit(1)`
- [ ] 8.5 On success, `os.Exit(0)`

## 9. Unit Tests — SummarizeUseCase

- [ ] 9.1 Create `internal/core/usecases/summarize_test.go`
- [ ] 9.2 Test request building — fake `Provider` captures the `ChatRequest`; assert `Stream: true`, `Model`, `System`, single user message with `Content` = input
- [ ] 9.3 Test delta writing — fake `Provider` emits `Text` events `"Hello"` then `" world"`; assert `bytes.Buffer` output equals `"Hello world"` and deltas appear in order
- [ ] 9.4 Test empty delta skipped — fake `Provider` emits `Text` with `Delta: ""`; assert nothing written
- [ ] 9.5 Test `Finish` event writes nothing — fake `Provider` emits `Text` then `Finish`; assert only the text delta appears in output
- [ ] 9.6 Test initial connection error — fake `Provider.ChatStream` returns `(nil, err)`; assert `Stream` returns the error and writer is empty
- [ ] 9.7 Test mid-stream error — fake `Provider` emits `Text: "Summar"` then `Error: err`; assert `"Summar"` in writer and `Stream` returns `err`
- [ ] 9.8 Test context cancellation — fake `Provider` emits events slowly; cancel context mid-stream; assert `Stream` returns without deadlock

## 10. Verification

- [ ] 10.1 Run `go build ./...` — all packages compile including `cmd/summarize`
- [ ] 10.2 Run `go vet ./...` — no warnings
- [ ] 10.3 Run `go test ./internal/core/usecases/...` — all summarize tests pass
- [ ] 10.4 Manually run `echo "Paste a long text here" | go run ./cmd/summarize` against OpenRouter with a real API key — verify tokens stream to stdout
- [ ] 10.5 Manually test `-model` override — run with `-model "openai/gpt-4o-mini"` and verify a different model responds
- [ ] 10.6 Manually test `-system` override — run with `-system "One-line TLDR"` and verify shorter output
- [ ] 10.7 Manually test Ctrl+C mid-stream — verify graceful exit, no panic, no leaked goroutine
- [ ] 10.8 Manually test error path — run with an invalid API key and verify stderr message + exit code 1
