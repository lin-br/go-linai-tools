## ADDED Requirements

### Requirement: Benchmark CLI entry point accepts flags

The system SHALL create `cmd/model-roulette/main.go` using the standard library `flag` package. The CLI SHALL accept `-providers` (comma-separated list, default from config), `-models` (comma-separated list, default from config), `-output` (CSV file path, default `model-roulette-results.csv`), `-runs` (integer, default `1`), and `-timeout` (duration, default `60s`). It SHALL read the active provider configuration from `configs.LoadConfigs()` and map provider names to credentials. Diagnostic output SHALL be written to stderr; CSV output SHALL be written to the file specified by `-output`.

#### Scenario: Run with default flags
- **WHEN** the user runs `go run ./cmd/model-roulette`
- **THEN** it SHALL use default providers and models from config, write results to `model-roulette-results.csv`, and print progress to stderr

#### Scenario: Override providers and models via flags
- **WHEN** the user runs `go run ./cmd/model-roulette -providers=anthropic,openai -models=claude-sonnet-4-20250514,gpt-4o`
- **THEN** it SHALL benchmark only the specified provider+model combinations

#### Scenario: Invalid timeout duration rejected
- **WHEN** the user runs `go run ./cmd/model-roulette -timeout=notaduration`
- **THEN** it SHALL print an error to stderr and exit with code 2

### Requirement: Benchmark runner defines ten prompt categories

The system SHALL define a `BenchmarkPrompt` struct in `cmd/model-roulette/prompts.go` with fields `ID string`, `Category string`, `SystemPrompt string`, and `UserPrompt string`. It SHALL define exactly ten prompts with categories: `classification`, `summarization`, `structured-extraction`, `creative`, `code`, `reasoning`, `translation`, `rag-response`, `tool-selection`, and `refusal-boundary`.

#### Scenario: Prompts are enumerable
- **WHEN** `AllPrompts()` is called
- **THEN** it SHALL return a slice of length exactly 10

#### Scenario: Each prompt has a unique ID
- **WHEN** iterating over `AllPrompts()`
- **THEN** every `ID` SHALL be non-empty and unique

### Requirement: Runner invokes every model against every prompt

The system SHALL define a `Runner` struct in `cmd/model-roulette/runner.go` that holds the provider factory, model list, prompt list, and output path. Its `Run(ctx context.Context) error` method SHALL loop over prompts, then over models, build a `domain.ChatRequest` from the prompt, call `provider.Chat(ctx, req)`, and record latency, prompt tokens, completion tokens, and cost. For streaming models it MAY call `ChatStream` and aggregate deltas; direct providers use `Chat`.

#### Scenario: One run produces one CSV row per prompt-model pair
- **WHEN** `Runner.Run(ctx)` is called with 3 models and 10 prompts
- **THEN** it SHALL produce exactly 30 CSV rows plus a header row

#### Scenario: Errors are recorded without aborting the run
- **WHEN** one model returns an error for one prompt
- **THEN** that row SHALL still be written to CSV with empty token/cost fields and the error recorded in a `status` column; the runner SHALL continue with the next prompt-model pair

### Requirement: Latency is measured as wall-clock milliseconds

The system SHALL record latency as `time.Since(start).Milliseconds()` from just before `provider.Chat` (or `ChatStream`) is called until the full response (or final stream event) is received. The value SHALL be stored in the CSV column `latency_ms` as an integer.

#### Scenario: Latency is non-negative integer
- **WHEN** a prompt-model call succeeds
- **THEN** the resulting CSV row SHALL contain `latency_ms >= 0`

### Requirement: Cost is estimated from a static price table

The system SHALL define a price table in `cmd/model-roulette/pricing.go` as `map[string]ModelPricing` where `ModelPricing` has `InputPer1M float64` and `OutputPer1M float64`. The runner SHALL compute `cost_usd = (prompt_tokens * InputPer1M + completion_tokens * OutputPer1M) / 1_000_000`. Models not found in the table SHALL write `""` for `cost_usd`.

#### Scenario: Known model cost is computed
- **WHEN** a model priced at `InputPer1M=3.0` and `OutputPer1M=15.0` produces `prompt_tokens=1000` and `completion_tokens=500`
- **THEN** `cost_usd` SHALL equal `(1000*3.0 + 500*15.0) / 1000000 == 0.0105`

#### Scenario: Unknown model leaves cost blank
- **WHEN** a model is not present in the price table
- **THEN** the CSV row SHALL contain an empty `cost_usd` field

### Requirement: Quality column is empty by default

The system SHALL write the CSV column `quality_1_to_5` as `""` for every row produced by the runner. This column is reserved for human post-run evaluation.

#### Scenario: CSV quality field is empty
- **WHEN** inspecting any row written by the runner
- **THEN** the `quality_1_to_5` column SHALL be empty

### Requirement: Output CSV has the required schema

The system SHALL write the CSV file with a header row `prompt_id, model, latency_ms, prompt_tokens, completion_tokens, cost_usd, quality_1_to_5` followed by one data row per prompt-model result. Token and latency fields SHALL be quoted only when necessary per RFC 4180; `encoding/csv` SHALL be used.

#### Scenario: CSV header matches schema
- **WHEN** the CSV file is opened
- **THEN** the first line SHALL be exactly `prompt_id, model, latency_ms, prompt_tokens, completion_tokens, cost_usd, quality_1_to_5`

#### Scenario: CSV row count matches runs
- **WHEN** running with 2 models, 10 prompts, and 1 run
- **THEN** the CSV SHALL contain 11 lines (1 header + 20 data rows)

### Requirement: Context cancellation aborts the runner

The system SHALL propagate `ctx` through the runner loops. When the context is cancelled (e.g., Ctrl+C via `signal.NotifyContext`), the runner SHALL stop launching new calls and exit `Run` returning the context error.

#### Scenario: Cancelled context stops runner
- **WHEN** the context is cancelled after the first prompt-model pair
- **THEN** no further provider calls SHALL be made and `Run` SHALL return a non-nil error equal to `context.Canceled`

### Requirement: Runner is testable with a fake provider

The system SHALL write table-driven tests in `cmd/model-roulette/runner_test.go` using a fake implementation of `outbound.Provider` that returns deterministic `domain.ChatResponse` values. Tests SHALL verify row count, latency, cost computation, error recording, and context cancellation.

#### Scenario: Fake provider test passes
- **WHEN** running `go test ./cmd/model-roulette/...`
- **THEN** table-driven runner tests SHALL pass
