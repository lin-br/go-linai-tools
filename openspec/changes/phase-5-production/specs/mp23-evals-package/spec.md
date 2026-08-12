## ADDED Requirements

### Requirement: Eval package exposes deterministic unit evals

The system SHALL provide package `internal/evals` with a `Case` struct containing `Name string`, `Input string`, `Expected string`, and `Checker Checker`. It SHALL provide a `Runner` type with `Run(ctx context.Context, cases []Case) (Report, error)` that executes each case and records `Pass bool`, `Got string`, and `Error string`. Deterministic checkers SHALL compare strings exactly and SHALL NOT call an LLM.

#### Scenario: Exact-match checker passes when output equals expected

- **WHEN** `Runner.Run` receives a case with `Expected: "hello"` and the function under test returns `"hello"`
- **THEN** the resulting `Report` SHALL contain one entry with `Pass: true`, `Got: "hello"`, and an empty `Error`

#### Scenario: Exact-match checker fails when output differs

- **WHEN** `Runner.Run` receives a case with `Expected: "hello"` and the function under test returns `"world"`
- **THEN** the resulting `Report` SHALL contain one entry with `Pass: false`, `Got: "world"`, and an `Error` describing the mismatch

### Requirement: Eval package supports LLM-as-judge behavioral evals

The system SHALL provide `NewLLMJudgeChecker(provider outbound.Provider, model, rubric string) Checker`. The checker SHALL build a prompt containing the rubric, the expected answer, and the actual answer, call `provider.Chat(ctx, req)`, parse the response for an integer score on a 1–5 scale, and return `Pass: true` when the score is 4 or 5. The checker SHALL return an error if the model response cannot be parsed into an integer.

#### Scenario: High judge score passes

- **WHEN** the LLM returns text containing the score `5`
- **THEN** the checker SHALL return `Pass: true`

#### Scenario: Low judge score fails

- **WHEN** the LLM returns text containing the score `2`
- **THEN** the checker SHALL return `Pass: false`

#### Scenario: Unparseable judge response returns an error

- **WHEN** the LLM returns text that does not contain a digit
- **THEN** the checker SHALL return an error wrapping `ErrUnparseableScore`

### Requirement: Golden dataset of 30 Q/A pairs is stored and loaded

The system SHALL store a golden dataset at `internal/evals/testdata/golden.jsonl` with exactly 30 JSON lines. Each line SHALL contain `query`, `expected_answer`, and `expected_tool` fields. The package SHALL expose `LoadGoldenDataset(path string) ([]Case, error)` that reads the file, unmarshals each line, and maps rows to `Case` values using the appropriate checker (exact-match or LLM-as-judge based on a `judge` boolean field).

#### Scenario: Dataset loads 30 cases

- **WHEN** `LoadGoldenDataset("internal/evals/testdata/golden.jsonl")` is called
- **THEN** it SHALL return a slice of length 30 and no error

#### Scenario: Missing dataset file returns an error

- **WHEN** `LoadGoldenDataset` is called with a non-existent path
- **THEN** it SHALL return an error wrapping `os.ErrNotExist`

### Requirement: Evals run with `go test`

The system SHALL provide `internal/evals/evals_test.go` with `TestGoldenDataset` that loads the golden dataset, runs it through `Runner.Run`, and fails the test if any case does not pass. The test SHALL print a per-case summary including name, pass/fail, and error. Behavioral evals SHALL be skipped when the environment variable `SKIP_LLM_JUDGE` is set, unless a `-llm-judge` flag is explicitly passed.

#### Scenario: All deterministic cases pass

- **WHEN** `go test ./internal/evals/...` runs and all deterministic cases match
- **THEN** the test SHALL pass with no failures

#### Scenario: A deterministic case fails

- **WHEN** `go test ./internal/evals/...` runs and a deterministic case mismatches
- **THEN** the test SHALL fail and print the failing case name, expected, and got values

### Requirement: Eval report is printable

The system SHALL provide `Report.String() string` and `Report.PassRate() float64`. `String` SHALL print each case on its own line with `PASS`/`FAIL`, name, and error (if any). `PassRate` SHALL return `passed / total` as a float64, or `1.0` when total is zero.

#### Scenario: Report prints pass/fail summary

- **WHEN** a report with 2 passing and 1 failing case is converted with `String()`
- **THEN** the output SHALL contain exactly two `PASS` lines, one `FAIL` line, and the failing case error
