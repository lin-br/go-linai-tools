package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	pgxvec "github.com/pgvector/pgvector-go/pgx"
)

// schemaSQL mirrors internal/rag/store/schema.sql and is executed by
// InitSchema. Keeping the string inline lets InitSchema be self-contained
// while schema.sql stays available for human review and migration tooling.
const schemaSQL = `CREATE EXTENSION IF NOT EXISTS vector;
CREATE TABLE IF NOT EXISTS chunks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    content     TEXT NOT NULL,
    embedding   vector(2048) NOT NULL,
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_path TEXT NOT NULL
);`

const insertSQL = `INSERT INTO chunks (id, content, embedding, metadata, source_path)
VALUES ($1, $2, $3, $4::jsonb, $5)`

const searchSQL = `SELECT id, content, source_path, metadata, embedding <-> $1 AS distance
FROM chunks
ORDER BY embedding <-> $1
LIMIT $2`

const listSQL = `SELECT id, content, source_path, metadata FROM chunks ORDER BY id`

// Chunk is the DTO for a document chunk persisted in the chunks table. The
// Embedding field is compatible with pgvector.Vector for database binding.
type Chunk struct {
	ID         uuid.UUID      `json:"id"`
	Content    string         `json:"content"`
	Embedding  []float32      `json:"embedding"`
	Metadata   map[string]any `json:"metadata"`
	SourcePath string         `json:"source_path"`
}

// SearchResult is a single nearest-neighbor hit returned by Store.Search.
type SearchResult struct {
	ID         uuid.UUID
	Content    string
	SourcePath string
	Metadata   map[string]any
	Distance   float64
}

// Execer runs a single parameterized statement within a transaction. It is the
// minimal capability InsertChunks needs from the active transaction.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) error
}

// Querier abstracts the database operations the Store depends on. The real
// implementation wraps a *pgxpool.Pool; a fake implements the same interface
// for unit tests without a live Postgres (task 3.5/3.6).
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) error
	WithTx(ctx context.Context, fn func(Execer) error) error
	QueryChunks(ctx context.Context, queryVec []float32, k int) ([]SearchResult, error)
	ListChunks(ctx context.Context) ([]Chunk, error)
}

// Store manages chunks in a PostgreSQL database with the pgvector extension.
type Store struct {
	q Querier
}

// NewStore wraps a *pgxpool.Pool. The pool must be non-nil; callers should
// construct it via NewPool so pgvector types are registered on each connection.
func NewStore(pool *pgxpool.Pool) (*Store, error) {
	if pool == nil {
		return nil, errors.New("store: pgx pool is required")
	}
	return &Store{q: &pgxQuerier{pool: pool}}, nil
}

// NewStoreFromQuerier builds a Store from a custom Querier. Intended for tests
// that inject a fake; production code uses NewStore.
func NewStoreFromQuerier(q Querier) *Store {
	return &Store{q: q}
}

// NewPool constructs a pgxpool.Pool whose connections have pgvector types
// registered via AfterConnect. dsn must be a valid Postgres connection string.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("store: parse dsn: %w", err)
	}
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		if _, err := conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector"); err != nil {
			return fmt.Errorf("store: create vector extension: %w", err)
		}
		return pgxvec.RegisterTypes(ctx, conn)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}
	return pool, nil
}

// InitSchema creates the vector extension and chunks table idempotently.
func (s *Store) InitSchema(ctx context.Context) error {
	return s.q.Exec(ctx, schemaSQL)
}

// InsertChunks inserts all chunks in a single transaction, pre-generating UUIDs
// client-side so the returned IDs match the input order. An empty input is a
// no-op that does not touch the database.
func (s *Store) InsertChunks(ctx context.Context, chunks []Chunk) ([]uuid.UUID, error) {
	if len(chunks) == 0 {
		return nil, nil
	}
	ids := make([]uuid.UUID, len(chunks))
	for i := range chunks {
		ids[i] = uuid.New()
	}
	err := s.q.WithTx(ctx, func(tx Execer) error {
		for i, c := range chunks {
			metaJSON, merr := json.Marshal(c.Metadata)
			if merr != nil {
				return fmt.Errorf("store: marshal metadata for chunk %d: %w", i, merr)
			}
			if err := tx.Exec(ctx, insertSQL, ids[i], c.Content, pgvector.NewVector(c.Embedding), string(metaJSON), c.SourcePath); err != nil {
				return fmt.Errorf("store: insert chunk %d: %w", i, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// Search returns the k nearest chunks to queryVec by L2 distance
// (embedding <-> $1), ordered by ascending distance.
func (s *Store) Search(ctx context.Context, queryVec []float32, k int) ([]SearchResult, error) {
	if k <= 0 {
		return nil, errors.New("store: k must be positive")
	}
	return s.q.QueryChunks(ctx, queryVec, k)
}

// ListChunks returns all stored chunks ordered by id, without their embeddings.
// Used by the CLI to rebuild the BM25 keyword index each run.
func (s *Store) ListChunks(ctx context.Context) ([]Chunk, error) {
	return s.q.ListChunks(ctx)
}

// pgxQuerier adapts a *pgxpool.Pool to the Querier interface.
type pgxQuerier struct {
	pool *pgxpool.Pool
}

func (q *pgxQuerier) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := q.pool.Exec(ctx, sql, args...)
	return err
}

func (q *pgxQuerier) WithTx(ctx context.Context, fn func(Execer) error) error {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	if err := fn(&pgxTxExecer{tx: tx}); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit tx: %w", err)
	}
	return nil
}

func (q *pgxQuerier) QueryChunks(ctx context.Context, queryVec []float32, k int) ([]SearchResult, error) {
	rows, err := q.pool.Query(ctx, searchSQL, pgvector.NewVector(queryVec), k)
	if err != nil {
		return nil, fmt.Errorf("store: query: %w", err)
	}
	defer rows.Close()

	results := make([]SearchResult, 0, k)
	for rows.Next() {
		var sr SearchResult
		var metaBytes []byte
		if err := rows.Scan(&sr.ID, &sr.Content, &sr.SourcePath, &metaBytes, &sr.Distance); err != nil {
			return nil, fmt.Errorf("store: scan row: %w", err)
		}
		if len(metaBytes) > 0 {
			if err := json.Unmarshal(metaBytes, &sr.Metadata); err != nil {
				return nil, fmt.Errorf("store: unmarshal metadata: %w", err)
			}
		}
		results = append(results, sr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: rows: %w", err)
	}
	return results, nil
}

func (q *pgxQuerier) ListChunks(ctx context.Context) ([]Chunk, error) {
	rows, err := q.pool.Query(ctx, listSQL)
	if err != nil {
		return nil, fmt.Errorf("store: list: %w", err)
	}
	defer rows.Close()

	out := make([]Chunk, 0)
	for rows.Next() {
		var c Chunk
		var metaBytes []byte
		if err := rows.Scan(&c.ID, &c.Content, &c.SourcePath, &metaBytes); err != nil {
			return nil, fmt.Errorf("store: scan row: %w", err)
		}
		if len(metaBytes) > 0 {
			if err := json.Unmarshal(metaBytes, &c.Metadata); err != nil {
				return nil, fmt.Errorf("store: unmarshal metadata: %w", err)
			}
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: rows: %w", err)
	}
	return out, nil
}

// pgxTxExecer adapts a pgx.Tx to the Execer interface.
type pgxTxExecer struct {
	tx pgx.Tx
}

func (e *pgxTxExecer) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := e.tx.Exec(ctx, sql, args...)
	return err
}
