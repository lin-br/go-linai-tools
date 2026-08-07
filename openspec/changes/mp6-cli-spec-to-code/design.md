## Context

MP0–MP3 built the provider-agnostic foundation: `Provider` interface with `Chat`, domain types (`ChatRequest`, `ChatResponse`, `ToolCall`), `OpenRouterProvider` using the OpenAI Chat Completions wire format, `RetryProvider` with exponential backoff, and the generic `Extract[T]` extraction helper backed by forced `tool_choice`. Nothing in the codebase yet consumes that stack as an end-user tool — `main.go` and `cmd/cli/main.go` still use the pre-MP0 `OpenRouterClient` + `DoSendMessageUseCase` wiring.

Phase 1 deliverable #6 is the `spec-to-code` CLI: the last of three CLIs (MP4 `summarize`, MP5 `extract`, MP6 `spec-to-code`). MP6 is the most demanding of the three because its output schema is deeply nested — `CodePlan` contains `[]FilePlan`, each `FilePlan` contains `[]TypeDecl` and `[]FuncDecl`, each `TypeDecl` contains `[]FieldDecl`. MP3's `SchemaFromStruct` was explicitly specced as "simple by design" with the documented limitation that "nested structs become `{}` — opaque" and "no array item types beyond `"array"`." That limitation is now on the critical path: `CodePlan` cannot be described by a flat schema.

The project constraint is strict: `net/http` + `encoding/json` + stdlib only, `flag` package for the CLI (no `spf13/cobra`), Go-first. The CLI is non-streaming — it needs the full structured response before rendering.

## Goals / Non-Goals

**Goals:**
- Deliver a working `cmd/spec-to-code/main.go` CLI that reads a feature description from stdin and outputs a structured `CodePlan`.
- Define the `CodePlan` aggregate (`FilePlan`, `TypeDecl`, `FieldDecl`, `FuncDecl`) as plain domain structs with `json` tags.
- Build a `SpecToCodeUseCase` that wraps `tools.Extract[CodePlan]`, keeping the CLI thin.
- Render output in two formats: machine-readable JSON (default, pipeable) and a human-readable ASCII tree (`-format text`).
- Resolve the nested-schema gap: either extend `SchemaFromStruct` to recurse, or hand-build the `CodePlan` schema.
- Wire `signal.NotifyContext` for Ctrl+C cancellation.

**Non-Goals:**
- Code generation — MP6 produces a PLAN (paths, signatures, field types), not implementation code. The `FuncDecl` carries a `Signature` string, not a body.
- Streaming output — `spec-to-code` uses non-streaming `Chat` with forced `tool_choice` (same as MP5 `extract`). It needs the complete structured response.
- Agentic tool-call loops — single-shot extraction only. The agent loop is Phase 3.
- Validation that the plan is "correct" or compileable — the model's output is JSON-valid (guaranteed by tool choice) but semantic correctness is the user's judgment.
- A TUI or interactive mode — stdin in, stdout out, one shot.
- Persisting plans to disk — output goes to stdout; the user redirects with `>` if they want a file.

## Decisions

### D1: Extend SchemaFromStruct to recurse into nested structs and struct slices

`CodePlan` is deeply nested. MP3's `SchemaFromStruct` produces opaque `{}` for nested structs and bare `"array"` for slices. Two options:

- **Option A (extend):** Add recursion to `SchemaFromStruct` — when a field is a struct, recurse and embed the sub-schema; when a field is `[]T` where `T` is a struct, produce `{"type": "array", "items": {<recursed schema for T>}}`.
- **Option B (hand-build):** Construct the `CodePlan` schema as a hand-built `map[string]any` literal, bypassing `SchemaFromStruct` entirely.

**Decision: Option A — extend `SchemaFromStruct`.**

**Why:** Hand-building the `CodePlan` schema (~80 lines of nested map literals) works but defeats the purpose of having `SchemaFromStruct` — every future nested-struct extraction (Phase 2+) would hand-roll again. The recursion is a natural, bounded extension: structs contain structs or slices-of-structs; the reflect walk already visits fields. Teaching the generator to emit `"items"` for struct slices and inline sub-schemas for struct fields is ~40 lines and directly addresses MP3's documented limitation.

**Scope of the extension:** Recurse into struct fields and `[]Struct` slices only. Pointer-to-struct (`*Struct`) is unwrapped to its element type. Slices of scalars (`[]string`) get `{"type": "array", "items": {"type": "string"}}`. Maps, `any`, and custom types remain opaque (`{}`) — out of scope for Phase 1. A cycle guard (visited-set) prevents infinite recursion on self-referential structs, falling back to `{}` for the cyclic field.

