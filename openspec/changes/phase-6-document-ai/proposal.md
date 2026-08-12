## Why

Phase 6 of the AI Engineer roadmap is the core data-pipeline milestone for the eventual Finanças IA showcase: it must turn a credit-card PDF into clean accounting JSON. MP0–MP3 already provide a working `outbound.Provider`, structured-output `tools.Extract[T]`, and retry logic, but the repo has no document-AI surface yet. This change closes that gap by adding a reusable `internal/document/` package that abstracts PDF ingestion behind a single `ExtractStatement` function and validates real-world extraction quality against redacted Nubank and Itaú statements.

## What Changes

- Add `internal/document/` package with `ExtractStatement(pdf []byte) (Statement, error)`.
- Add `internal/document/statement.go` defining `Transaction` and `Statement` structs with JSON tags.
- Add `internal/document/strategy.go` with a `Strategy` interface and three implementations:
  - `VisionStrategy` — sends the base64 PDF directly to a multimodal model (Claude via OpenRouter/Anthropic).
  - `OCRStrategy` — calls a Python `pdfplumber` script through `os/exec` and feeds the extracted text to the LLM.
  - `HybridStrategy` — runs OCR for raw text plus a vision model pass focused on tables.
- Add `internal/document/schema.go` producing a tool-use `input_schema` matching `Statement` for `tools.Extract[T]`.
- Add `internal/document/validate.go` using `github.com/go-playground/validator/v10` to validate populated `Statement` structs.
- Add `internal/document/dedup.go` computing `sha256` over raw PDF bytes so the same file is not re-processed.
- Add typed errors: `ErrEmptyPDF`, `ErrExtractionFailed`, `ErrValidationFailed`.
- Add `internal/document/testdata/` with 3 redacted Nubank statements and 2 redacted Itaú statements plus golden JSON files.
- Add table-driven unit tests in `internal/document/` and integration-style extraction tests guarded by build tags.
- No breaking changes to `internal/core/domain/`, `internal/core/ports/`, `internal/core/tools/`, or existing CLIs.

## Capabilities

### New Capabilities

- `mp29-document-structs`: `Statement` and `Transaction` domain structs plus the top-level `ExtractStatement` API.
- `mp30-pdf-strategies`: Pluggable PDF ingestion strategies — vision, OCR via `pdfplumber`, and hybrid.
- `mp31-extraction-validation`: Tool-use schema generation, `go-playground/validator` validation, and `sha256` deduplication.
- `mp32-real-world-tests`: Real-world extraction accuracy tests against redacted Nubank and Itaú PDFs.

### Modified Capabilities

(No existing specs are modified. Phase 6 consumes the MP0 `Provider` interface and MP3 `tools.Extract[T]`/`SchemaFromStruct` as-is.)

## Impact

- **New files**:
  - `internal/document/statement.go` — `Transaction`, `Statement` structs.
  - `internal/document/strategy.go` — `Strategy` interface + vision/OCR/hybrid implementations.
  - `internal/document/extract.go` — `ExtractStatement` orchestration.
  - `internal/document/schema.go` — JSON schema for tool-use extraction.
  - `internal/document/validate.go` — struct validation.
  - `internal/document/dedup.go` — SHA-256 deduplication.
  - `internal/document/errors.go` — typed sentinel errors.
  - `internal/document/extract_test.go` — unit + integration tests.
  - `internal/document/testdata/` — redacted PDFs and golden JSON.
  - `scripts/ocr.py` — minimal `pdfplumber` OCR script invoked by `OCRStrategy`.
- **New Go dependencies**:
  - `github.com/go-playground/validator/v10` for struct validation.
  - Standard library only otherwise (`os/exec`, `crypto/sha256`, `encoding/hex`, etc.).
- **New external dependency**:
  - Python `pdfplumber` package for the OCR strategy (managed outside Go modules; install via `pip install pdfplumber`).
- **No breaking changes** — purely additive package.
- **Enables** Phase 8 `Finanças IA` ingestion pipeline (PDF upload → `ExtractStatement` → chunks → pgvector).
