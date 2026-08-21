## ADDED Requirements

### Requirement: pgvector-backed chunk repository package exists under internal/rag/store

The system SHALL provide a package `internal/rag/store` that exports a `Store` struct managing chunks in a PostgreSQL database using the `pgvector` extension. The package SHALL use `pgx/v5` and `pgvector-go`. The schema SHALL contain columns `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`, `content TEXT NOT NULL`, `embedding vector(1024) NOT NULL`, `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`, and `source_path TEXT NOT NULL`.

#### Scenario: Store constructor accepts a pgx pool

- **WHEN** code calls `store.New(pool)` with a non-nil `*pgxpool.Pool`
- **THEN** it SHALL return a non-nil `*store.Store`

#### Scenario: Store requires non-nil pool

- **WHEN** code calls `store.New(nil)`
- **THEN** it SHALL return an error indicating that the pool is required

### Requirement: Schema initialization is explicit and idempotent

The system SHALL provide `Store.InitSchema(ctx context.Context) error` that creates the `chunks` table and the `vector(1024)` extension if they do not exist. The method SHALL be idempotent and SHALL return an error if the database command fails.

#### Scenario: Schema can be created on an empty database

- **WHEN** `store.InitSchema(ctx)` is called on a fresh Postgres database
- **THEN** it SHALL create the `chunks` table and the pgvector extension and return no error

#### Scenario: Schema is idempotent

- **WHEN** `store.InitSchema(ctx)` is called a second time on the same database
- **THEN** it SHALL return no error and SHALL NOT drop existing data

### Requirement: Store inserts chunked documents

The system SHALL provide `Store.InsertChunks(ctx context.Context, chunks []Chunk) ([]uuid.UUID, error)`. Each `Chunk` SHALL contain `Content string`, `Embedding []float32`, `Metadata map[string]any`, and `SourcePath string`. The method SHALL insert all chunks in a single transaction and return the generated `id` values in the same order as the input slice.

#### Scenario: Insert single chunk

- **WHEN** `store.InsertChunks(ctx, []Chunk{{Content: "hello", Embedding: vec, Metadata: map[string]any{"idx": 0}, SourcePath: "notes.txt"}})` is called
- **THEN** it SHALL insert one row and return one UUID

#### Scenario: Insert batch preserves order

- **WHEN** `store.InsertChunks(ctx, []Chunk{c1, c2, c3})` is called
- **THEN** the returned UUID slice SHALL have length 3 and the IDs SHALL correspond to `c1`, `c2`, and `c3` in that order

#### Scenario: Empty insert is a no-op

- **WHEN** `store.InsertChunks(ctx, nil)` is called
- **THEN** it SHALL return an empty UUID slice and no error without touching the database

### Requirement: Store retrieves nearest neighbors by vector similarity

The system SHALL provide `Store.Search(ctx context.Context, query []float32, k int) ([]SearchResult, error)`. The method SHALL perform an L2 distance (`embedding <-> $1`) query against the `chunks` table, order by ascending distance, and return the top `k` results. Each `SearchResult` SHALL contain `ID uuid.UUID`, `Content string`, `SourcePath string`, `Metadata map[string]any`, and `Distance float64`.

#### Scenario: Search returns top-k nearest chunks

- **WHEN** the table contains chunks with embeddings and `store.Search(ctx, queryVec, 3)` is called
- **THEN** it SHALL return up to 3 results ordered by closest distance first

#### Scenario: Search returns empty when table is empty

- **WHEN** `store.Search(ctx, queryVec, 5)` is called on an empty table
- **THEN** it SHALL return an empty slice and no error

#### Scenario: Search validates k is positive

- **WHEN** `store.Search(ctx, queryVec, 0)` is called
- **THEN** it SHALL return an error indicating `k` must be positive

### Requirement: DTOs serialize metadata cleanly

The system SHALL define `store.Chunk` and `store.SearchResult` structs with JSON tags. `Metadata` SHALL be `map[string]any` and stored as JSONB. Embedding fields in DTOs used for database binding SHALL be compatible with `pgvector.Vector`.

#### Scenario: Chunk round-trips metadata

- **WHEN** a chunk with `Metadata: map[string]any{"section": "intro"}` is inserted and then retrieved via search
- **THEN** the returned `SearchResult.Metadata["section"]` SHALL equal `"intro"`

### Requirement: Schema SQL is documented alongside code

The system SHALL include a file `internal/rag/store/schema.sql` containing the `CREATE EXTENSION IF NOT EXISTS vector;` and `CREATE TABLE IF NOT EXISTS chunks (...)` statements used by `InitSchema`. The file is for human review and copy-paste into migration tools.

#### Scenario: schema.sql matches InitSchema behavior

- **WHEN** `schema.sql` is compared to the SQL executed by `Store.InitSchema`
- **THEN** both SHALL create the same table and extension
