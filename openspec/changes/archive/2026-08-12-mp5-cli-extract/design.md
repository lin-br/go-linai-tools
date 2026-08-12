## Context

MP3 shipped the structured-output capability: a generic `Extract[T any]` function that builds a forced-tool-choice `ChatRequest`, calls `Provider.Chat`, locates the matching `ToolCall`, and unmarshals its `Arguments` into a typed Go struct. It also shipped `SchemaFromStruct` (reflect-based JSON Schema generator) and `ToolSchema`. MP2 shipped `RetryProvider`, a decorator wrapping any `Provider` with exponential backoff + jitter. MP0 shipped the `Provider` interface, domain types, and `OpenRouterProvider`.

None of these have a user-facing CLI yet. Phase 1 deliverable #2 is `cmd/extract/` — "free-form text → structured data via tool use." The `extract` CLI is the first consumer of `Extract[T]`: it reads free-form text from stdin, extracts structured entities, and writes indented JSON to stdout. It is a thin adapter over an `ExtractUseCase`; the use case owns the `Extract[T]` orchestration.

The CLI is pipeable: `cat notes.txt | extract | jq .`. It uses non-streaming `Chat` (via `Extract[T]`) because the full tool-call response is needed to parse the JSON arguments once — there is nothing to stream incrementally.

## Goals / Non-Goals

**Goals:**
- Provide a `cmd/extract` binary that reads free-form text from stdin and writes structured JSON to stdout.
- Define a single predefined extraction schema (`ExtractionResult`) for Phase 1: summary, entities (name + type), action items, dates, amounts.
- Encapsulate the `Extract[T]` call in an `ExtractUseCase` so the CLI stays thin and the orchestration is testable independently.
- Generate the tool schema from `ExtractionResult` via `SchemaFromStruct` — no hand-built schema for Phase 1.
- Support `-model` to override the config default, `-format` (json, default), and `-pretty` (indented JSON, default true).
- Handle Ctrl+C via `signal.NotifyContext`, propagating cancellation through the use case to the provider.
- Map MP3's typed errors (`ErrNoToolCall`, `ErrUnmarshalFailed`) to clear user-facing messages and exit code 1.

**Non-Goals:**
- Custom schemas via a `-schema` flag (future extension; Phase 1 uses one predefined schema).
- YAML output (`-format yaml`) — the flag is reserved but only `json` is implemented in MP5.
- Streaming output — `extract` uses non-streaming `Chat`; streaming is MP1/MP4 (`summarize`).
- Batch extraction (multiple inputs per run) — one stdin → one extraction.
- Validation of the extracted struct beyond JSON decoding (no `validator` tags, no business-rule checks) — callers validate post-extraction.
- Agentic tool-call loops — single-shot extraction only. The agent loop is Phase 3.
- External CLI frameworks (`cobra`, `urfave/cli`) — `flag` package only, per roadmap constraint.

## Decisions

### D1: Non-streaming — use Chat, not ChatStream

`extract` calls `tools.Extract[T]`, which calls `Provider.Chat` (non-streaming). The model fills in the tool's JSON schema arguments in a single response; we parse `ToolCall.Arguments` once.

**Why:** Structured extraction needs the complete tool-call response to unmarshal the arguments. Streaming a tool call is not useful here — there is no incremental text to display; the value is the final JSON blob. Streaming would add complexity (reassembly of streamed function-call deltas) for zero UX benefit. MP1's `StreamClient` is not a dependency of MP5.

**Alternative considered:** Stream the response and reassemble the function-call arguments from `delta.tool_calls[].function.arguments` fragments (OpenAI streaming tool-call format). Rejected — the reassembly logic is fragile across models and gains nothing for a CLI that outputs a single JSON object.

### D2: JSON output to stdout — pipeable by design

The extracted `ExtractionResult` is marshaled to JSON and written to stdout. With `-pretty` (default true), the JSON is indented (`json.MarshalIndent` with 2-space prefix). With `-pretty=false`, it is compact (`json.Marshal`).

**Why:** JSON to stdout makes the CLI composable in Unix pipelines: `cat input.txt | extract | jq '.entities[] | .name'`. Indented JSON is the sensible default for human readability; compact mode is for piping into other tools that don't care about whitespace.

**Alternative considered:** Plain-text formatted output (key: value). Rejected — loses structure (arrays of entities, nested name/type) and is not machine-parseable. JSON is the universal interchange format.

### D3: Predefined schema for Phase 1 — SchemaFromStruct, not hand-built

The extraction schema is derived from a fixed Go struct, `ExtractionResult`, using `tools.SchemaFromStruct` (MP3). The struct captures the Phase 1 target: a summary, entities with name + type, action items, dates, and amounts.

```go
type ExtractionResult struct {
    Summary     string   `json:"summary"`
    Entities    []Entity `json:"entities"`
    ActionItems []string `json:"action_items"`
    Dates       []string `json:"dates"`
    Amounts     []string `json:"amounts"`
}

type Entity struct {
    Name string `json:"name"`
    Type string `json:"type"`
}
```

The schema is generated once at startup (`SchemaFromStruct(&ExtractionResult{})`) and reused for every call.

**Why:** `SchemaFromStruct` is the MP3 capability designed for exactly this. Using it validates the helper on a real struct and keeps the schema in sync with the Go type — change the struct, the schema follows. No hand-maintained JSON Schema string that can drift from the struct.

