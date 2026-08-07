## ADDED Requirements

### Requirement: Extract CLI reads stdin and writes JSON to stdout

The system SHALL provide a `cmd/extract` binary that reads free-form text from stdin until EOF, passes it to an extraction use case, marshals the result to JSON, and writes the JSON to stdout. All diagnostic and error messages SHALL be written to stderr so that stdout contains only the JSON result. The CLI SHALL use non-streaming `Provider.Chat` (via `tools.Extract[T]`) and SHALL NOT depend on streaming infrastructure.

#### Scenario: Basic extraction pipeline

- **WHEN** the user runs `echo "Meeting with John on 2024-03-15 about $500 budget" | go run ./cmd/extract`
- **THEN** the CLI SHALL read the input from stdin, extract structured data, and write a JSON object to stdout containing `summary`, `entities`, `action_items`, `dates`, and `amounts` fields

#### Scenario: Pipeable to jq

- **WHEN** the user runs `cat notes.txt | go run ./cmd/extract | jq '.entities[] | .name'`
- **THEN** stdout SHALL contain only valid JSON (no log lines, no prompts) so that `jq` parses it successfully

#### Scenario: Empty stdin

- **WHEN** stdin is empty (zero bytes) and the user closes the input
- **THEN** the CLI SHALL still call the extraction use case with an empty input string and write the resulting JSON (which may have empty arrays/strings) to stdout, or return an error if the use case fails

### Requirement: Predefined extraction schema for Phase 1

The system SHALL define an `ExtractionResult` struct in `internal/core/usecases` with fields: `Summary string` (json `summary`), `Entities []Entity` (json `entities`), `ActionItems []string` (json `action_items`), `Dates []string` (json `dates`), `Amounts []string` (json `amounts`). The `Entity` struct SHALL have fields `Name string` (json `name`) and `Type string` (json `type`). The tool schema SHALL be generated from `ExtractionResult` via `tools.SchemaFromStruct` (MP3) at startup and reused for every extraction call. The CLI SHALL NOT support custom schemas via flags in Phase 1.

#### Scenario: Schema generated from struct

- **WHEN** the extraction use case is initialized
- **THEN** it SHALL call `tools.SchemaFromStruct(&ExtractionResult{})` to produce the `ToolSchema` and use that schema for all subsequent `Extract[T]` calls

#### Scenario: Output JSON matches struct shape

- **WHEN** the CLI writes the extraction result to stdout
- **THEN** the JSON SHALL have top-level keys `summary`, `entities`, `action_items`, `dates`, and `amounts`, where `entities` is an array of objects with `name` and `type` keys

### Requirement: ExtractUseCase wraps tools.Extract

The system SHALL define an `ExtractUseCase` in `internal/core/usecases` that holds a `Provider` (wrapped with `RetryProvider` at wiring time) and exposes an `Extract(ctx context.Context, model, input string) (*ExtractionResult, error)` method. The method SHALL call `tools.Extract[ExtractionResult]` with the predefined schema and a system prompt instructing the model to extract structured information and to use empty arrays or empty strings for fields with no data, never guessing. The CLI SHALL be a thin adapter: it parses flags, reads stdin, calls `useCase.Extract`, marshals the result to JSON, and writes to stdout.

#### Scenario: Use case delegates to Extract generic

- **WHEN** `ExtractUseCase.Extract(ctx, "anthropic/claude-sonnet-4-20250514", "John is 30")` is called
- **THEN** it SHALL invoke `tools.Extract[ExtractionResult](ctx, provider, "anthropic/claude-sonnet-4-20250514", systemPrompt, "John is 30", schema)` and return the resulting `*ExtractionResult`

#### Scenario: Context propagated to provider

- **WHEN** `ExtractUseCase.Extract` is called with a context that has a deadline
- **THEN** the context SHALL be passed unmodified through `tools.Extract[T]` to `provider.Chat`, and if the context expires before the provider returns, the use case SHALL propagate the error

### Requirement: -model flag overrides config default

The CLI SHALL accept a `-model` string flag. When provided, the CLI SHALL pass the flag value as the `model` argument to `useCase.Extract` instead of the config default model. When not provided, the CLI SHALL use the resolved default model from `configs.Models.Get()`.

#### Scenario: Model override

- **WHEN** the user runs `go run ./cmd/extract -model openai/gpt-4o` with input on stdin
- **THEN** the extraction use case SHALL be called with `"openai/gpt-4o"` as the model argument