**Alternative considered:** Option B (hand-built schema). Faster to ship, but creates a precedent that complex schemas bypass the generator. Phase 2's agent memory structs will be similarly nested; fixing the generator now pays forward.

### D2: CodePlan lives in domain, SpecToCodeUseCase wraps Extract

```
internal/core/domain/codeplan.go     — CodePlan, FilePlan, TypeDecl, FieldDecl, FuncDecl (plain structs)
internal/core/usecases/spec_to_code.go — SpecToCodeUseCase{ provider }, NewSpecToCodeUseCase(...), Plan(ctx, input) (*CodePlan, error)
```

`SpecToCodeUseCase.Plan(ctx, input)` does three things:
1. Builds a `ToolSchema` for `CodePlan` via `SchemaFromStruct(&CodePlan{})` (now recursive).
2. Calls `tools.Extract[CodePlan](ctx, provider, model, systemPrompt, input, schema)`.
3. Returns the `*CodePlan` or propagates the error.

**Why a use case and not a direct `Extract` call in `main.go`:** The use case owns the system prompt and model resolution (config default vs `-model` flag). It also is the seam for future extensions — e.g., enriching the plan with file-existence checks, or caching. Keeping the CLI thin (parse flags, read stdin, call use case, render) matches the hexagonal pattern established by `DoSendMessageUseCase`.

**Why domain and not tools:** `CodePlan` is a data structure specific to this use case's output, not a reusable tool capability. It belongs in `domain` alongside `Request`/`Response`. The `tools` package stays generic — it never imports `CodePlan`.

### D3: Non-streaming, forced tool_choice — same as MP5

`spec-to-code` calls `provider.Chat` (non-streaming) with `ToolChoice{Type: "tool", Name: "generate_code_plan"}`. The model MUST return a single tool call whose `Arguments` JSON-decodes into `CodePlan`.

**Why not streaming:** The output is a structured object, not token-by-token prose. The CLI cannot render a partial `CodePlan` — it needs the full JSON to unmarshal and then format. Streaming would require accumulating the tool-call arguments delta-by-delta (OpenAI streams `tool_calls[].function.arguments` in fragments), which is a Phase 3 concern (agentic loops). For Phase 1, the non-streaming path is simpler and already proven by `Extract[T]`.

### D4: Two output formats — JSON default, ASCII tree optional

**JSON mode (`-format json`, default):** `json.MarshalIndent(plan, "", "  ")` to stdout. Pipeable — `spec-to-code < feature.txt | jq '.files[].path'`. Exit 0.

**Text mode (`-format text`):** A human-readable ASCII tree using `├──`, `└──`, and `│` connectors. No emojis (project convention). Example:

```
Code Plan: User Authentication
Language: go

internal/core/domain/user.go
  Types:
    User
      ID: string
      Email: string
      PasswordHash: string
  Functions:
    func NewUser(email, password string) (*User, error)

internal/core/usecases/auth.go
  Functions:
    func Login(ctx context.Context, email, password string) (string, error)
    func Register(ctx context.Context, req *RegisterReq) (*User, error)
```

**Why ASCII tree over JSON-only:** The JSON output is for piping; the tree is for a human reading the plan in a terminal. The tree uses indentation (2 spaces per level) and section labels (`Types:`, `Functions:`) rather than heavy box-drawing — simple to implement with `fmt.Fprintf` and a depth counter.

**Why JSON is the default:** Composability. The most common Phase 1 use is piping into `jq` or saving to a file. Text is opt-in.

### D5: Flags via the stdlib flag package

```
-model   string   override config default model (optional)
-format  string   "json" | "text"  (default "json")
-lang    string   target language hint passed to the system prompt (default "go")
```

**Why `flag` and not `cobra`:** The roadmap constraint is stdlib-only for Phase 1 CLIs. Three flags do not justify a framework. `flag.Parse()` reads the flags; the feature description comes from stdin (not a flag argument), so there is no positional-argument parsing complexity.

**`-lang` is a hint, not a filter:** It is injected into the system prompt ("Target language: go"). The model may still produce plans in another language if the feature description implies it — MP6 does not validate `plan.Language` against the `-lang` flag. The flag biases the model; it does not constrain the output.

### D6: System prompt — architect, not coder

