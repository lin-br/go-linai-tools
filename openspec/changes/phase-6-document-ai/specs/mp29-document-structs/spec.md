## ADDED Requirements

### Requirement: Statement and Transaction structs model a credit-card statement

The system SHALL define `Transaction` and `Statement` structs in `internal/document/statement.go` with exported fields, JSON struct tags, and `validate` tags. `Transaction` SHALL contain `Date` (`string`, `json:"date"`), `Description` (`string`, `json:"description"`), `Amount` (`float64`, `json:"amount"`), and `Category` (`string`, `json:"category"`). `Statement` SHALL contain `Bank` (`string`, `json:"bank"`), `PeriodStart` (`string`, `json:"period_start"`), `PeriodEnd` (`string`, `json:"period_end"`), and `Transactions` (`[]Transaction`, `json:"transactions"`).

#### Scenario: Struct marshals to expected JSON shape

- **WHEN** a `Statement` value with one `Transaction` is marshaled with `json.Marshal`
- **THEN** the resulting JSON SHALL contain top-level keys `bank`, `period_start`, `period_end`, and `transactions`, and each transaction object SHALL contain keys `date`, `description`, `amount`, and `category`

#### Scenario: JSON unmarshals into Statement

- **WHEN** a JSON object matching the `Statement` schema is unmarshaled into `*Statement`
- **THEN** all fields SHALL be populated according to their JSON tags, with `transactions` decoded as a slice of `Transaction`

### Requirement: ExtractStatement provides a single extraction entry point

The system SHALL define `ExtractStatement(ctx context.Context, pdf []byte) (Statement, error)` in `internal/document/extract.go`. The function SHALL validate that `pdf` is non-empty, compute `sha256(pdf)` for deduplication, invoke the configured extraction strategy with the active provider and model, validate the returned `Statement`, and return it. The function SHALL return `ErrEmptyPDF` when `len(pdf) == 0`.

#### Scenario: Empty PDF returns typed error

- **WHEN** `ExtractStatement(ctx, []byte{})` is called
- **THEN** the function SHALL return `ErrEmptyPDF` and a zero-value `Statement`, and the error SHALL be detectable with `errors.Is(err, ErrEmptyPDF)`

#### Scenario: Valid PDF yields a populated Statement

- **WHEN** `ExtractStatement(ctx, validPDFBytes)` is called with a non-empty PDF and the configured strategy returns a valid `Statement`
- **THEN** the function SHALL return the `Statement`, a nil error, and the returned `Statement.Bank` and `Statement.Transactions` fields SHALL be populated

#### Scenario: Context cancellation propagates to strategy

- **WHEN** `ExtractStatement` is called with a cancelled `context.Context`
- **THEN** the strategy SHALL receive the cancelled context and return an error without blocking indefinitely

### Requirement: Strategy interface abstracts PDF ingestion

The system SHALL define a `Strategy` interface in `internal/document/strategy.go` with method `Extract(ctx context.Context, provider outbound.Provider, model string, pdf []byte) (*Statement, error)`. All concrete strategies (vision, OCR, hybrid) SHALL satisfy this interface.

#### Scenario: Compile-time interface compliance

- **WHEN** the package is compiled
- **THEN** there SHALL be a compile-time check such as `var _ Strategy = (*VisionStrategy)(nil)` for each concrete strategy type

#### Scenario: ExtractStatement uses the configured strategy

- **WHEN** `ExtractStatement` is initialized with a specific `Strategy` implementation
- **THEN** it SHALL call that implementation's `Extract` method with the active provider, model string, and raw PDF bytes
