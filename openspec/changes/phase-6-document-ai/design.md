## Context

MP0–MP3 established the provider boundary (`outbound.Provider.Chat`/`ChatStream`), `tools.Extract[T]` for forced-tool-choice structured extraction, and `RetryProvider` for transient retries. Phase 6 is the first multimodal capability: it must accept raw PDF bytes, turn them into a typed `Statement`, and do so reliably enough to feed the Finanças IA ingestion pipeline.

The package is intentionally scoped as a library (`internal/document/`) rather than a CLI. The showcase project will call `ExtractStatement` from an HTTP handler; for Phase 6 the deliverable is the package plus a real-world test suite.

## Goals / Non-Goals

**Goals:**
- Provide a single entry point `ExtractStatement(pdf []byte) (Statement, error)` that hides strategy details.
- Define stable `Transaction`/`Statement` structs with JSON tags suitable for downstream storage and serialization.
- Support three pluggable ingestion strategies: vision-only, OCR-only, hybrid.
- Guarantee valid JSON shape via tool-use `input_schema` matching `Statement`.
- Validate populated structs with `go-playground/validator`.
- Detect duplicate files by `sha256(file bytes)`.
- Ship tests against 3 redacted Nubank + 2 redacted Itaú statements with golden expectations.
- Keep Go code using `net/http` for provider calls; Python is only the OCR helper process.

**Non-Goals:**
- A user-facing CLI in Phase 6 (the CLI surface is Phase 1; Phase 8 will use `ExtractStatement` from HTTP).
- Layout analysis, table detection, or PDF rendering in Go — OCR is delegated to `pdfplumber`.
- Production persistence (pgvector, S3) — that is Phase 8.
- Authenticating or decrypting password-protected PDFs.
- Full end-to-end Finanças IA deployment.

## Decisions

### D1: Single entry point — `ExtractStatement`

All callers use `ExtractStatement(ctx, pdf)`. Internally it picks the configured strategy, validates input, hashes for dedup, and delegates extraction. The function signature is stable even as strategies change.

**Why:** A single function is the easiest contract for Phase 8 to consume. Strategy selection can be driven by config or by caller-provided options later without changing the signature.

**Alternative considered:** Expose three separate functions (`ExtractWithVision`, `ExtractWithOCR`, `ExtractWithHybrid`). Rejected — it leaks strategy details to callers and makes the Phase 8 ingestion pipeline harder to evolve.

### D2: Strategy interface

```go
type Strategy interface {
    Extract(ctx context.Context, provider outbound.Provider, model string, pdf []byte) (*Statement, error)
}
```

`VisionStrategy`, `OCRStrategy`, and `HybridStrategy` implement this interface.

**Why:** The best extraction approach depends on PDF quality (scanned vs. text-based, tables vs. narrative). An interface lets us swap strategies without touching `ExtractStatement`.

**Alternative considered:** Strategy as a string switch inside `ExtractStatement`. Rejected — it complicates the function, makes testing each branch harder, and prevents injecting test doubles.

### D3: Vision strategy — base64 PDF as image/document

`VisionStrategy` builds a `domain.ChatRequest` with a user message whose content combines a text prompt and the base64-encoded PDF. The provider must be a multimodal model (e.g., `anthropic/claude-sonnet-4-20250514`). The system prompt instructs the model to return a tool call matching the `Statement` schema.

**Why:** Modern multimodal models read text-based and scanned PDFs directly with good accuracy on simple layouts. It keeps the stack 100% Go (no Python) when the PDF is clean.

**Risk accepted:** Large PDFs may exceed context limits or model token budgets. Callers must manage model selection and PDF size.

### D4: OCR strategy — `pdfplumber` via `os/exec`

`OCRStrategy` writes the PDF to a temporary file, invokes `python scripts/ocr.py <path>` with separate arguments, reads stdout, and feeds the resulting text into `tools.Extract[T]` with a text-only prompt.

**Why:** `pdfplumber` extracts text from native PDFs accurately and preserves line/column structure better than some vision models on dense tables. It is the only Python dependency allowed by the roadmap.

**Security decision:** `os/exec.Command` uses a fixed script path and passes the PDF path as a separate argument, never via shell concatenation. The script only reads the file and prints extracted text to stdout. `scripts/ocr.py` SHALL validate that the input path is under a known temporary directory and SHALL NOT follow symlinks.

**Alternative considered:** Use a pure-Go PDF text extractor. Rejected — the roadmap explicitly calls for `pdfplumber` as the learning exercise.

### D5: Hybrid strategy — OCR text + vision for tables

