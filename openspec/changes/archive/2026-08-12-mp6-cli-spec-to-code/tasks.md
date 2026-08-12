## 1. CodePlan Domain Aggregate

- [x] 1.1 Create `internal/core/domain/codeplan.go` — define `CodePlan{Summary, Language string, Files []FilePlan}` with `json` tags
- [x] 1.2 Define `FilePlan{Path, Description string, Types []TypeDecl, Functions []FuncDecl}` with `omitempty` on `Types` and `Functions`
- [x] 1.3 Define `TypeDecl{Name, Description string, Fields []FieldDecl}` with `omitempty` on `Fields`
- [x] 1.4 Define `FieldDecl{Name, Type, Description string}` with `omitempty` on `Description`
- [x] 1.5 Define `FuncDecl{Name, Signature, Description string}` with `omitempty` on `Description` — no body/implementation field
- [x] 1.6 Verify the structs are plain data (no methods) and compile with `go build ./internal/core/domain/`

## 2. SchemaFromStruct Nested-Struct Recursion (MP3 Extension)

- [x] 2.1 Extend `SchemaFromStruct` in `internal/core/tools/schema.go` to detect struct-typed fields and recurse, producing inline `{type: "object", properties: {...}, required: [...]}` sub-schemas
- [x] 2.2 Unwrap pointer-to-struct fields (`*T`) to element type `T` before recursing
- [x] 2.3 Add a cycle-detection set (visited types) to prevent infinite recursion on self-referential structs; fall back to `{}` when a cycle is detected
- [x] 2.4 Extend slice handling: for `[]Struct` (or `[]*Struct`), produce `{type: "array", items: {<recursed schema for element>}}`
- [x] 2.5 For scalar slices (`[]string`, `[]int`, `[]bool`, etc.), produce `{type: "array", items: {type: "<scalar>"}}`
- [x] 2.6 Leave maps, `any`, interfaces, and custom non-struct types as opaque `{}` — out of scope

## 3. SchemaFromStruct Tests for Nesting

- [x] 3.1 Add test: nested struct field produces inline object schema with properties and required
- [x] 3.2 Add test: pointer-to-struct field unwrapped to element type, `omitempty` excludes from required
- [x] 3.3 Add test: self-referential struct (`Node{Next *Node}`) terminates without panic, `next` property is `{}`
- [x] 3.4 Add test: `[]Struct` slice produces `{type: "array", items: {type: "object", ...}}` with correct item schema
- [x] 3.5 Add test: `[]string` slice produces `{type: "array", items: {type: "string"}}`
- [x] 3.6 Add test: `SchemaFromStruct(&CodePlan{})` produces a complete schema with `files` as an array of objects containing `path`, `types`, `functions`

## 4. SpecToCodeUseCase

- [x] 4.1 Create `internal/core/usecases/spec_to_code.go` — define `SpecToCodeUseCase` struct holding `provider outbound.Provider` and `model string`
- [x] 4.2 Implement `NewSpecToCodeUseCase(provider outbound.Provider, model string) *SpecToCodeUseCase`
- [x] 4.3 Define the system prompt as a constant — must contain "software architect" and "Do not write implementation code"; include a `Target language: %s` placeholder
- [x] 4.4 Implement `Plan(ctx context.Context, input string) (*domain.CodePlan, error)` — build `ToolSchema` via `SchemaFromStruct(&domain.CodePlan{})`, format the system prompt with the language, call `tools.Extract[domain.CodePlan]`, return result
- [x] 4.5 Propagate `Extract` errors directly (no wrapping) — `ErrNoToolCall`, `ErrToolNameMismatch`, `ErrUnmarshalFailed`, provider errors
- [x] 4.6 Verify no import cycle: `usecases` imports `domain`, `outbound`, `tools` — none import `usecases`

## 5. SpecToCodeUseCase Tests

