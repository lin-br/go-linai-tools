# cmd/summarize

> stdin → LLM → streamed summary to stdout. The first of three Phase 1 CLIs (`summarize`, `extract`, `spec-to-code`). Exercises the MP0–MP2 streaming stack end-to-end: `OpenRouterProvider` (SSE parsing) → `RetryProvider` (exponential backoff on initial connection) → `SummarizeUseCase` (delta-to-writer loop).

---

## What it does

Reads arbitrary text from stdin, sends it to an LLM with a summarization system prompt, and prints the response **token-by-token** as deltas arrive — not after the full response completes. No buffering of the full response.

Two flags override defaults at the CLI layer without touching config:

| Flag     | Default | Purpose                                                        |
|----------|---------|----------------------------------------------------------------|
| `-model` | empty   | Model id (e.g. `anthropic/claude-sonnet-4-20250514`). When empty, resolves via `config.Models.Get()`. |
| `-system`| empty   | System prompt. When empty, uses `usecases.DefaultSummarizeSystemPrompt`. |

Exit codes: `0` on success, `1` on any error (config load, connection, mid-stream, missing model). Errors go to **stderr** — stdout contains only the streamed summary, safe for piping.

---

## Prerequisites

### 1. Env vars

The config file (`internal/configs/configs.yaml`) is loaded with `os.ExpandEnv`. Required:

```bash
export OPENROUTER_API_KEY="sk-or-v1-..."
export DEFAULT_MODEL="anthropic/claude-sonnet-4-20250514"
# Optional alternatives resolved by config.Models.Get() when DEFAULT_MODEL is empty:
# export PRO_MODEL="..."
# export FREE_MODEL="..."
```

The app `log.Fatal`s if `OPENROUTER_API_KEY` is missing or no model resolves.

### 2. Run from repo root

The config path is hardcoded as `./internal/configs/configs.yaml` (relative). You **must** run from the repo root:

```bash
cd /Users/wesley.silva/Desktop/go-linai-tools
```

---

## How to run

### Basic — summarize piped text

```bash
echo "Paste a long text here..." | go run ./cmd/summarize
```

### Summarize a file

```bash
cat notes.txt | go run ./cmd/summarize
```

### Override the model

```bash
cat notes.txt | go run ./cmd/summarize -model "openai/gpt-4o-mini"
```

### Override the system prompt

```bash
cat notes.txt | go run ./cmd/summarize -system "One-line TLDR"
```

### Combine both overrides

```bash
cat notes.txt | go run ./cmd/summarize -model "openai/gpt-4o-mini" -system "Give me 3 bullet points"
```

### Build a binary

```bash
go build -o summarize ./cmd/summarize
./summarize < notes.txt
```

---

## How to test the error paths

### Invalid API key → stderr + exit 1

```bash
OPENROUTER_API_KEY="invalid" echo "text" | go run ./cmd/summarize
# stderr: 401 unauthorized (or similar)
# exit code: 1
```

### Ctrl+C mid-stream → graceful exit

```bash
cat long-doc.txt | go run ./cmd/summarize
# while tokens are streaming, press Ctrl+C
# → context cancels, provider closes the channel, CLI exits promptly
# → no panic, no leaked goroutine
```

### Missing model → stderr + exit 1

```bash
unset DEFAULT_MODEL
echo "text" | go run ./cmd/summarize
# stderr: no model resolved: set -model flag or configure a default model
# exit code: 1
```

---

## How it works (architecture)

```
stdin ──► cmd/summarize/main.go ──► SummarizeUseCase.Stream ──► RetryProvider.ChatStream ──► OpenRouterProvider.ChatStream ──► OpenRouter SSE API
                                       │                          │                          │
                                       │                          │                          └─ HTTP POST /chat/completions (stream: true)
                                       │                          │                          └─ goroutine parses SSE chunks → StreamEvent channel
                                       │                          │
                                       │                          └─ retries initial connection error (429/5xx) with backoff; mid-stream failures NOT retried
                                       │
                                       └─ ranges over <-chan StreamEvent
                                          ├─ Text + non-empty Delta → bufio.Writer.WriteString + Flush (immediate visibility)
                                          ├─ Text + empty Delta     → skip
                                          ├─ Finish                 → skip (no output)
                                          ├─ Error                  → return event.Err (partial output stays on stdout)
                                          └─ channel closed         → return nil
```

**Wiring order** (in `main.go`):
1. `configs.LoadConfigs()` — YAML + env vars
2. `clients.NewOpenRouterProvider(apiKey)` — raw HTTP client (MP0)
3. `retry.NewRetryProvider(inner)` — decorator with exponential backoff + jitter (MP2)
4. `usecases.NewSummarizeUseCase(retryProvider)` — the streaming loop (MP4)
5. `signal.NotifyContext(ctx, os.Interrupt)` — Ctrl+C cancels through the whole chain

**Key design decisions** (full rationale in `openspec/changes/mp4-cli-summarize/design.md`):
- **D1**: Use case owns the streaming loop; CLI owns only I/O. Testable with `bytes.Buffer` + fake provider.
- **D2**: `bufio.Writer` + `Flush()` per delta — tokens appear immediately, no full-response buffering.
- **D4**: `flag` package, not cobra/urfave. Two flags don't justify a dependency.
- **D6**: Two distinct error paths — initial connection error (nothing streamed) vs. mid-stream error (partial output stays).
- **D8**: Read all of stdin first, then stream. The LLM summarizes the complete text, not line-by-line.

---

## Files

| File | Role |
|------|------|
| `cmd/summarize/main.go` | CLI entry point — flag parsing, stdin read, wiring, stdout streaming, signal handling, exit codes |
| `internal/core/usecases/summarize.go` | `SummarizeUseCase` struct, `NewSummarizeUseCase` constructor, `DefaultSummarizeSystemPrompt` constant, `Stream` method |
| `internal/core/usecases/summarize_test.go` | 8 unit tests — request building, delta ordering, empty-delta skip, finish no-op, connection error, mid-stream error, context cancellation |

---

## Verification

```bash
go build ./...                              # compile all packages
go vet ./...                                # no warnings
go test ./internal/core/usecases/...        # 8 summarize tests pass
```

Manual smoke tests (require live API key) — see "How to test the error paths" above.

---

## What's next

This CLI establishes the pattern that MP5 (`extract`) and MP6 (`spec-to-code`) will follow, adapted for non-streaming `Chat` + structured output extraction instead of streaming.
