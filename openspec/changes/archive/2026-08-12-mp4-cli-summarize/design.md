## Context

MP0–MP3 built the streaming pipeline in layers: `Provider` interface with `ChatStream` returning `<-chan StreamEvent`, `OpenRouterProvider` with hand-rolled SSE parsing, and `RetryProvider` wrapping the initial connection with exponential backoff. None of it has a user-facing entry point yet. The existing CLIs (`main.go`, `cmd/cli/main.go`) use the old `DoSendMessageUseCase` + `ProviderModelHandler` path — non-streaming, no context, no retry. MP4 introduces the first CLI that exercises the new streaming stack end-to-end.

Phase 1 deliverable #2 is explicit: "stdin → Claude → streamed summary to stdout." The `summarize` CLI reads arbitrary text from stdin, sends it to the LLM with a summarization system prompt, and prints the response as tokens arrive — not after the full response completes. This is the first of three CLIs (MP4 `summarize`, MP5 `extract`, MP6 `spec-to-code`) and establishes the wiring + use case pattern the others will follow.

## Goals / Non-Goals

**Goals:**
- Build `cmd/summarize/main.go` — a thin CLI that wires config → provider → retry → use case and streams the LLM response to stdout.
- Build `SummarizeUseCase` in `internal/core/usecases/` — builds a streaming `ChatRequest`, calls `provider.ChatStream`, and writes `StreamEvent` deltas to an `io.Writer`.
- Stream text deltas to stdout immediately (no full-response buffering) using `bufio.Writer` + `Flush()` per delta.
- Support `signal.NotifyContext` so Ctrl+C cancels the stream through the provider and retry layers.
- Support `-model` and `-system` flags to override config defaults at the CLI layer.
- Keep the CLI thin — flag parsing, stdin reading, wiring, and stdout writing only. All LLM logic lives in the use case.

**Non-Goals:**
- Non-streaming summarization — MP4 is streaming-only. The `extract` and `spec-to-code` CLIs (MP5/MP6) use non-streaming `Chat` + `Extract[T]`.
- Tool use / structured outputs — summarization is text-only. No dependency on MP3.
- Prompt engineering tuning — the default system prompt is a single focused directive, not a multi-shot template. Iterating on prompt quality is implementation-time, not spec-time.
- File input — stdin only. No `-file` flag. Users pipe with `cat file.txt | summarize`.
- Token counting / cost display — `StreamEvent.Finish` carries `Usage`, but MP4 does not print it. Future enhancement.
- Multi-turn conversation — single-shot summarize. No history, no follow-up.
- Configurable config path — reuses the hardcoded `./internal/configs/configs.yaml` path from MP0.

## Decisions

### D1: SummarizeUseCase owns the streaming loop, CLI owns I/O

The use case takes a `Provider` and an `io.Writer`. It builds the `ChatRequest`, calls `provider.ChatStream(ctx, req)`, ranges over the event channel, and writes text deltas to the writer. The CLI (`cmd/summarize/main.go`) is responsible only for: flag parsing, stdin reading, wiring construction, passing `os.Stdout` as the writer, and exit codes.

**Why:** Hexagonal separation. The use case is testable without touching the filesystem — inject a `bytes.Buffer` as the writer, inject a fake `Provider` that emits canned `StreamEvent`s. The CLI is a driving adapter with no logic beyond orchestration. This mirrors how `DoSendMessageUseCase` + `driving.CLI` already work, but with streaming.

**Alternative considered:** Put the streaming loop in the CLI adapter directly. Rejected — the loop (channel range, delta extraction, error event handling) is reusable logic that belongs in the use case. The CLI would become a second place with LLM awareness, breaking the hexagonal boundary.

### D2: Stream deltas immediately via bufio.Writer + Flush per delta

The use case wraps the `io.Writer` in a `bufio.Writer` and calls `Flush()` after writing each `StreamEvent.Delta`. This ensures tokens appear on stdout as they arrive, not buffered until completion.

**Why:** The entire point of streaming is live display. If we buffer the full response, we might as well use non-streaming `Chat`. `bufio.Writer` gives efficient writes (avoids a syscall per token) while `Flush()` guarantees visibility. `os.Stdout` is line-buffered by default in terminals, but explicit `Flush()` after each delta removes ambiguity across piped and interactive contexts.

**Alternative considered:** Write directly to `os.Stdout` without buffering. Rejected — one syscall per token delta is wasteful for fast streams, and the use case should not depend on `os.Stdout` specifically (it takes an `io.Writer`). The `bufio.Writer` wrapping is internal to the use case.

### D3: System prompt as a package-level constant

```go
const DefaultSummarizeSystemPrompt = "Summarize the following text concisely. Capture key points, decisions, and action items. Be direct — no preamble."
```

**Why:** A constant is the simplest thing that works for Phase 1. It's visible in the source, easy to iterate on, and overridable via the `-system` flag without config changes. Storing it in `configs.yaml` would add a config field that needs env-var expansion and validation for a string that rarely changes.

**Alternative considered:** Store in `configs.yaml` under a `prompts.summarize` key. Rejected for Phase 1 — config is for environment-specific values (API keys, model names). The prompt is a product decision, not a deployment variable. If A/B testing prompts becomes a goal later, move it to config then.

### D4: flag package, not cobra/urfave