```
You are a software architect. Given a feature description, produce a structured
code plan. Identify the files to create, the types to define, and the functions
to implement. Be specific about paths, signatures, and field types. Do not
write implementation code — only the plan. Target language: <lang>.
```

**Why this phrasing:** "Software architect" frames the model as a planner. "Do not write implementation code" is the hard constraint that keeps `FuncDecl` to signatures only. The `-lang` hint is appended so the model biases toward the requested ecosystem's conventions (e.g., `go` → package paths like `internal/core/domain/`).

### D7: Context cancellation via signal.NotifyContext

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
defer stop()
plan, err := useCase.Plan(ctx, input)
```

`signal.NotifyContext` (Go 1.16+) returns a context cancelled on SIGINT (Ctrl+C). `Extract` passes `ctx` to `provider.Chat`, so an in-flight HTTP call is abandoned. The `RetryProvider` (MP2) respects `ctx.Done()` during backoff waits, so Ctrl+C also aborts retry backoff.

**Why not context.WithTimeout:** The `OpenRouterProvider` (MP0) already has a 5-minute HTTP timeout. Adding a CLI-level timeout would double-layer it. Ctrl+C is the user's escape hatch. A `-timeout` flag is deferred to a later microphase if needed.

### D8: Error handling — map MP3 sentinel errors to exit codes

| Error                     | Exit code | Message (stderr)                                              |
|---------------------------|-----------|---------------------------------------------------------------|
| `ErrNoToolCall`           | 1         | "model did not return a structured plan"                      |
| `ErrToolNameMismatch`     | 1         | "model returned an unexpected tool call"                      |
| `ErrUnmarshalFailed`      | 1         | "failed to parse model output: <wrapped detail>"              |
| Provider error (HTTP)     | 1         | "request failed: <error>"                                     |
| `context.Canceled`        | 130       | (no message — user pressed Ctrl+C)                            |
| Empty stdin               | 1         | "no input: provide a feature description via stdin"           |

**Why 130 for SIGINT:** Convention — 128 + signal number (SIGINT = 2). Distinguishes user-initiated abort from failure.

## Risks / Trade-offs

- **[SchemaFromStruct extension may be incomplete]** → The recursive generator handles structs and `[]Struct` but not maps, `any`, interfaces, or self-referential types (cycle guard falls back to `{}`). If `CodePlan` is extended later with a `map[string]any` field, the schema for that field is opaque and the model may produce unstructured output for it. Mitigation: `CodePlan` is designed with only string fields and slices of structs — no maps. If a future field needs a map, hand-build that portion of the schema.
- **[Model may ignore forced tool_choice]** → Some OpenRouter models do not support forced tool calling and return prose instead. `ErrNoToolCall` surfaces this with a clear message. Mitigation: document recommended models in the CLI help; the config `DEFAULT_MODEL` should be a tool-capable model (e.g., Claude Sonnet).
- **[Model may produce an invalid plan semantically]** → `Extract` guarantees JSON-validity (it unmarshals), but the plan may have empty `Files`, duplicate paths, or signatures that don't match the target language's syntax. Mitigation: out of scope — MP6 is a planning aid, not a compiler. The user reviews the output.
- **[Deeply nested schema may exceed model context or confuse smaller models]** → The `CodePlan` schema with 4 levels of nesting is more complex than MP5's flat `ExtractionResult`. Smaller models may produce incomplete plans. Mitigation: recommend a `PRO_MODEL` (e.g., Claude Sonnet / GPT-4-class) in config; the `-model` flag lets the user override per-invocation.
- **[No stdin TTY detection]** → If the user runs `spec-to-code` without piping input and without typing anything, it blocks on stdin read. Mitigation: acceptable for Phase 1 — the CLI is designed for pipe usage (`echo "..." | spec-to-code`). A future `-i` interactive flag could read from a prompt; out of scope.

## Open Questions

- **Should `CodePlan` carry a `Dependencies []string` field (import paths)?** Deferred — the current `FilePlan` captures types and functions, which is sufficient for a Phase 1 plan. Adding dependency tracking is a Phase 2 refinement if the plan feeds into a code generator.
- **Should the text tree render to a pager (`less`)?** No — stdout is pipeable; the user pipes to `less` if needed. Adding pager logic complicates the CLI and breaks pipeability.
- **Should `-lang` validate against a known set (`go`, `python`, `typescript`)?** No — it is a free-form hint. Constraining it adds maintenance without value for a learning tool.
