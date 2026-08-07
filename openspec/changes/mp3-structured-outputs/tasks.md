## 1. Package Scaffolding and Errors

- [ ] 1.1 Create `internal/core/tools/errors.go` — define sentinel errors `ErrNoToolCall`, `ErrToolNameMismatch`, `ErrUnmarshalFailed`
- [ ] 1.2 Verify `internal/core/tools` package imports `domain` and `outbound` without import cycles

## 2. ToolSchema and SchemaFromStruct

- [ ] 2.1 Create `internal/core/tools/schema.go` — define `ToolSchema{Name, Description, InputSchema}` struct
- [ ] 2.2 Implement `SchemaFromStruct(v any) (map[string]any, error)` — `reflect`-based JSON Schema generation: `type: "object"`, `properties`, `required`
- [ ] 2.3 Map Go types to JSON Schema types: `string`→`"string"`, `int`/`int64`/`int32`/`float64`/`float32`→`"number"`, `bool`→`"boolean"`
- [ ] 2.4 Parse `json` struct tags for field names; use Go field name when no tag present; exclude `omitempty` fields from `required`
- [ ] 2.5 Skip unexported fields; return error for non-struct input

## 3. BuildToolRequest

- [ ] 3.1 Create `internal/core/tools/extract.go` — implement `BuildToolRequest(model, system, input string, schema ToolSchema) *domain.ChatRequest`
- [ ] 3.2 Set `Model`, `System`, single `Messages` entry (`Role: "user"`, `Content: input`), single `Tools` entry from `ToolSchema`, and `ToolChoice{Type: "tool", Name: schema.Name}`

## 4. Extract Generic Function

- [ ] 4.1 Implement `Extract[T any](ctx context.Context, p outbound.Provider, model, system, input string, schema ToolSchema) (*T, error)` in `extract.go`
- [ ] 4.2 Call `BuildToolRequest` then `p.Chat(ctx, req)`; propagate provider errors directly
- [ ] 4.3 Find first `ToolCall` where `Name == schema.Name`; return `ErrNoToolCall` when `ToolCalls` is nil/empty
- [ ] 4.4 Return `ErrToolNameMismatch` when tool calls exist but none matches `schema.Name`
- [ ] 4.5 `json.Unmarshal` the matching `ToolCall.Arguments` into a new `T`; return `ErrUnmarshalFailed` (wrapped) on decode failure
- [ ] 4.6 Return `*T` and nil error on success; pass `ctx` to `provider.Chat` unmodified

## 5. Unit Tests

- [ ] 5.1 Create `internal/core/tools/schema_test.go` — test `SchemaFromStruct` with required/optional fields, type mapping, unexported fields, non-struct error
- [ ] 5.2 Create `internal/core/tools/extract_test.go` — test `BuildToolRequest` output (forced tool choice, single tool, user message)
- [ ] 5.3 Test `Extract[T]` with a fake `Provider` mock: success path returns parsed `*T`
- [ ] 5.4 Test `Extract[T]` error paths: provider error, `ErrNoToolCall`, `ErrToolNameMismatch`, `ErrUnmarshalFailed` (verify `errors.Is`)
- [ ] 5.5 Test `Extract[T]` context propagation — cancelled context returns provider error

## 6. Verification

- [ ] 6.1 Run `go build ./...` — all packages compile
- [ ] 6.2 Run `go vet ./...` — no warnings
- [ ] 6.3 Run `go test ./internal/core/tools/...` — all tests pass
- [ ] 6.4 Manually verify `Extract[T]` end-to-end against OpenRouter with a simple struct (e.g. `Person{Name, Age}`) using a real API key
