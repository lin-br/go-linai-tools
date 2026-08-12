## ADDED Requirements

### Requirement: Testdata contains redacted Nubank and Itaú statements

The system SHALL include `internal/document/testdata/` containing at least three redacted Nubank credit-card statements and two redacted Itaú credit-card statements. Each PDF SHALL have a corresponding golden JSON file (e.g., `nubank-2024-01.json`) describing the expected `Statement` output, including expected `Bank`, `PeriodStart`, `PeriodEnd`, transaction count, and total amount. Redaction SHALL replace real names, card numbers, addresses, and transaction identifiers with synthetic values while preserving layout and structure.

#### Scenario: Golden files are parseable

- **WHEN** the test suite reads all `*.json` files in `internal/document/testdata/`
- **THEN** each file SHALL decode into a `Statement` struct without error

#### Scenario: Redaction preserves page count and layout

- **WHEN** a redacted PDF is inspected
- **THEN** it SHALL contain no real personal information and SHALL retain the original bank statement layout (header, transaction table, totals)

### Requirement: Real-world extraction tests validate Nubank and Itaú statements

The system SHALL include extraction tests in `internal/document/extract_test.go` guarded by the `//go:build integration` tag. Each test SHALL call `ExtractStatement` with one redacted PDF and compare the result to its golden `Statement`. Comparisons SHALL use tolerance bands: transaction count MUST match exactly, total amount MUST match within ±1%, and date range (`PeriodStart` and `PeriodEnd`) MUST match exactly. Tests for strategies other than the default SHALL be parameterized so the same assertions run against vision, OCR, and hybrid strategies.

#### Scenario: Nubank statement extraction matches golden

- **WHEN** the integration test runs `ExtractStatement` on `testdata/nubank-2024-01.pdf` with the configured strategy
- **THEN** the returned `Statement.Bank` SHALL equal `"Nubank"`, the transaction count SHALL equal the golden value, and the total amount SHALL be within ±1% of the golden total

#### Scenario: Itaú statement extraction matches golden

- **WHEN** the integration test runs `ExtractStatement` on `testdata/itau-2024-02.pdf` with the configured strategy
- **THEN** the returned `Statement.Bank` SHALL equal `"Itaú"`, the transaction count SHALL equal the golden value, and the total amount SHALL be within ±1% of the golden total

#### Scenario: All three strategies are evaluated

- **WHEN** integration tests are run with the environment variable `DOCUMENT_STRATEGY` set to `vision`, `ocr`, or `hybrid`
- **THEN** the same Nubank and Itaú assertions SHALL execute against the selected strategy

### Requirement: Unit tests cover document package without real providers

The system SHALL include unit tests (no build tag, no external API calls) for `internal/document/` that verify `Statement` JSON round-tripping, `ExtractStatement` error paths (`ErrEmptyPDF`, `ErrValidationFailed`), cache hits/misses, and strategy selection. Provider interactions SHALL be tested with a fake `outbound.Provider` implementation.

#### Scenario: Empty PDF unit test

- **WHEN** `TestExtractStatement_EmptyPDF` runs
- **THEN** it SHALL assert `errors.Is(err, ErrEmptyPDF)` and a zero-value `Statement`

#### Scenario: Validation failure unit test

- **WHEN** `TestExtractStatement_ValidationFailed` runs with a fake provider returning a `Statement` missing `Bank`
- **THEN** it SHALL assert `errors.Is(err, ErrValidationFailed)`

#### Scenario: Cache hit unit test

- **WHEN** `TestExtractStatement_CacheHit` calls `ExtractStatement` twice with identical bytes using a fake provider
- **THEN** it SHALL assert the fake provider's `Chat` was invoked exactly once and the two returned statements are equal

### Requirement: Test suite documents extraction accuracy

The system SHALL add a `README.md` inside `internal/document/testdata/` (or a section in the package doc comment) listing the bank formats tested, the number of pages per statement, the strategy used to generate the golden files, and known limitations (e.g., "Foreign-currency transactions are merged into BRL amounts"). The integration test output SHALL log the strategy name, model name, and per-statement extraction summary.

#### Scenario: README documents test corpus

- **WHEN** a developer opens `internal/document/testdata/README.md`
- **THEN** it SHALL list the Nubank and Itaú PDFs, page counts, redaction method, and the strategy used to create the golden JSON files

#### Scenario: Integration tests log strategy and model

- **WHEN** integration tests run
- **THEN** each test SHALL log the active strategy name, model identifier, PDF filename, transaction count, and total amount
