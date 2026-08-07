## 1. Extraction Schema and Structs

- [ ] 1.1 Create `internal/core/usecases/extract_schema.go` — define `ExtractionResult{Summary, Entities, ActionItems, Dates, Amounts}` and `Entity{Name, Type}` with `json` struct tags
- [ ] 1.2 Verify `SchemaFromStruct(&ExtractionResult{})` (MP3) produces a schema with `summary`, `entities`, `action_items`, `dates`, `amounts` properties — confirm `[]Entity` and `[]string` map to `{type: "array"}`

## 2. ExtractUseCase

- [ ] 2.1 Create `internal/core/usecases/extract.go` — define `ExtractUseCase` struct holding a `Provider` (retry-wrapped) and the predefined `ToolSchema`
- [ ] 2.2 Implement `NewExtractUseCase(provider outbound.Provider) *ExtractUseCase` — generate the schema via `tools.SchemaFromStruct(&ExtractionResult{})` once at construction
- [ ] 2.3 Implement `Extract(ctx context.Context, model, input string) (*ExtractionResult, error)` — call `tools.Extract[ExtractionResult](ctx, uc.provider, model, systemPrompt, input, uc.schema)` and return the result
- [ ] 2.4 Define the system prompt: "Extract structured information from the following text. Fill in all fields. If a field has no data, use an empty array or empty string — do not guess."

## 3. CLI Entry Point

- [ ] 3.1 Create `cmd/extract/main.go` — wire `configs.LoadConfigs()` → `OpenRouterProvider` → `RetryProvider` → `ExtractUseCase`
- [ ] 3.2 Add `signal.NotifyContext(context.Background(), os.Interrupt)` and pass the context to `useCase.Extract`
- [ ] 3.3 Read all of stdin into a string until EOF using `io.ReadAll(os.Stdin)`
- [ ] 3.4 Parse flags: `-model` (string, default from `configs.Models.Get()`), `-format` (string, default "json"), `-pretty` (bool, default true)
- [ ] 3.5 Validate `-format` — accept `json`; reject any other value with an error to stderr and exit 1

## 4. Output and Error Handling

- [ ] 4.1 On success, marshal `*ExtractionResult` to JSON: use `json.MarshalIndent` with 2-space indent when `-pretty` is true, `json.Marshal` when false; write to stdout
- [ ] 4.2 Map `errors.Is(err, tools.ErrNoToolCall)` → print "Model did not return structured data." to stderr, exit 1
- [ ] 4.3 Map `errors.Is(err, tools.ErrUnmarshalFailed)` → print the underlying error and raw `ToolCall.Arguments` to stderr, exit 1
- [ ] 4.4 Map any other error → print "Extraction failed: <err>" to stderr, exit 1
- [ ] 4.5 Ensure no partial JSON is written to stdout on any error path

## 5. Unit Tests

- [ ] 5.1 Create `internal/core/usecases/extract_test.go` — test `ExtractUseCase.Extract` with a fake `Provider` mock returning a valid tool call; verify the returned `*ExtractionResult` fields
- [ ] 5.2 Test `ExtractUseCase.Extract` error paths: provider error, `ErrNoToolCall`, `ErrUnmarshalFailed` (verify `errors.Is`)
- [ ] 5.3 Test that `NewExtractUseCase` generates the schema once and reuses it across calls
- [ ] 5.4 Test context propagation — cancelled context returns the provider error

## 6. Verification

- [ ] 6.1 Run `go build ./...` — all packages compile
- [ ] 6.2 Run `go vet ./...` — no warnings
- [ ] 6.3 Run `go test ./internal/core/usecases/...` — all tests pass
- [ ] 6.4 Manually test `echo "Meeting with John on 2024-03-15 about $500 budget" | go run ./cmd/extract` — verify JSON output with summary, entities, dates, amounts
- [ ] 6.5 Manually test `-model` override, `-pretty=false` compact output, and `-format yaml` rejection
- [ ] 6.6 Manually verify pipeability: `cat input.txt | go run ./cmd/extract | jq '.entities[] | .name'`