- [x] 5.1 Create `internal/core/usecases/spec_to_code_test.go` — test `Plan` success path with a fake `Provider` mock returning a valid `CodePlan` tool call; assert typed `*CodePlan` returned
- [x] 5.2 Test `Plan` propagates `ErrNoToolCall` when provider returns no tool calls
- [x] 5.3 Test `Plan` propagates `ErrUnmarshalFailed` when tool call arguments are invalid JSON
- [x] 5.4 Test `Plan` propagates provider errors directly
- [x] 5.5 Test `Plan` passes `ctx` through to `provider.Chat` — cancelled context returns provider error

## 6. CLI Entry Point — Wiring and Flags

- [x] 6.1 Create `cmd/spec-to-code/main.go` — load configs, construct `OpenRouterProvider`, wrap in `RetryProvider`, construct `SpecToCodeUseCase`
- [x] 6.2 Parse flags via `flag` package: `-model string` (default empty), `-format string` (default `json`), `-lang string` (default `go`)
- [x] 6.3 Validate `-format` is `json` or `text`; print error to stderr and exit 1 on invalid value
- [x] 6.4 Resolve model: if `-model` non-empty use it, else use config `DEFAULT_MODEL`
- [x] 6.5 Read feature description from `os.Stdin` via `io.ReadAll`; if empty, print "no input: provide a feature description via stdin" to stderr and exit 1

## 7. CLI — Context, Invocation, and Output

- [x] 7.1 Create context via `signal.NotifyContext(context.Background(), os.Interrupt)`; defer `stop()`
- [x] 7.2 Call `useCase.Plan(ctx, input)`; pass `-lang` through to the use case's system prompt
- [x] 7.3 On success with `-format json`: write `json.MarshalIndent(plan, "", "  ")` + newline to stdout
- [x] 7.4 On success with `-format text`: call `renderTree(plan)` and write to stdout
- [x] 7.5 On `context.Canceled`: exit 130 with no message
- [x] 7.6 On `ErrNoToolCall`: print "model did not return a structured plan" to stderr, exit 1
- [x] 7.7 On `ErrToolNameMismatch`: print "model returned an unexpected tool call" to stderr, exit 1
- [x] 7.8 On `ErrUnmarshalFailed`: print "failed to parse model output: <wrapped detail>" to stderr, exit 1
- [x] 7.9 On other errors: print "request failed: <error>" to stderr, exit 1

## 8. Text Tree Renderer

- [x] 8.1 Implement `renderTree(plan *domain.CodePlan) string` (in `cmd/spec-to-code/main.go` or a sibling `output.go`)
- [x] 8.2 Print header: plan summary and language
- [x] 8.3 For each `FilePlan`: print the file path, then `Types:` section listing each `TypeDecl` name with indented `FieldDecl` lines (`Name: Type`)
- [x] 8.4 Under each file: print `Functions:` section listing each `FuncDecl.Signature` indented
- [x] 8.5 Use ASCII indentation (spaces) for hierarchy — no emojis
- [x] 8.6 Handle empty `Types` or `Functions` slices gracefully (omit the section label)

## 9. Verification

- [x] 9.1 Run `go build ./...` — all packages compile including `cmd/spec-to-code`
- [x] 9.2 Run `go vet ./...` — no warnings
- [x] 9.3 Run `go test ./internal/core/tools/...` — SchemaFromStruct nesting tests pass
- [x] 9.4 Run `go test ./internal/core/usecases/...` — SpecToCodeUseCase tests pass
- [x] 9.5 Run `go test ./...` — all tests pass
- [x] 9.6 Manually run `echo "Add user authentication with login and register" | go run ./cmd/spec-to-code -format json` and verify valid JSON output
- [x] 9.7 Manually run the same input with `-format text` and verify the ASCII tree is readable
- [x] 9.8 Manually test Ctrl+C during a slow request — verify exit code 130
- [x] 9.9 Manually test empty stdin — verify exit code 1 and stderr message
- [x] 9.10 Manually test `-format yaml` — verify exit code 1 and stderr message
