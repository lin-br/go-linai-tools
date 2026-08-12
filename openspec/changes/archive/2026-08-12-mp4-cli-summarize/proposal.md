## Why

Phase 1 deliverable #2 calls for a `cmd/summarize/` CLI: "stdin → Claude → streamed summary to stdout." MP0–MP3 built the foundation — `Provider` interface with `ChatStream`, `StreamEvent` channel semantics, `OpenRouterProvider` SSE parsing, and a `RetryProvider` decorator — but none of it is wired to a user-facing command yet. This microphase is the first CLI that exercises the streaming pipeline end-to-end: read text from stdin, send it to the LLM with a summarization system prompt, and print the response token-by-token as deltas arrive. No buffering, no full-response waits.

## What Changes

- Add `cmd/summarize/main.go` — a thin CLI entry point that parses flags, reads stdin, constructs the wiring (config → `OpenRouterProvider` → `RetryProvider` → `SummarizeUseCase`), and writes streamed deltas to stdout.
- Add a `SummarizeUseCase` in `internal/core/usecases/` that builds a `ChatRequest` with `Stream: true`, a summarization system prompt, and the input text as a single user message; calls `provider.ChatStream(ctx, req)`; and writes `StreamEvent` text deltas to an `io.Writer` as they arrive.
- Add a default summarization system prompt as a package-level constant — focused, directive, no preamble.
- Wire `signal.NotifyContext(context.Background(), os.Interrupt)` so Ctrl+C cancels the stream gracefully through the provider and retry layers.
- Support `-model` flag to override the config-resolved default model and `-system` flag to override the default system prompt.
- Stream text deltas to stdout immediately using `bufio.Writer` with `Flush()` after each delta — no buffering of the full response.
- Handle two error paths distinctly: `ChatStream` initial-connection error (print to stderr, exit 1) and `StreamEvent{Type: Error}` mid-stream (print to stderr, exit 1).

## Capabilities

### New Capabilities

- `cli-summarize`: The `summarize` CLI command — its flags (`-model`, `-system`), stdin/stdout streaming behavior, the `SummarizeUseCase` orchestration (request building, `ChatStream` consumption, delta-to-writer writing), default system prompt, signal-based context cancellation, and exit-code semantics.

### Modified Capabilities

(No existing specs in `openspec/specs/` are modified — MP0–MP3 have not been archived yet. MP4 consumes the `Provider` interface, `ChatStream`/`StreamEvent` types (MP0/MP1), and `RetryProvider` (MP2) as-is. It adds a new use case and CLI on top of them without changing their contracts.)

## Impact

- **New files**:
  - `cmd/summarize/main.go` — CLI entry point: flag parsing, stdin reading, wiring, stdout streaming, signal handling, exit codes.
  - `internal/core/usecases/summarize.go` — `SummarizeUseCase` struct, `NewSummarizeUseCase(provider, ...)` constructor, `Stream(ctx, input, writer)` method, and the default system prompt constant.
- **No changes** to `internal/core/domain/`, `internal/core/ports/`, `internal/core/tools/`, or `internal/adapters/` — MP4 is a consumer of MP0–MP2, not a modifier.
- **No new external dependencies** — `flag`, `context`, `os`, `os/signal`, `bufio`, `io`, `fmt` from the standard library only. No cobra/urfave CLI framework, per the roadmap's `net/http`-over-SDKs philosophy.
- **No config changes** — model resolution reuses `config.Models.Get()`; the `-model` flag overrides at the CLI layer without touching `configs.yaml`.
- **Does NOT depend on MP3** — summarization is text-only streaming; no structured outputs or tool use involved.
- **Downstream pattern** — establishes the CLI + use case + streaming pattern that MP5 (`extract`) and MP6 (`spec-to-code`) will follow, adapted for non-streaming `Chat` + `Extract[T]`.
