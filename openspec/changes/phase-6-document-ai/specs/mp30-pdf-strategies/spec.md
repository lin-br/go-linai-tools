## ADDED Requirements

### Requirement: VisionStrategy sends base64 PDF to a multimodal model

The system SHALL provide `VisionStrategy` in `internal/document/strategy.go` that implements the `Strategy` interface. It SHALL encode the raw PDF bytes using standard base64, build a `domain.ChatRequest` with a user message containing the encoded PDF and a prompt asking the model to extract the statement, and call `provider.Chat` with a forced tool choice referencing the `Statement` schema. The system prompt SHALL instruct the model to return a `Statement` tool call.

#### Scenario: VisionStrategy encodes PDF as base64

- **WHEN** `VisionStrategy.Extract(ctx, provider, model, pdfBytes)` is called with a 4-byte PDF slice
- **THEN** the `domain.ChatRequest` passed to `provider.Chat` SHALL contain a user message whose content includes the standard base64 encoding of those bytes

#### Scenario: VisionStrategy requests Statement tool

- **WHEN** `VisionStrategy.Extract` is called
- **THEN** the request's `ToolChoice` SHALL be `{Type: "tool", Name: "extract_statement"}` (or the schema's defined name) and the single tool's `InputSchema` SHALL match the generated `Statement` schema

#### Scenario: VisionStrategy decodes tool result into Statement

- **WHEN** the provider returns a `ToolCall` whose arguments match the `Statement` schema
- **THEN** `VisionStrategy.Extract` SHALL return a `*Statement` with fields populated from the arguments

### Requirement: OCRStrategy extracts text via pdfplumber through os/exec

The system SHALL provide `OCRStrategy` in `internal/document/strategy.go` that writes the PDF to a temporary file with permissions `0600`, invokes `python scripts/ocr.py <tempfile>` using `exec.Command` with the path as a separate argument (not through a shell), reads stdout as the extracted text, and feeds that text to `tools.Extract[Statement]` with a text-only prompt. If `scripts/ocr.py` exits non-zero or is not found, it SHALL return `ErrExtractionFailed` wrapping the underlying error.

#### Scenario: OCRStrategy runs pdfplumber helper

- **WHEN** `OCRStrategy.Extract(ctx, provider, model, pdfBytes)` is called and the `scripts/ocr.py` helper prints "Pagamento recebido\nTransacao 1" to stdout
- **THEN** the strategy SHALL pass that exact text as the user input to `tools.Extract[Statement]`

#### Scenario: Missing pdfplumber helper returns clear error

- **WHEN** `OCRStrategy.Extract` is called but `python scripts/ocr.py` is not installed or exits with code 1
- **THEN** it SHALL return a non-nil error wrapping `ErrExtractionFailed`, detectable with `errors.Is(err, ErrExtractionFailed)`

#### Scenario: Temporary file is cleaned up

- **WHEN** `OCRStrategy.Extract` finishes, regardless of success or failure
- **THEN** the temporary PDF file SHALL be removed from disk

#### Scenario: Command injection is prevented

- **WHEN** `OCRStrategy.Extract` is called with PDF bytes whose filename would otherwise contain shell metacharacters
- **THEN** the helper SHALL still be invoked with a separate, sanitized temporary file path and no shell interpretation SHALL occur

### Requirement: HybridStrategy combines OCR text with vision for tables

The system SHALL provide `HybridStrategy` in `internal/document/strategy.go`. It SHALL first run OCR via the same mechanism as `OCRStrategy` to obtain baseline text, then send the original PDF bytes plus the OCR text to a vision model with a prompt instructing the model to use the OCR text as context while correcting and completing the transactions table from the PDF. The final output SHALL be the `Statement` returned by `tools.Extract[Statement]`.

#### Scenario: HybridStrategy uses both OCR and vision

- **WHEN** `HybridStrategy.Extract(ctx, provider, model, pdfBytes)` is called
- **THEN** the strategy SHALL invoke the OCR helper once and the provider vision request once, and the vision request's user message SHALL contain both the base64-encoded PDF and a reference to the OCR text

#### Scenario: OCR failure aborts hybrid extraction

- **WHEN** the OCR helper fails during `HybridStrategy.Extract`
- **THEN** the strategy SHALL return `ErrExtractionFailed` and SHALL NOT make the vision request

### Requirement: Strategy selection is configurable

The system SHALL read the active strategy from configuration. The default strategy name SHALL be `"vision"`. Supported names SHALL be `"vision"`, `"ocr"`, and `"hybrid"`. An unsupported strategy name SHALL cause `ExtractStatement` to return an error at construction time, not at call time.

#### Scenario: Default strategy is vision

- **WHEN** no strategy override is configured
- **THEN** `ExtractStatement` SHALL use `VisionStrategy`

#### Scenario: Config overrides strategy to OCR

- **WHEN** the configuration sets `document.strategy = "ocr"`
- **THEN** `ExtractStatement` SHALL use `OCRStrategy`

#### Scenario: Invalid strategy name rejected early

- **WHEN** the configuration sets `document.strategy` to an unsupported value such as `"teapot"`
- **THEN** package initialization SHALL return an error before any PDF is processed
