## Context

MP0 established the provider-agnostic foundation: a `Provider` interface with `Chat(ctx, *ChatRequest) (*ChatResponse, error)`, domain types for `Tool`/`ToolChoice`/`ToolCall`, and an `OpenRouterProvider` that translates these to the OpenAI Chat Completions wire format — including `tools[]` with `function.parameters` and `tool_choice` forced to a named function. That translation layer is complete but unused: nothing in the codebase actually forces a tool call or parses the returned `ToolCall.Arguments`.

Phase 1 deliverable #5 is "Structured outputs via forced `tool_choice` (NOT JSON-mode prompting)." The roadmap is explicit — we do NOT use `response_format: {type: "json_object"}`. Instead we define a tool whose `input_schema` matches the desired output struct, set `tool_choice` to force that tool, and parse `tool_call.function.arguments` from the response. This guarantees the model returns arguments conforming to the schema, and it works across every OpenRouter-backed model (not all support JSON mode).

## Goals / Non-Goals

**Goals:**
- Provide a single generic `Extract[T any]` function that turns free-form text into a typed Go struct via forced tool choice.
- Generate a basic JSON Schema from Go struct fields using `reflect` and `json` tags — no external schema libraries.
- Build a reusable `ToolSchema` type and `BuildToolRequest` helper so the `extract` and `spec-to-code` CLIs (MP4–MP6) share one code path.
- Return clear, typed errors for the three failure modes: no tool call, name mismatch, unmarshal failure.

**Non-Goals:**
- JSON-mode prompting (`response_format: {type: "json_object"}` or `json_schema`) — explicitly rejected by the roadmap.
- Advanced JSON Schema features (`oneOf`, `allOf`, `$ref`, `pattern`, `format`, `minimum`/`maximum`) — Phase 1 keeps the schema generator deliberately simple.
- Tool-call loop / agentic execution (model calls tool → execute → feed result back) — MP3 is single-shot extraction only. The agent loop is Phase 3.
- Validation of the unmarshaled struct beyond JSON decoding (no `validator` tags, no business-rule checks).
- Streaming structured outputs — `Extract` uses non-streaming `Chat`.

## Decisions

### D1: Forced tool_choice, NOT response_format JSON mode

We define a `Tool` with an `InputSchema` (JSON Schema) matching the desired output struct, set `ToolChoice{Type: "tool", Name: schema.Name}`, and parse the `ToolCall.Arguments` from the response.

**Why:** The roadmap explicitly says "Structured outputs via forced tool_choice (NOT JSON-mode prompting)." Forced tool choice works across every OpenRouter-backed model — JSON mode is opt-in and not universally supported. It also guarantees the model emits a `tool_call` with `function.arguments`, giving us a single, well-defined parse target rather than free-form text that might or might not be valid JSON.

**Alternative considered:** `response_format: {type: "json_schema", json_schema: {...}}` (OpenAI structured outputs). Rejected — not all OpenRouter models support it, and the roadmap forbids JSON-mode prompting.

### D2: Generic Extract[T any]

```go
func Extract[T any](ctx context.Context, p outbound.Provider, model, system, input string, schema ToolSchema) (*T, error)
```

**Why:** Go generics (1.18+) let us return a concrete `*T` without `any` + type assertions at the call site. The caller defines a struct, passes it (or its schema), and gets back a typed pointer. This is the cleanest API for the `extract` and `spec-to-code` CLIs.

**Flow:**
1. `BuildToolRequest(model, system, input, schema)` → `*domain.ChatRequest` with one `Tool` + forced `ToolChoice`.
2. `provider.Chat(ctx, req)` → `*domain.ChatResponse`.
3. Find `ToolCall` where `Name == schema.Name` in `response.ToolCalls`.
4. `json.Unmarshal([]byte(toolCall.Arguments), &T{})` → `*T`.

### D3: SchemaFromStruct via reflect — simple by design

```go
func SchemaFromStruct(v any) (map[string]any, error)
```

Walks the struct with `reflect`, maps Go types to JSON Schema types (`string`→`"string"`, `int`/`int64`/`float64`→`"number"`, `bool`→`"boolean"`), reads `json` tags for field names and `omitempty` for optional (not in `required`), and builds `{type: "object", properties: {...}, required: [...]}`.

