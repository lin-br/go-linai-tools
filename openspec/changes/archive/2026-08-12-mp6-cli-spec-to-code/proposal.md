## Why

Phase 1 deliverable #6 is a `spec-to-code` CLI: given a natural-language feature description, produce a structured code plan — the files to create, the types to define, and the functions to implement (signatures only, no implementation). MP0–MP3 built the foundation (`Provider`, `RetryProvider`, `Extract[T]`, `SchemaFromStruct`), but nothing yet exercises that stack as an end-user-facing tool. MP6 closes Phase 1 with the final CLI: it composes `RetryProvider` + `Extract[CodePlan]` behind a thin `flag`-based entry point, proving the whole pipeline works on a realistic, deeply-nested structured output.

## What Changes

- Add a `CodePlan` domain aggregate with nested `FilePlan`, `TypeDecl`, `FieldDecl`, and `FuncDecl` structs — the typed output the LLM MUST produce via forced tool choice.
- Add a `SpecToCodeUseCase` in `internal/core/usecases/` that wraps `tools.Extract[CodePlan]`: builds a `ToolSchema` for `CodePlan`, calls `Extract` against a `Provider` (already wrapped with `RetryProvider`), and returns a typed `*CodePlan`.
- Add `cmd/spec-to-code/main.go` — a standalone CLI that reads a feature description from stdin, invokes the use case, and renders the plan in two formats: JSON (default, pipeable) or a human-readable ASCII tree (`-format text`).
- Add flags: `-model` (override config default), `-format` (`json|text`, default `json`), `-lang` (target language hint, default `go`).
- Add context cancellation via `signal.NotifyContext` so Ctrl+C aborts the in-flight LLM call.
- System prompt positions the model as a software architect that emits plans, not code.
- **Cross-cutting dependency on MP3**: `CodePlan` is a deeply-nested struct (slices of structs containing slices). If MP3's `SchemaFromStruct` only handles flat structs (as its design explicitly accepts as a limitation), MP6 MUST either extend `SchemaFromStruct` to recurse into nested structs and array items, or supply a hand-built `map[string]any` schema for `CodePlan`. This decision is resolved in the design.

## Capabilities

### New Capabilities

- `cli-spec-to-code`: The `spec-to-code` CLI command — its flags, stdin/stdout contract, the `CodePlan`/`FilePlan`/`TypeDecl`/`FieldDecl`/`FuncDecl` struct hierarchy, the `SpecToCodeUseCase` orchestration, JSON and ASCII-tree output rendering, and the system prompt governing model behavior.

### Modified Capabilities

(No existing specs in `openspec/specs/` to modify — MP0–MP3 have not been archived yet. MP6 consumes the MP0 `Provider` interface + config, MP2 `RetryProvider`, and MP3 `Extract[T]`/`ToolSchema`/`SchemaFromStruct` as-is. If `SchemaFromStruct` must be extended to handle nested structs, that extension is captured as a requirement inside the `cli-spec-to-code` spec and coordinated with MP3 during implementation, since neither is implemented yet.)

## Impact

- **New files**:
  - `internal/core/domain/codeplan.go` — `CodePlan`, `FilePlan`, `TypeDecl`, `FieldDecl`, `FuncDecl` structs (plain data, no behavior).
  - `internal/core/usecases/spec_to_code.go` — `SpecToCodeUseCase` + `NewSpecToCodeUseCase(provider outbound.Provider, config)`.
  - `cmd/spec-to-code/main.go` — CLI entry point: config load, provider wiring, flag parsing, stdin read, use-case call, output rendering, signal handling.
  - `cmd/spec-to-code/output.go` (or inline in `main.go`) — `renderJSON` and `renderTree` formatters.
- **MP3 dependency**: MAY require extending `internal/core/tools/schema.go` `SchemaFromStruct` to recurse into nested structs and `[]Struct` slices. If MP3 is implemented with flat-only support, MP6 adds the recursive path; otherwise MP6 uses `SchemaFromStruct(&CodePlan{})` directly.
- **No new external dependencies** — `flag`, `os`, `io`, `context`, `signal`, `encoding/json`, and the MP0–MP3 packages only. No `spf13/cobra` or CLI frameworks, per the roadmap constraint.
- **No breaking changes** — purely additive. Existing `main.go` and `cmd/cli/main.go` are untouched.
- **Closes Phase 1** — MP6 is the final microphase; the Phase 1 LinkedIn post follows after verification.
