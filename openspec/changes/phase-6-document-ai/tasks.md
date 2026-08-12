## 1. MP29 — Document structs and extraction API

- [ ] 1.1 Create `internal/document/statement.go` with `Transaction` and `Statement` structs, JSON tags, and `validate` tags
- [ ] 1.2 Create `internal/document/errors.go` with typed sentinel errors `ErrEmptyPDF`, `ErrExtractionFailed`, `ErrValidationFailed`
- [ ] 1.3 Create `internal/document/schema.go` to generate and cache `tools.ToolSchema` from `Statement` via `tools.SchemaFromStruct`
- [ ] 1.4 Create `internal/document/cache.go` with a `Cache` interface and an in-memory `mapCache` implementation keyed by `sha256`
- [ ] 1.5 Create `internal/document/strategy.go` with the `Strategy` interface
- [ ] 1.6 Create `internal/document/extract.go` with `ExtractStatement(ctx context.Context, pdf []byte) (Statement, error)`
- [ ] 1.7 Wire `ExtractStatement` to load config, dedup, call strategy, and validate the result
- [ ] 1.8 Add unit tests in `internal/document/statement_test.go` for JSON round-trip and validation tags

## 2. MP30 — PDF ingestion strategies

- [ ] 2.1 Implement `VisionStrategy.Extract` in `internal/document/strategy.go` — base64 PDF, multimodal prompt, `tools.Extract[Statement]`
- [ ] 2.2 Create `scripts/ocr.py` — minimal `pdfplumber` script that reads a PDF path argument and prints extracted text
- [ ] 2.3 Implement `OCRStrategy.Extract` — write temp PDF with `0600` permissions, invoke `python scripts/ocr.py <path>` via `exec.Command`, read stdout, feed text to `tools.Extract[Statement]`, cleanup temp file
- [ ] 2.4 Implement `HybridStrategy.Extract` — run OCR, then vision request with OCR text context, return `tools.Extract[Statement]` result
- [ ] 2.5 Add strategy selection helper in `internal/document/strategy.go` (`NewStrategy(name string) (Strategy, error)`)
- [ ] 2.6 Add unit tests for each strategy using a fake `outbound.Provider` and small synthetic PDF bytes
- [ ] 2.7 Verify `go build ./...` passes and `go vet ./...` has no warnings

## 3. MP31 — Extraction, validation, and dedup

- [ ] 3.1 Add `github.com/go-playground/validator/v10` to `go.mod`
- [ ] 3.2 Create `internal/document/validate.go` with a `Validator` wrapper around `validator.Validate`
- [ ] 3.3 Ensure `Statement` and `Transaction` fields have correct `validate` tags (`required`, `dive`)
- [ ] 3.4 Integrate validation into `ExtractStatement` so strategy output is validated before return
- [ ] 3.5 Ensure `ErrValidationFailed` is returned with `errors.Is` compatibility and includes field-level details
- [ ] 3.6 Implement `sha256` hashing in `ExtractStatement` and use the `Cache` interface to skip re-extraction
- [ ] 3.7 Add unit tests for cache hit/miss, validation success, and validation failure paths
- [ ] 3.8 Run `go test ./internal/document/...` and confirm all unit tests pass

## 4. MP32 — Real-world tests

- [ ] 4.1 Create `internal/document/testdata/` directory
- [ ] 4.2 Add 3 redacted Nubank PDFs and 3 corresponding golden JSON files
- [ ] 4.3 Add 2 redacted Itaú PDFs and 2 corresponding golden JSON files
- [ ] 4.4 Create `internal/document/testdata/README.md` documenting corpus, redaction method, page counts, and golden generation strategy
- [ ] 4.5 Create `internal/document/extract_integration_test.go` with `//go:build integration`
- [ ] 4.6 Write table-driven integration tests that compare extraction results to golden files with transaction-count exact match, total amount ±1%, and exact date-range match
- [ ] 4.7 Parameterize integration tests by `DOCUMENT_STRATEGY` environment variable (`vision`, `ocr`, `hybrid`)
- [ ] 4.8 Log strategy, model, filename, transaction count, and total amount for each integration test run
- [ ] 4.9 Run integration tests for each strategy and record pass/fail per bank
- [ ] 4.10 Update `docs/roadmap-ai-engineer-status.md` to mark Phase 6 as COMPLETE once tests pass
