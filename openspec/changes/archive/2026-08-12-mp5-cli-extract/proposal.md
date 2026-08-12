## Why

Phase 1 deliverable #2 requires `cmd/extract/` — "free-form text → structured data via tool use." MP3 shipped the generic `Extract[T]` helper, `SchemaFromStruct`, and forced-tool-choice request building, but nothing in the codebase consumes them yet. Without a dedicated CLI, the structured-output capability is a library function with no user-facing surface. MP5 closes that gap: a thin, pipeable CLI that reads free-form text from stdin, extracts structured entities via `Extract[T]`, and writes indented JSON to stdout — `cat input.txt | extract | jq .`.

## What Changes

- Add `cmd/extract/main.go` — a CLI entry point that reads stdin, calls an extraction use case, marshals the result to indented JSON, and writes to stdout.
- Add an `ExtractionResult` struct (the predefined extraction schema for Phase 1): `Summary string`, `Entities []Entity` (`Entity{Name, Type}`), `ActionItems []string`, `Dates []string`, `Amounts []string`.
- Add `ExtractUseCase` in `internal/core/usecases/` — takes a `Provider` (wrapped with `RetryProvider`), uses `tools.Extract[T]` to get structured data, returns the typed result. The CLI is thin; the use case owns the orchestration.
- Generate the tool schema from `ExtractionResult` via `tools.SchemaFromStruct` (from MP3) — no hand-built schema for Phase 1.
- Add flags: `-model` to override the config default model, `-format` to choose output format (json, default), `-pretty` for indented JSON (default true).
- Add `signal.NotifyContext` for Ctrl+C handling, propagated through the use case to the provider.
- Add clear error handling: `ErrNoToolCall` → "Model did not return structured data."; `ErrUnmarshalFailed` → print raw arguments for debugging; exit 1 on any error.
- Use non-streaming `Chat` (via `Extract[T]`) — the full tool-call response is needed to parse JSON once. No dependency on MP1 (streaming).
- No external CLI framework — `flag` package only.

## Capabilities

### New Capabilities

- `cli-extract`: The `extract` CLI command — stdin/stdout behavior, flags (`-model`, `-format`, `-pretty`), JSON output, predefined `ExtractionResult` schema, and the `ExtractUseCase` that wraps `tools.Extract[T]`.

### Modified Capabilities

(No existing specs are modified. MP5 consumes the MP0 `Provider` interface, MP2 `RetryProvider`, and MP3 `Extract[T]`/`SchemaFromStruct`/`ToolSchema` as-is — it adds a new CLI capability on top of them.)

## Impact

- **New files**:
  - `cmd/extract/main.go` — CLI entry point (flag parsing, stdin read, signal context, use case wiring, JSON output, error handling).
  - `internal/core/usecases/extract.go` — `ExtractUseCase` struct and `NewExtractUseCase(provider)` constructor.
  - `internal/core/usecases/extract_schema.go` — `ExtractionResult` and `Entity` structs (the predefined Phase 1 extraction schema).
- **No changes** to `internal/core/domain/`, `internal/core/ports/`, `internal/core/tools/`, or `internal/adapters/` — MP5 is a consumer of MP0/MP2/MP3, not a modifier.
- **No new external dependencies** — `flag`, `os`, `io`, `encoding/json`, `context`, `os/signal`, and the MP0–MP3 packages only.
- **No breaking changes** — purely additive (a new `cmd/extract` binary and a new use case).
- **Enables** the Phase 1 close: `extract` is deliverable #2 of 3 CLIs. MP6 (`spec-to-code`) is the remaining CLI.
