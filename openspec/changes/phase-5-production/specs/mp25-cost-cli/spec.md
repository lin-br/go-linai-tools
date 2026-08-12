## ADDED Requirements

### Requirement: Cost CLI reads Langfuse traces for the current week

The system SHALL provide `cmd/cost/main.go`, a CLI that connects to Langfuse using env vars `LANGFUSE_BASE_URL`, `LANGFUSE_PUBLIC_KEY`, and `LANGFUSE_SECRET_KEY`, fetches traces or observations created since the most recent Monday 00:00:00 UTC, and aggregates `cost` metadata by `model` and by `feature` (read from observation metadata). The CLI SHALL print a plain-text table to stdout.

#### Scenario: Weekly spend by model

- **WHEN** the user runs `go run ./cmd/cost`
- **THEN** stdout SHALL contain a table with columns `Model` and `Cost ($)` summing all observations for the current week

#### Scenario: Weekly spend by feature

- **WHEN** the user runs `go run ./cmd/cost -by feature`
- **THEN** stdout SHALL contain a table with columns `Feature` and `Cost ($)` summing all observations grouped by the `feature` metadata field

### Requirement: Cost CLI accepts date range flags

The CLI SHALL accept `-since` and `-until` flags in RFC3339 format. When omitted, `-since` defaults to the most recent Monday 00:00:00 UTC and `-until` defaults to `time.Now()`. Invalid date formats SHALL cause the CLI to print an error to stderr and exit with code 1.

#### Scenario: Custom date range

- **WHEN** the user runs `go run ./cmd/cost -since 2026-08-01T00:00:00Z -until 2026-08-07T23:59:59Z`
- **THEN** the CLI SHALL fetch and aggregate only observations within that range

#### Scenario: Invalid date format rejected

- **WHEN** the user runs `go run ./cmd/cost -since not-a-date`
- **THEN** the CLI SHALL print an error to stderr and exit with code 1

### Requirement: Cost aggregation uses net/http and parses Langfuse JSON

The system SHALL provide `internal/observability/langfuse/cost_client.go` with `CostClient` and `ListObservations(ctx, since, until time.Time) ([]Observation, error)`. The client SHALL use `net/http`, build query parameters `fromTimestamp` and `toTimestamp`, handle pagination via `nextCursor`, and return a slice of observations containing at least `ID`, `Model`, `Cost`, and `Metadata map[string]string`.

#### Scenario: Pagination is followed

- **WHEN** Langfuse returns a response with a non-empty `nextCursor`
- **THEN** the client SHALL make additional requests until `nextCursor` is empty

#### Scenario: HTTP error returns a wrapped error

- **WHEN** Langfuse returns a 500 status
- **THEN** `ListObservations` SHALL return an error wrapping `ErrLangfuseAPI` and include the status code

### Requirement: Output format supports JSON

The CLI SHALL accept `-output` flag with values `table` (default) and `json`. When `-output json`, the CLI SHALL print a JSON array of `{model, feature, cost}` objects to stdout.

#### Scenario: JSON output

- **WHEN** the user runs `go run ./cmd/cost -output json`
- **THEN** stdout SHALL contain a valid JSON array with aggregated cost records

### Requirement: Cost CLI surfaces zero-spend gracefully

When no observations exist in the range, the CLI SHALL print a message indicating zero spend and exit with code 0. It SHALL NOT print an error or exit non-zero.

#### Scenario: No traces in range

- **WHEN** the user runs `go run ./cmd/cost` and Langfuse returns no observations for the current week
- **THEN** the CLI SHALL print `No spend recorded for the selected range.` and exit 0
