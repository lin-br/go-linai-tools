# cmd/extract

> stdin → LLM → structured JSON to stdout. The second of three Phase 1 CLIs (`summarize`, `extract`, `spec-to-code`). Exercises the MP0/MP2/MP3 non-streaming stack: `OpenRouterProvider` (HTTP POST) → `RetryProvider` (exponential backoff) → `ExtractUseCase` wrapping `tools.Extract[ExtractionResult]` (forced tool choice + JSON unmarshal).

---

## What it does

Reads free-form text from stdin, sends it to an LLM with a forced tool choice (the model must call the `extract_structured_data` tool), and writes the extracted structured data as **JSON to stdout** — a single object, not streamed.

The output schema is predefined (Phase 1 — no custom schemas via flags):

```json
{
  "summary": "Meeting with John about budget",
  "entities": [{"name": "John", "type": "person"}],
  "action_items": ["Follow up with John"],
  "dates": ["2024-03-15"],
  "amounts": ["$500"]
}
```

Three flags control behavior:

| Flag      | Default  | Purpose                                                                                  |
|-----------|----------|------------------------------------------------------------------------------------------|
| `-model`  | empty    | Model id (e.g. `anthropic/claude-sonnet-4-20250514`). When empty, resolves via `config.Models.Get()`. |
| `-format` | `json`   | Output format. Only `json` is supported; any other value exits with code 1.             |
| `-pretty` | `true`   | Indent JSON with 2-space (`json.MarshalIndent`). Use `-pretty=false` for compact output. |

Exit codes: `0` on success, `1` on any error (config load, provider failure, model returned no tool call, JSON unmarshal failure, missing model). Errors go to **stderr** — stdout contains only the JSON result, safe for piping into `jq`.

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

The app exits with code 1 if `OPENROUTER_API_KEY` is missing or no model resolves.

### 2. Run from repo root

The config path is hardcoded as `./internal/configs/configs.yaml` (relative). You **must** run from the repo root:

```bash
cd /Users/wesley.silva/Desktop/go-linai-tools
```

---

## How to run

### Basic — extract from piped text

```bash
echo "Meeting with John on 2024-03-15 about $500 budget" | go run ./cmd/extract
```

Output (pretty by default):

```json
{
  "summary": "Meeting with John about budget",
  "entities": [{"name": "John", "type": "person"}],
  "action_items": ["Follow up with John"],
  "dates": ["2024-03-15"],
  "amounts": ["$500"]
}
```

### Extract from a file

```bash
cat notes.txt | go run ./cmd/extract
```

### Pipe into jq

```bash
cat notes.txt | go run ./cmd/extract | jq '.entities[] | .name'
```

### Override the model

```bash
cat notes.txt | go run ./cmd/extract -model "openai/gpt-4o"
```

### Compact JSON (no indentation)

```bash
cat notes.txt | go run ./cmd/extract -pretty=false
```

### Build a binary

```bash
go build -o extract ./cmd/extract
./extract < notes.txt
```

---

## How to test the error paths

### Invalid API key → stderr + exit 1

```bash
OPENROUTER_API_KEY="invalid" echo "text" | go run ./cmd/extract
# stderr: Extraction failed: provider error: HTTP 401: ...
# exit code: 1
```

### Unsupported format → stderr + exit 1

```bash
echo "text" | go run ./cmd/extract -format yaml
# stderr: unsupported format: "yaml" (only "json" is supported)
# exit code: 1
```

### Model returns no tool call → stderr + exit 1

Some models don't support forced tool choice and return plain text instead. The CLI detects this:

```bash
cat notes.txt | go run ./cmd/extract -model "some-model-without-tool-support"
# stderr: Model did not return structured data.
# exit code: 1
```

### Ctrl+C mid-request → graceful exit

```bash
cat long-doc.txt | go run ./cmd/extract
# while waiting for the provider response, press Ctrl+C
# → context cancels, provider request aborts, CLI exits without writing JSON
```

### Missing model → stderr + exit 1

```bash
unset DEFAULT_MODEL
echo "text" | go run ./cmd/extract
# stderr: no model resolved: set -model flag or configure a default model
# exit code: 1
```

---

## How it works (architecture)

```
stdin ──► cmd/extract/main.go ──► ExtractUseCase.Extract ──► tools.Extract[ExtractionResult] ──► RetryProvider.Chat ──► OpenRouterProvider.Chat ──► OpenRouter API
                                      │                          │                               │                      │
                                      │                          │                               │                      └─ HTTP POST /chat/completions (stream: false)
                                      │                          │                               │                      └─ forced tool_choice: {type: "tool", name: "extract_structured_data"}
                                      │                          │                               │
                                      │                          │                               └─ retries transient failures (429/5xx) with backoff; non-retryable (4xx) returned immediately
                                      │                          │
                                      │                          └─ locates ToolCall where Name == "extract_structured_data"
                                      │                          └─ json.Unmarshal(ToolCall.Arguments) → ExtractionResult
                                      │                          └─ ErrNoToolCall / ErrUnmarshalFailed on failure
                                      │
                                      └─ returns *ExtractionResult or error
```

