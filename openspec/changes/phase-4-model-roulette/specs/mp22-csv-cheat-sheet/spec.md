## ADDED Requirements

### Requirement: CSV writer is reusable and tested

The system SHALL define a `CSVWriter` in `cmd/model-roulette/writer.go` with constructor `func NewCSVWriter(w io.Writer) *CSVWriter` and method `func (w *CSVWriter) WriteResult(r Result) error`. The `Result` struct SHALL have fields `PromptID string`, `Model string`, `LatencyMs int64`, `PromptTokens int64`, `CompletionTokens int64`, `CostUSD float64`, `Quality int`, and `Err error`. The writer SHALL write the header on construction and one CSV row per result, using `encoding/csv`. Errors SHALL be returned wrapped with `fmt.Errorf("...: %w", err)`.

#### Scenario: Writer emits correct header
- **WHEN** `NewCSVWriter(buf)` is created
- **THEN** the underlying writer SHALL contain the header `prompt_id, model, latency_ms, prompt_tokens, completion_tokens, cost_usd, quality_1_to_5`

#### Scenario: Writer emits result row
- **WHEN** `WriteResult(Result{PromptID: "p1", Model: "claude", LatencyMs: 123, PromptTokens: 10, CompletionTokens: 20, CostUSD: 0.001})` is called
- **THEN** the next CSV line SHALL contain `p1, claude, 123, 10, 20, 0.001,`

#### Scenario: Writer handles errors
- **WHEN** `WriteResult` is called after the underlying writer returns an error
- **THEN** it SHALL return an error wrapping the underlying error

### Requirement: Result row maps errors to a status column

The system SHALL add a `status` column to the CSV after `quality_1_to_5`. When `Result.Err` is non-nil, `cost_usd` and `quality_1_to_5` SHALL be empty, `latency_ms` SHALL record the time spent before failure if measurable, and `status` SHALL be `"error"`. When `Result.Err` is nil, `status` SHALL be `"ok"`.

#### Scenario: Error result row
- **WHEN** `WriteResult(Result{PromptID: "p1", Model: "claude", Err: errors.New("timeout")})` is called
- **THEN** the CSV row SHALL be `p1, claude, , , , , , error`

### Requirement: CSV file path is configurable

The system SHALL allow the user to specify the output CSV file via the `-output` flag. The runner SHALL create the file if it does not exist and truncate it if it does. Failure to create or write the file SHALL produce an error to stderr and exit code 1.

#### Scenario: Default output path
- **WHEN** the user runs `go run ./cmd/model-roulette` without `-output`
- **THEN** it SHALL write to `model-roulette-results.csv` in the current working directory

#### Scenario: Custom output path
- **WHEN** the user runs `go run ./cmd/model-roulette -output /tmp/roulette.csv`
- **THEN** it SHALL write results to `/tmp/roulette.csv`

#### Scenario: Unwritable path returns error
- **WHEN** the user runs `go run ./cmd/model-roulette -output /nonexistent/dir/out.csv`
- **THEN** it SHALL print an error to stderr and exit with code 1

### Requirement: docs/model-selection.md is created as the permanent cheat sheet

The system SHALL create `docs/model-selection.md` with sections: "When to pick which model", "Latency vs quality trade-offs", "Cost per 1M tokens", "Prompt category notes", and "Refresh log". The document SHALL contain concrete guidance for the ten benchmark categories and example models for each provider. The "Refresh log" SHALL start with the Phase 4 completion date.

#### Scenario: Cheat sheet has required sections
- **WHEN** `docs/model-selection.md` is read
- **THEN** it SHALL contain markdown headings for all five required sections

#### Scenario: Cheat sheet maps categories to model families
- **WHEN** reading the "Prompt category notes" section
- **THEN** it SHALL contain one paragraph or bullet per benchmark category: classification, summarization, structured extraction, creative, code, reasoning, translation, RAG response, tool selection, refusal boundary

### Requirement: Cheat sheet explains the CSV columns

The system SHALL document each CSV column in `docs/model-selection.md` under a "Interpreting the CSV" section. It SHALL explain that `quality_1_to_5` is human-rated after the run and that cost is an estimate.

#### Scenario: CSV documentation present
- **WHEN** reading the "Interpreting the CSV" section
- **THEN** it SHALL describe `prompt_id`, `model`, `latency_ms`, `prompt_tokens`, `completion_tokens`, `cost_usd`, and `quality_1_to_5`

### Requirement: Writer package has unit tests

The system SHALL add `cmd/model-roulette/writer_test.go` with table-driven tests covering header emission, successful result rows, error result rows, and underlying writer errors.

#### Scenario: Writer tests pass
- **WHEN** running `go test ./cmd/model-roulette/...`
- **THEN** all table-driven tests in `writer_test.go` SHALL pass