#### Scenario: Default model when flag omitted

- **WHEN** the user runs `go run ./cmd/extract` without `-model`
- **THEN** the CLI SHALL use the default model resolved from configuration and pass it to the use case

### Requirement: -format flag selects output format

The CLI SHALL accept a `-format` string flag with accepted value `json` (the default). When `-format` is `json`, the result SHALL be marshaled as JSON. Unsupported format values SHALL cause the CLI to print an error to stderr and exit with code 1.

#### Scenario: JSON format default

- **WHEN** the user runs `go run ./cmd/extract` without `-format`
- **THEN** the output SHALL be JSON written to stdout

#### Scenario: Unsupported format rejected

- **WHEN** the user runs `go run ./cmd/extract -format yaml`
- **THEN** the CLI SHALL print an error message to stderr indicating the format is unsupported and exit with code 1

### Requirement: -pretty flag controls JSON indentation

The CLI SHALL accept a `-pretty` bool flag defaulting to `true`. When `-pretty` is true, the JSON SHALL be indented using `json.MarshalIndent` with a 2-space indent. When `-pretty` is false, the JSON SHALL be compact using `json.Marshal`.

#### Scenario: Pretty JSON by default

- **WHEN** the user runs `go run ./cmd/extract` without `-pretty`
- **THEN** the JSON output SHALL be indented with 2-space indentation

#### Scenario: Compact JSON

- **WHEN** the user runs `go run ./cmd/extract -pretty=false`
- **THEN** the JSON output SHALL be compact (no extra whitespace)

### Requirement: Error handling maps typed errors to user messages

The CLI SHALL use `errors.Is` to detect MP3's typed errors and map them to user-facing messages written to stderr. When `errors.Is(err, tools.ErrNoToolCall)` is true, the CLI SHALL print "Model did not return structured data." to stderr. When `errors.Is(err, tools.ErrUnmarshalFailed)` is true, the CLI SHALL print the underlying error and the raw `ToolCall.Arguments` string to stderr for debugging. For any other error, the CLI SHALL print a generic "Extraction failed: <err>" message to stderr. The CLI SHALL exit with code 1 on any error and SHALL NOT write partial JSON to stdout.

#### Scenario: Model returns no tool call

- **WHEN** the extraction use case returns an error where `errors.Is(err, tools.ErrNoToolCall)` is true
- **THEN** the CLI SHALL print "Model did not return structured data." to stderr and exit with code 1

#### Scenario: Arguments fail to unmarshal

- **WHEN** the extraction use case returns an error where `errors.Is(err, tools.ErrUnmarshalFailed)` is true
- **THEN** the CLI SHALL print the error and the raw arguments string to stderr and exit with code 1

#### Scenario: Provider error after retries exhausted

- **WHEN** the extraction use case returns a provider error that is not a typed MP3 error
- **THEN** the CLI SHALL print "Extraction failed: <err>" to stderr and exit with code 1

#### Scenario: No partial output on error

- **WHEN** any error occurs during extraction
- **THEN** the CLI SHALL NOT write any JSON to stdout — only the error message to stderr

### Requirement: Ctrl+C handling via signal.NotifyContext

The CLI SHALL create a context using `signal.NotifyContext(context.Background(), os.Interrupt)` and pass it to `useCase.Extract`. When the user presses Ctrl+C, the context SHALL be cancelled, propagating through `tools.Extract[T]` to `provider.Chat` (and through `RetryProvider`'s backoff waits), causing the in-flight request to abort.

#### Scenario: Ctrl+C aborts extraction

- **WHEN** the user presses Ctrl+C while the extraction use case is waiting for a provider response
- **THEN** the context SHALL be cancelled, the provider request SHALL be aborted, and the CLI SHALL exit without writing JSON to stdout

### Requirement: Provider wiring with RetryProvider

The CLI SHALL construct the provider chain at startup: `configs.LoadConfigs()` → `OpenRouterProvider` (MP0) → `RetryProvider` (MP2) wrapping it → `ExtractUseCase` receiving the retry-wrapped provider. The `ExtractUseCase` SHALL call `tools.Extract[T]` with the retry-wrapped provider so transient failures (429, 5xx) are retried automatically before surfacing an error.

#### Scenario: Retry wraps provider

- **WHEN** the CLI starts up
- **THEN** the `ExtractUseCase` SHALL hold a `RetryProvider`-wrapped `OpenRouterProvider`, and transient provider errors SHALL be retried before the use case returns an error to the CLI
