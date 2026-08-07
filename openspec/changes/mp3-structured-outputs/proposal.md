## Why

Phase 1 deliverable #5 requires "Structured outputs via forced `tool_choice` (NOT JSON-mode prompting)." MP0 defined the `Tool`, `ToolChoice`, and `ToolCall` domain types and the `OpenRouterProvider` tool translation, but there is no reusable helper that turns free-form text into a typed Go struct. Without it, the `extract` and `spec-to-code` CLIs (MP4–MP6) would each hand-roll schema generation, request building, and argument parsing — duplicating logic and drifting in behavior. MP3 closes that gap with a single generic `Extract[T]` capability built on forced tool choice.

## What Changes

- Add a `ToolSchema` type that bundles a tool name, description, and JSON Schema (`map[string]any`) describing the desired output shape.
- Add `SchemaFromStruct(v any) (map[string]any, error)` — a `reflect`-based helper that walks a Go struct's fields and produces a basic JSON Schema (`type`, `properties`, `required`) honoring `json` tags and `omitempty`. Intentionally simple — no `oneOf`, `allOf`, or nested schema composition for Phase 1.
- Add `BuildToolRequest(model, system, input, schema)` — a helper that constructs a `*domain.ChatRequest` with a single `Tool` definition and `ToolChoice{Type: "tool", Name: schema.Name}` forcing the model to call that exact tool.
- Add a generic `Extract[T any](ctx, provider, model, system, input, schema) (*T, error)` function that builds the forced-tool request, calls `provider.Chat`, locates the matching `ToolCall`, and `json.Unmarshal`s its `Arguments` string into a `*T`.
- Add structured error handling: no tool call in response, tool call name mismatch, and JSON unmarshal failure — each a distinct, wrappable error.
- All new code lives in a new `internal/core/tools/` package (reusable capability, not a use case).

## Capabilities

### New Capabilities

- `structured-outputs`: The generic `Extract[T]` extraction helper, `ToolSchema` type, `SchemaFromStruct` JSON Schema generator from Go struct tags, and `BuildToolRequest` forced-tool-choice request builder.

### Modified Capabilities

(No existing specs are modified. MP3 consumes the MP0 `Provider` interface, `ChatRequest`/`ChatResponse`, and `Tool`/`ToolChoice`/`ToolCall` types as-is — it adds a new capability on top of them.)

## Impact

- **`internal/core/tools/`** — new package: `schema.go` (`ToolSchema`, `SchemaFromStruct`), `extract.go` (`Extract[T]`, `BuildToolRequest`), `errors.go` (sentinel/typed errors).
- **No changes** to `internal/core/domain/`, `internal/core/ports/`, `internal/core/usecases/`, or `internal/adapters/` — MP3 is a consumer of MP0, not a modifier.
- **No new external dependencies** — `reflect`, `encoding/json`, and the MP0 domain types only.
- **No breaking changes** — purely additive.
- **Enables** MP4–MP6 (`extract` and `spec-to-code` CLIs depend on `Extract[T]`).