Use the standard `flag` package for `-model` and `-system`. No subcommands, no help templating, no shell completion.

**Why:** The roadmap's philosophy is "learn the wire protocol" — `net/http` over SDKs. The same minimalism applies to CLI: `flag` over frameworks. Two flags do not justify a dependency. The `summarize` command is a single binary (`go run ./cmd/summarize`), not a subcommand of a parent CLI. When MP5/MP6 land, each gets its own `cmd/<name>/main.go` — same pattern.

**Alternative considered:** cobra for future subcommand consolidation. Rejected — premature. Three independent binaries is simpler than one binary with a command tree for a learning project. Revisit in Phase 3 if the agent loop needs a unified CLI.

### D5: signal.NotifyContext for graceful Ctrl+C

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
defer stop()
```

The context is passed to `useCase.Stream(ctx, ...)`, which passes it to `provider.ChatStream(ctx, req)`. When Ctrl+C fires, `ctx.Done()` closes, the MP1 streaming goroutine detects cancellation, closes the response body and channel, and the use case's range loop exits cleanly.

**Why:** This is the idiomatic Go pattern for signal handling (Go 1.16+). It threads cancellation through the existing `context.Context` propagation that MP0 established — no special signal-handling code in the provider or use case. The retry wrapper (MP2) also respects context cancellation during backoff waits, so Ctrl+C during a retry backoff aborts immediately.

**Alternative considered:** `signal.Notify` channel + manual select. Rejected — `signal.NotifyContext` is the modern replacement, cleaner and composable with `context.WithTimeout` if a deadline is needed later.

### D6: Two distinct error paths

1. **Initial connection error**: `provider.ChatStream` returns `(nil, err)`. The use case returns the error. The CLI prints it to stderr and exits 1.
2. **Mid-stream error event**: `ChatStream` returns `(ch, nil)` but a `StreamEvent{Type: Error, Err: ...}` arrives on the channel. The use case writes any already-received deltas (they're already on stdout), then returns the error. The CLI prints it to stderr and exits 1.

**Why:** These are fundamentally different failure modes. The first means nothing was streamed — the request never started (bad auth, model not found, retried-out connection). The second means partial output was delivered before failure (network drop mid-stream, malformed SSE). The CLI must not print a Go error object to stdout (it would corrupt the summary output), so errors go to stderr. Exit code 1 signals failure to shell pipelines.

**Partial output on mid-stream error:** Already-written deltas are not "rolled back" — they're on stdout. This is correct for streaming: the user saw tokens, then the stream broke. The error message on stderr explains why. No re-attempt at the CLI level (MP2's retry only covers initial connection, not mid-stream).

### D7: Model resolution — config default with flag override

The CLI resolves the model via `config.Models.Get()` (same as existing code). If `-model` is set, it overrides the config value. The resolved model is passed to the use case, which sets it on `ChatRequest.Model`.

**Why:** Config holds the environment default (set via `DEFAULT_MODEL` env var). The flag allows one-off overrides without editing config or env — useful for testing different models on the same input. The use case does not resolve the model itself — it receives the resolved model string, keeping it config-agnostic and testable.

### D8: stdin read strategy — read all, then stream

The CLI reads all of stdin into a string (via `io.ReadAll(os.Stdin)`), then passes it to the use case as the user message content. It does not stream stdin line-by-line into the request.

**Why:** The `ChatRequest` needs the full input text as a single user message before calling `ChatStream`. The LLM summarizes the complete text, not line-by-line. Reading all of stdin first is simple and correct for the summarization use case. stdin is typically small (a paste, a file piped in). For very large inputs, a `MaxTokens` limit on the request would matter, but that's a future concern.

**Alternative considered:** Stream stdin line-by-line and accumulate. Rejected — adds complexity for no benefit. The request is built once and sent once; there's no incremental request API.

## Risks / Trade-offs

- **[Large stdin could exceed model context window]** → The CLI reads all of stdin. If the input is larger than the model's context window, the API returns a 400 (non-retryable, per MP2). Mitigation: the error surfaces clearly on stderr. Chunking large inputs is a future enhancement, out of scope for Phase 1.
- **[No partial-output rollback on mid-stream error]** → Deltas already written to stdout stay there. A shell pipeline consuming the output sees valid text followed by an error on stderr and exit 1. Mitigation: documented behavior — streaming is fire-and-forget for displayed tokens. The caller can detect failure via exit code.
- **[bufio.Writer Flush on every delta]** → One `Flush()` per token delta means a syscall per token on piped stdout. For interactive terminal use, this is fine (terminals are fast enough). For high-throughput piped scenarios, it's slightly wasteful. Mitigation: acceptable for Phase 1 — the bottleneck is the LLM stream latency, not the syscall. If profiling shows otherwise, batch flushes.
- **[Config path hardcoded]** → `./internal/configs/configs.yaml` is a relative path. Running `summarize` from a different working directory fails. Mitigation: inherited from MP0, not introduced here. Documented as a known limitation. Future: add `-config` flag or env-based path resolution.
- **[Exit code only signals success/failure, not error type]** → Exit 1 for all errors (connection, mid-stream, config load). No granular exit codes (e.g., 2 for auth, 3 for rate limit). Mitigation: the error message on stderr carries the detail. Granular exit codes are over-engineering for a learning CLI.