**Wiring order** (in `main.go`):
1. `configs.LoadConfigs()` — YAML + env vars
2. `clients.NewOpenRouterProvider(apiKey)` — raw HTTP client (MP0)
3. `retry.NewRetryProvider(inner)` — decorator with exponential backoff + jitter (MP2)
4. `usecases.NewExtractUseCase(retryProvider)` — builds the tool schema via `buildExtractSchema()` (hand-built JSON Schema with full array item types)
5. `signal.NotifyContext(ctx, os.Interrupt)` — Ctrl+C cancels through the whole chain

**Error mapping** (in `handleError`):

| Error (`errors.Is`)              | User-facing output                              | Exit code |
|----------------------------------|-------------------------------------------------|-----------|
| `tools.ErrNoToolCall`            | `Model did not return structured data.`         | 1         |
| `tools.ErrUnmarshalFailed`       | `Failed to parse structured data: <err>`        | 1         |
| Provider error (retry-exhausted) | `Extraction failed: <err>`                      | 1         |
| stdin read error                 | `Failed to read input: <err>`                   | 1         |

No partial JSON is written to stdout on any error path.

**Key design decisions** (full rationale in `openspec/changes/mp5-cli-extract/design.md`):
- **D1**: Non-streaming `Chat` (not `ChatStream`). Structured extraction needs the complete tool-call response to unmarshal the arguments — there's nothing to stream incrementally.
- **D3**: Schema is hand-built (not generated via `tools.SchemaFromStruct` from MP3). `SchemaFromStruct` is deliberately shallow — `[]Entity` becomes `{type: "array"}` with no item types, leaving the model unable to infer what the arrays should contain. The hand-built schema describes array items explicitly: `entities` as objects with `name`+`type`, the rest as arrays of strings. The `ExtractionResult` struct is still used for JSON marshaling and the Go return type.
- **D4**: `ExtractUseCase` owns the `tools.Extract[T]` orchestration; the CLI stays thin (flag parsing, I/O, error mapping). Testable with a fake provider.
- **D5**: `flag` package only — no `cobra`/`urfave`. Three flags don't justify a dependency.
- **D6**: Typed errors (`ErrNoToolCall`, `ErrUnmarshalFailed`) mapped to clear user-facing messages. All errors to stderr so stdout stays clean for `jq`.
- **D7**: `signal.NotifyContext` for Ctrl+C — context propagates through the use case → `tools.Extract` → `RetryProvider` → `OpenRouterProvider`, aborting the in-flight HTTP request immediately.
- **D8**: `ExtractionResult` lives in `internal/core/usecases/`, not `internal/core/domain/` — it's a use-case-specific output type, not a shared domain primitive.

**Limitation**: The schema is hand-built rather than generated via `SchemaFromStruct` (MP3). `SchemaFromStruct` produces shallow array schemas (`{type: "array"}` with no item types) — models can't infer what the arrays should contain and return empty results. The hand-built schema in `buildExtractSchema()` describes items explicitly. If the `ExtractionResult` struct changes, update the schema in `extract.go` to match.

---

## Files

| File | Role |
|------|------|
| `cmd/extract/main.go` | CLI entry point — flag parsing, stdin read, wiring, JSON output, error mapping, signal handling, exit codes |
| `internal/core/usecases/extract.go` | `ExtractUseCase` struct, `NewExtractUseCase` constructor, `DefaultExtractSystemPrompt`, `Extract` method |
| `internal/core/usecases/extract_schema.go` | `ExtractionResult` and `Entity` structs (the predefined Phase 1 extraction schema) |
| `internal/core/usecases/extract_test.go` | 6 unit tests — success path, error paths (table-driven), schema reuse, context propagation, system prompt |

---

## Verification

```bash
go build ./...                              # compile all packages
go vet ./...                                # no warnings
go test ./internal/core/usecases/...        # 6 extract tests pass (plus 6 summarize tests)
```

Manual smoke tests (require live API key) — see "How to test the error paths" above.

---

## What's next

This CLI closes Phase 1 deliverable #2 of 3. MP6 (`spec-to-code`) is the remaining CLI — it will follow the same `Extract[T]` pattern but with a code-generation schema instead of `ExtractionResult`.