**Why:** Avoids external dependencies (`invopop/jsonschema`, `xeipuuv/gojsonschema`). Phase 1 schemas are flat or shallow — structs with scalar fields. Hand-rolling the common cases is ~60 lines and teaches the `reflect` package, which is a roadmap learning goal.

**Limitations explicitly accepted:** no nested object schemas (nested structs become `{}` — opaque), no array item types beyond `"array"`, no enums, no string formats. Callers who need richer schemas pass a hand-built `map[string]any` directly to `ToolSchema` instead of using `SchemaFromStruct`.

### D4: ToolSchema allows both generated and hand-built schemas

```go
type ToolSchema struct {
    Name        string
    Description string
    InputSchema map[string]any
}
```

`InputSchema` is a plain `map[string]any` — the same type as `domain.Tool.InputSchema`. Callers either run `SchemaFromStruct(&MyStruct{})` for the simple case or construct the map by hand for complex schemas. No coupling between the schema generator and the extraction path.

### D5: Location — internal/core/tools/

The `Extract` helper, `ToolSchema`, `SchemaFromStruct`, and `BuildToolRequest` live in a new `internal/core/tools/` package.

**Why not `internal/core/usecases/`:** `Extract` is a reusable capability, not a single-use-case orchestration. It doesn't implement `inbound.Entrypoint` and has no single "Send" method. Putting it in `usecases` would imply it's a driving-adapter-facing use case; it's actually a library function called by use cases and CLIs alike.

**Why not `internal/core/domain/`:** `domain` holds plain data structs. `Extract` has behavior (calls `Provider`, parses responses) — it belongs in a layer above domain but below adapters.

### D6: Error model — typed sentinel errors

Three distinct error values in `internal/core/tools/errors.go`:
- `ErrNoToolCall` — response has no `ToolCalls` (model ignored the forced choice).
- `ErrToolNameMismatch` — tool call exists but `Name != schema.Name`.
- `ErrUnmarshalFailed` — `json.Unmarshal` of `Arguments` into `T` failed (wrapped with the underlying `*json.SyntaxError` or similar).

**Why sentinels over custom error types:** Callers (CLIs) need to branch on failure mode for user-facing messages ("model didn't return structured data" vs "returned data didn't match the schema"). `errors.Is` on sentinels is the simplest correct approach. The unmarshal error wraps the underlying cause via `fmt.Errorf("%w: %v", ErrUnmarshalFailed, err)` so `errors.Is` matches the sentinel while the detail is preserved.

### D7: Arguments is a JSON string

`domain.ToolCall.Arguments` is a `string` (decided in MP0 to stay provider-agnostic — OpenAI sends a string, Anthropic sends an object). `Extract` MUST `json.Unmarshal([]byte(toolCall.Arguments), &target)` — it does not assume the provider pre-parsed the arguments.

## Risks / Trade-offs

- **[Model may not respect forced tool_choice]** → Some models on OpenRouter may ignore `tool_choice: {type: "function", ...}` and return text instead. Mitigation: `ErrNoToolCall` is returned with a clear message; the CLI layer can retry or surface a user-facing error. Not all OpenRouter models support forced tool choice — this is a provider/model capability, not something we can enforce in code.
- **[SchemaFromStruct is shallow]** → Nested structs, slices of structs, maps, and custom types produce incomplete schemas (`{}` or `"array"` without item types). Mitigation: documented as a Phase 1 limitation; callers with complex schemas pass a hand-built `map[string]any`. The `extract`/`spec-to-code` CLIs are designed around flat structs for Phase 1.
- **[No validation beyond JSON decode]** → `Extract` returns a `*T` that is JSON-valid but may be semantically empty (zero-value fields if the model omitted them). Mitigation: callers validate business rules after extraction. Adding a validation layer is out of scope for MP3.
- **[Single tool call assumed]** → `Extract` finds the first `ToolCall` matching `schema.Name`. If the model returns multiple calls (unlikely with forced single-tool choice), extras are ignored. Mitigation: forced `tool_choice` with a single tool definition makes multiple calls extremely unlikely; if it happens, the first match is the correct one.
- **[reflect performance]** → `SchemaFromStruct` runs per call. For a CLI tool processing one input at a time, the reflect cost is negligible compared to the LLM round-trip. Mitigation: none needed for Phase 1. If batch processing arrives later, cache the schema per type.