**Limitation accepted:** `SchemaFromStruct` is deliberately shallow (MP3 D3). `[]Entity` becomes `{type: "array"}` without item types, and `Entity`'s nested fields are not described. This is a known Phase 1 limitation — the model still fills in the array correctly because the system prompt and field names are descriptive enough. If extraction quality suffers, a hand-built schema can replace `SchemaFromStruct` without changing the use case or CLI.

**Alternative considered:** Hand-build the `map[string]any` schema with full nested item types. Rejected for Phase 1 — it would bypass the `SchemaFromStruct` learning goal and duplicate the struct's shape in two places.

### D4: Use case pattern — ExtractUseCase wraps Extract[T]

`ExtractUseCase` lives in `internal/core/usecases/extract.go`. It holds a `Provider` (already wrapped with `RetryProvider` at wiring time) and exposes `Extract(ctx, model, input) (*ExtractionResult, error)`. Internally it calls `tools.Extract[ExtractionResult]` with the predefined schema and system prompt.

**Why:** Keeps the CLI thin (flag parsing, I/O, error mapping) and the orchestration in the use case layer, consistent with the hexagonal layout. The use case is independently testable with a fake `Provider` — no need to spawn the CLI binary in tests.

**Alternative considered:** Call `tools.Extract[ExtractionResult]` directly from `main.go`. Rejected — puts orchestration in the adapter layer, untestable without a subprocess, and breaks the "thin CLI" convention established by `cmd/cli/main.go`.

### D5: No external CLI framework — flag package only

`cmd/extract/main.go` uses the standard library `flag` package for `-model`, `-format`, and `-pretty`. No `cobra`, no `urfave/cli`.

**Why:** The roadmap constraint is explicit: `net/http` + `encoding/json` only, learn the wire protocol. A CLI with three flags does not justify a framework. `flag` is sufficient and keeps the dependency surface at zero.

### D6: Error mapping — typed errors to user-facing messages

| Error (`errors.Is`) | User-facing output | Exit code |
|---|---|---|
| `tools.ErrNoToolCall` | `Model did not return structured data.` written to stderr | 1 |
| `tools.ErrUnmarshalFailed` | `Failed to parse structured data: <err>` + raw `ToolCall.Arguments` written to stderr for debugging | 1 |
| Provider error (retry-exhausted) | `Extraction failed: <err>` written to stderr | 1 |
| stdin read error | `Failed to read input: <err>` written to stderr | 1 |

**Why:** MP3 defined typed sentinel errors precisely so callers can branch on failure mode. The CLI translates each into a clear, actionable message. For `ErrUnmarshalFailed`, printing the raw arguments lets the user see what the model actually returned — critical for debugging schema mismatches. All errors go to stderr so stdout stays clean for JSON piping.

### D7: signal.NotifyContext for Ctrl+C

`main` creates a context via `signal.NotifyContext(context.Background(), os.Interrupt)`. The context is passed to `useCase.Extract(ctx, ...)`, which passes it to `tools.Extract[T]`, which passes it to `provider.Chat(ctx, req)`. The `RetryProvider` (MP2) also respects the context in its backoff waits.

**Why:** A Ctrl+C during a slow LLM call should abort immediately, not wait for the 5-minute HTTP timeout. `signal.NotifyContext` is the idiomatic Go pattern. Context propagation is already wired through every layer by MP0/MP2/MP3 — MP5 just creates the context at the top.

### D8: Struct location — usecases package, not domain

`ExtractionResult` and `Entity` live in `internal/core/usecases/extract_schema.go`, not in `internal/core/domain/`.

**Why:** `ExtractionResult` is a use-case-specific output type, not a provider-agnostic domain primitive. It is the result shape of the `extract` use case, not a type shared across use cases or providers. Putting it in `domain` would imply it is part of the core domain model shared by all capabilities — it is not. `Entity` is a helper type for `ExtractionResult` and lives alongside it.

**Alternative considered:** Put the struct in `cmd/extract/main.go`. Rejected — the use case returns `*ExtractionResult`, so the type must be importable by both the CLI and the use case. A shared package is the clean boundary.

## Risks / Trade-offs

- **[SchemaFromStruct is shallow on nested arrays]** → `[]Entity` produces `{type: "array"}` with no item schema; the model must infer array item shape from field names and the system prompt. Mitigation: the system prompt explicitly describes the expected shape; if extraction quality is poor on real input, swap `SchemaFromStruct` for a hand-built schema without touching the use case or CLI. This is a documented MP3 limitation.
- **[Model may ignore forced tool_choice]** → Some OpenRouter models do not support forced tool choice and return text instead. Mitigation: `ErrNoToolCall` is mapped to a clear message ("Model did not return structured data."); the user can switch models via `-model`. This is a provider/model capability, not something MP5 can enforce.
- **[Single predefined schema]** → Phase 1 ships one extraction shape. Users who need a different schema (e.g., extract only dates) cannot do it without code changes. Mitigation: documented as a non-goal; the `-schema` flag is a future extension. The use case and CLI are structured so adding a schema selector is a localized change.
- **[No semantic validation]** → `Extract[T]` returns a JSON-valid struct that may be semantically empty (zero-value fields if the model omitted data). Mitigation: the system prompt instructs "If a field has no data, use an empty array or empty string — do not guess." Post-extraction validation is the caller's responsibility and out of scope for MP5.
- **[stdin is unbounded]** → The CLI reads all of stdin into memory. For very large inputs, this could be a problem. Mitigation: Phase 1 inputs are notes/documents, not gigabyte streams. A streaming-input approach would conflict with the non-streaming `Chat` requirement. No mitigation needed for Phase 1.