`HybridStrategy` runs OCR first to obtain full text, then makes a second request to a vision model with a prompt like: "Use this OCR text as context, but correct and complete the transactions table from the attached PDF." The final output is the tool-call `Statement`.

**Why:** Vision can misread table rows or skip pages; OCR can misorder multi-column tables. Combining both usually outperforms either alone on complex statements.

**Trade-off:** Two provider calls = higher latency and cost. This is acceptable for a batch ingestion use case and can be optimized later.

### D6: Tool-use schema from `Statement`

The `Statement` schema is generated via `tools.SchemaFromStruct` (MP3) and passed to `tools.Extract[Statement]`. The model is forced to call the tool, so the arguments must decode into the Go struct.

**Why:** Reuses the existing structured-output machinery and guarantees valid JSON matching the struct.

**Limitation accepted:** `SchemaFromStruct` is shallow on nested arrays (`[]Transaction` will be `{type: "array"}`). To compensate, the system prompt describes the expected transaction fields explicitly.

### D7: Validation with `go-playground/validator`

After extraction, `ExtractStatement` runs `validator.Validate.Struct(statement)`. Fields use tags such as `validate:"required"` for bank, period start/end, and `validate:"required,dive"` for transactions.

**Why:** A model can return syntactically valid JSON that is semantically incomplete (empty period, zero transactions). Validation catches that before the caller persists it.

**Error contract:** Validation failures wrap `ErrValidationFailed` and include per-field error messages.

### D8: Dedup by `sha256` over raw bytes

`ExtractStatement` computes `sha256(pdf)` before extraction. If a cache map/file records that hash, it returns the previously extracted `Statement`.

**Why:** Statement extraction is expensive (multimodal tokens or OCR + LLM). Avoiding re-processing the same upload is a cheap cost win.

**Phase 6 scope:** The cache is an in-memory map keyed by hash, exposed behind a small `Cache` interface. Phase 8 can replace it with a Postgres-backed cache without changing `ExtractStatement`.

### D9: Structs live in `internal/document/`, not `internal/core/domain/`

`Transaction` and `Statement` are document-extraction-specific types. They are not shared by chat, RAG, or agent packages, so they belong with the document package.

**Why:** Keeps `internal/core/domain/` provider-agnostic. `Statement` is a domain model of the document extraction bounded context, not the LLM provider bounded context.

### D10: Error handling — typed sentinel errors

`errors.go` defines:
- `ErrEmptyPDF` — input slice is zero length.
- `ErrExtractionFailed` — strategy failed to produce a `Statement`.
- `ErrValidationFailed` — extraction succeeded but struct validation failed.

All errors are wrapped with `fmt.Errorf("...: %w", err)` so callers can use `errors.Is`.

**Why:** Callers (Phase 8 HTTP handler) need to distinguish retryable extraction failures from bad input from validation problems. Typed errors make that possible without string matching.

### D11: Tests with redacted real-world PDFs

`internal/document/testdata/` holds 3 Nubank + 2 Itaú credit-card statements that have been redacted: replace real names, card numbers, and transaction details with synthetic placeholders. Golden JSON files describe the expected `Statement` shape. Tests verify that extracted transaction counts, date ranges, and totals are within tolerance of the golden files.

**Why:** Synthetic PDFs do not exercise real-world layout noise (column alignment, page breaks, bank-specific headers). Redacted real statements give confidence without exposing personal data.

## Risks / Trade-offs

- **[PDFs exceed model context window]** → Mitigation: document a max-size recommendation; future work can split statements by page or month.
- **[`pdfplumber` not installed]** → Mitigation: `OCRStrategy` returns a clear error if `python scripts/ocr.py` exits non-zero or is not found. README documents `pip install pdfplumber`.
- **[Model refuses to read financial documents]** → Mitigation: `ErrExtractionFailed` is returned; caller can retry with a different model/strategy.
- **[Validation too strict early on]** → Mitigation: start with `required` on bank, period dates, and at least one transaction; loosen tags if real-world tests show acceptable partial output.
- **[Vision/OCR model drift]** → Mitigation: golden tests use tolerance bands (transaction count ±1, total ±5%) rather than exact byte matches.
- **[Security: temporary files]** → Mitigation: use `os.CreateTemp`, restrict permissions to `0600`, delete after use, validate path is local.

## Open Questions

- Which exact OpenRouter model string becomes the default for vision? (`anthropic/claude-sonnet-4-20250514` is a safe starting guess; confirm availability at implementation time.)
- Should `ExtractStatement` accept an options struct for strategy/model selection, or should it read from `configs.Models.Get()` like the existing CLIs? Decision: read from config by default, accept optional functional options for overrides in future work.
