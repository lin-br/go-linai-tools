## ADDED Requirements

### Requirement: Postgres connection pool is configured with pgx

The system SHALL create a `*pgxpool.Pool` in `cmd/financas-ia/main.go` using the connection string from `DATABASE_URL` environment variable. The pool SHALL be configured with `MaxConns`, `MinConns`, `MaxConnLifetime`, and `MaxConnIdleTime` values loaded from environment variables or sensible defaults (`MaxConns=25`, `MinConns=5`, `MaxConnLifetime=30m`, `MaxConnIdleTime=5m`).

#### Scenario: Pool connects on startup

- **WHEN** the application starts with a valid `DATABASE_URL`
- **THEN** it SHALL successfully create a `pgxpool.Pool` and ping the database before starting the HTTP server

#### Scenario: Missing DATABASE_URL fails fast

- **WHEN** the application starts without `DATABASE_URL`
- **THEN** it SHALL log an error and exit with code 1 before accepting requests

### Requirement: Migrations create statements, chunks, and embeddings tables

The system SHALL provide versioned SQL migration files under `internal/financas/repository/postgres/migrations/` that create the `pgvector` extension and the tables `statements`, `chunks`, and `statement_embeddings`. The migrations SHALL be applied at startup by `cmd/financas-ia/main.go` using `golang-migrate` or an equivalent embedded runner.

#### Scenario: Migrations apply on first startup

- **WHEN** the application starts against an empty Postgres database
- **THEN** it SHALL create the `statements`, `chunks`, and `statement_embeddings` tables and the `vector` extension

#### Scenario: Re-running migrations is idempotent

- **WHEN** the application starts against a database that already has the latest migrations
- **THEN** it SHALL succeed without re-running destructive changes

### Requirement: statements table stores PDF metadata and content

The system SHALL define a `statements` table with columns: `id uuid PRIMARY KEY`, `file_hash sha256 UNIQUE`, `filename text`, `bank text`, `period_start date`, `period_end date`, `raw_text text`, `raw_pdf bytea`, `created_at timestamptz`, `updated_at timestamptz`. The table SHALL enforce uniqueness on `file_hash` to prevent duplicate ingestion.

#### Scenario: Insert statement with all fields

- **WHEN** the repository inserts a `Statement` record after extraction
- **THEN** the row SHALL contain `bank`, `period_start`, `period_end`, `raw_text`, and `file_hash`

### Requirement: chunks table stores retrievable text fragments

The system SHALL define a `chunks` table with columns: `id uuid PRIMARY KEY`, `statement_id uuid REFERENCES statements(id) ON DELETE CASCADE`, `content text`, `metadata jsonb`, `chunk_index int`, `created_at timestamptz`.

#### Scenario: Insert chunks for a statement

- **WHEN** the ingestion pipeline chunks a statement and inserts chunks
- **THEN** each chunk row SHALL reference the statement and contain searchable `content`

### Requirement: statement_embeddings table stores pgvector vectors

The system SHALL define a `statement_embeddings` table with columns: `id uuid PRIMARY KEY`, `chunk_id uuid REFERENCES chunks(id) ON DELETE CASCADE`, `embedding vector(1024)`, `model text`, `created_at timestamptz`. The `embedding` column SHALL use the `pgvector` `vector` type with dimension 1024 (matching Voyage `voyage-3-large`).

#### Scenario: Insert embedding for a chunk

- **WHEN** the embeddings client returns a 1024-dimensional vector for a chunk
- **THEN** the repository SHALL insert the vector into `statement_embeddings.embedding`

### Requirement: Repository interfaces abstract persistence

The system SHALL define interfaces `StatementRepository`, `ChunkRepository`, and `EmbeddingRepository` in `internal/financas/repository/repository.go` with methods such as `CreateStatement(ctx, stmt) (*Statement, error)`, `GetStatementByHash(ctx, hash) (*Statement, error)`, `CreateChunks(ctx, statementID, chunks) error`, `SearchEmbeddings(ctx, queryVector, limit) ([]SearchResult, error)`. Postgres implementations SHALL live in `internal/financas/repository/postgres/`.

#### Scenario: Use case depends on interface

- **WHEN** the chat orchestrator queries transactions
- **THEN** it SHALL call `SearchEmbeddings` through the `EmbeddingRepository` interface, not directly through pgx

### Requirement: Context propagation to all database operations

The system SHALL pass `context.Context` as the first parameter to every repository method and use pgx `*Context` variants (`QueryRowContext`, `ExecContext`, `BeginTx`) so cancellation and timeouts propagate to the database.

#### Scenario: Cancelled context aborts query

- **WHEN** a repository method is called with a cancelled context
- **THEN** it SHALL return an error wrapping `context.Canceled` without executing the query

### Requirement: Typed domain errors for missing records

The system SHALL define sentinel errors `ErrStatementNotFound` and `ErrChunkNotFound` in `internal/financas/domain/errors.go`. The Postgres repository SHALL translate `pgx.ErrNoRows` into the appropriate domain error using `errors.Is`.

#### Scenario: Statement not found returns domain error

- **WHEN** `GetStatementByHash` finds no matching row
- **THEN** it SHALL return `ErrStatementNotFound` wrapped with context, not the raw pgx error
