## ADDED Requirements

### Requirement: Statement schema is generated from the Statement struct

The system SHALL provide a function in `internal/document/schema.go` that builds a `tools.ToolSchema` from the `Statement` struct using `tools.SchemaFromStruct(&Statement{})`. The schema name SHALL be `"extract_statement"` and the description SHALL explain that the tool extracts a credit-card statement. This schema SHALL be used by all three extraction strategies when calling `tools.Extract[Statement]`.

#### Scenario: Schema generation at startup

- **WHEN** the document package is initialized
- **THEN** `SchemaFromStruct(&Statement{})` SHALL be called once and the resulting `ToolSchema` SHALL be cached for reuse across calls

#### Scenario: Schema fields match Statement struct

- **WHEN** the generated schema is inspected
- **THEN** it SHALL contain top-level properties `bank`, `period_start`, `period_end`, and `transactions`

### Requirement: Extracted Statement is validated with go-playground/validator

The system SHALL validate every `Statement` produced by a strategy using `github.com/go-playground/validator/v10`. The `Bank`, `PeriodStart`, and `PeriodEnd` fields SHALL be marked `validate:"required"`. The `Transactions` slice SHALL be marked `validate:"required"` and, when non-empty, each transaction SHALL be validated with `validate:"dive"` such that `Date`, `Description`, and `Amount` are required. Validation failures SHALL wrap `ErrValidationFailed` and include the validator error messages.

#### Scenario: Valid statement passes validation

- **WHEN** a `Statement` has `Bank`, `PeriodStart`, `PeriodEnd`, and at least one `Transaction` with `Date`, `Description`, and `Amount`
- **THEN** validation SHALL succeed and `ExtractStatement` SHALL return the statement

#### Scenario: Missing bank returns validation error

- **WHEN** a strategy returns a `Statement` with an empty `Bank` field
- **THEN** `ExtractStatement` SHALL return `ErrValidationFailed` detectable with `errors.Is(err, ErrValidationFailed)` and the error message SHALL mention the `Bank` field

#### Scenario: Empty transactions returns validation error

- **WHEN** a strategy returns a `Statement` with `Transactions == nil` or zero length
- **THEN** `ExtractStatement` SHALL return `ErrValidationFailed`

### Requirement: Duplicate PDFs are detected by sha256 hash

The system SHALL compute `sha256.Sum256(pdf)` for every non-empty input. It SHALL maintain an in-memory deduplication cache keyed by the lower-case hexadecimal string of the hash. If the same PDF bytes are submitted again, `ExtractStatement` SHALL return the cached `Statement` without re-invoking the provider or OCR helper. The cache interface SHALL be small enough to be replaced by a persistent store in Phase 8 without changing `ExtractStatement`.

#### Scenario: Same bytes return cached result

- **WHEN** `ExtractStatement(ctx, pdfBytes)` succeeds and is called a second time with the same `pdfBytes`
- **THEN** the provider SHALL be invoked exactly once and the second call SHALL return the cached `Statement`

#### Scenario: Different bytes bypass cache

- **WHEN** `ExtractStatement` is called with two different PDF byte slices
- **THEN** the provider SHALL be invoked twice and two distinct `Statement` values SHALL be returned

#### Scenario: Cache interface is swappable

- **WHEN** inspecting the document package
- **THEN** there SHALL exist a `Cache` interface with `Get(key string) (Statement, bool)` and `Set(key string, value Statement)` methods, and `ExtractStatement` SHALL depend on this interface rather than a concrete map
