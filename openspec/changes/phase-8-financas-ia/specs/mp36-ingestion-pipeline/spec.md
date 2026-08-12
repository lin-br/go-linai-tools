## ADDED Requirements

### Requirement: PDF upload is validated before processing

The system SHALL validate every uploaded file in the `POST /ingest` handler before creating an async task. Validation SHALL check that the `Content-Type` is `application/pdf`, the file size does not exceed 10 MB, and the PDF magic bytes (`%PDF`) are present at the start of the file content.

#### Scenario: Valid PDF passes validation

- **WHEN** a client uploads a file with MIME type `application/pdf`, size under 10 MB, and valid `%PDF` header
- **THEN** the handler SHALL create an async task and return `202 Accepted`

#### Scenario: PDF with wrong MIME type but correct magic bytes passes validation

- **WHEN** a client uploads a file whose multipart MIME type is `application/octet-stream` but whose content starts with `%PDF`
- **THEN** the handler SHALL accept the file after magic-byte verification

#### Scenario: File with invalid magic bytes is rejected

- **WHEN** a client uploads a `.txt` file disguised as `.pdf`
- **THEN** the handler SHALL return `400 Bad Request` with `{"error":"invalid pdf file"}`

### Requirement: Ingestion pipeline deduplicates by file hash

The system SHALL compute `sha256` of the uploaded PDF bytes and check `StatementRepository.GetStatementByHash(ctx, hash)` before extraction. If a statement with the same hash exists, the existing `statement_id` SHALL be returned immediately without re-running extraction or embedding.

#### Scenario: Duplicate PDF returns existing statement

- **WHEN** a client uploads a PDF that was already ingested
- **THEN** the task SHALL complete with `status: "completed"` and `result.statement_id` pointing to the existing record

### Requirement: Document extraction reuses internal/document package

The system SHALL call `document.ExtractStatement(ctx, provider, pdfBytes)` (or equivalent function signature from MP29–MP32) to extract a `domain.Statement` containing `bank`, `period_start`, `period_end`, and `transactions`. The extraction SHALL run inside the async worker goroutine, not in the HTTP handler.

#### Scenario: Extraction produces Statement struct

- **WHEN** the async worker processes a valid Nubank PDF
- **THEN** `document.ExtractStatement` SHALL return a `Statement` with `bank`, `period`, and `transactions` populated

### Requirement: Extracted transactions are chunked for retrieval

The system SHALL chunk the statement text and each transaction using the `internal/rag/chunk` package (recursive character split + contextual chunking). Each chunk SHALL include a 1-sentence document summary prepended to its content. Chunks SHALL be associated with the `statement_id`.

#### Scenario: Statement produces multiple chunks

- **WHEN** a statement with 50 transactions is ingested
- **THEN** the system SHALL create at least one chunk per transaction plus a header chunk

### Requirement: Embeddings are generated via Voyage client

The system SHALL call `internal/rag/embeddings.Client.Embed(ctx, []string)` with `voyage-3-large` (or model configured by `EMBEDDING_MODEL`) to generate 1024-dimensional vectors for all chunks. The embeddings client SHALL reuse the existing Phase 2 Voyage implementation.

#### Scenario: Embedding batch succeeds

- **WHEN** 10 chunks are submitted to the embeddings client
- **THEN** it SHALL return a slice of 10 vectors, each with length 1024

#### Scenario: Embedding failure marks task failed

- **WHEN** the embeddings client returns an error
- **THEN** the async task SHALL transition to `failed` and store the error message

### Requirement: Embeddings and chunks are persisted in a single transaction

The system SHALL insert `chunks` rows and their corresponding `statement_embeddings` rows inside a Postgres transaction. If any insert fails, the transaction SHALL roll back so the database is never left with orphan chunks or embeddings.

#### Scenario: Chunk insert fails, no embeddings persisted

- **WHEN** a chunk insert fails due to a constraint violation
- **THEN** the transaction SHALL roll back and no embeddings SHALL be persisted

### Requirement: Async task state is queryable

The system SHALL store task state with fields `id`, `status`, `message`, `result`, `error`, `created_at`, `updated_at`. The worker SHALL update `status` from `pending` → `running` → `completed`/`failed` and set `message` to human-readable progress messages.

#### Scenario: Task transitions through statuses

- **WHEN** a PDF is uploaded
- **THEN** `GET /tasks/{id}` SHALL initially show `pending`, then `running`, then `completed` after successful ingestion

### Requirement: Worker handles panics gracefully

The async worker SHALL recover from panics using `defer` + `recover()`, mark the task as `failed`, and log the panic stack with `slog`. It SHALL NOT crash the entire process if a single ingestion panics.

#### Scenario: Panic during extraction is recovered

- **WHEN** a bug causes `document.ExtractStatement` to panic inside the worker
- **THEN** the task SHALL be marked `failed`, the process SHALL remain running, and the panic SHALL be logged
