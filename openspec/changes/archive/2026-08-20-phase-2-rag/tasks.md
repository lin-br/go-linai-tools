## 1. Dependencies & Configuration

- [x] 1.1 Run `go get github.com/jackc/pgx/v5 github.com/pgvector/pgvector-go github.com/blevesearch/bleve/v2 github.com/cenkalti/backoff/v4 github.com/google/uuid`
- [x] 1.2 Run `go mod tidy`
- [x] 1.3 Add Voyage, Cohere, Postgres, and RAG model keys to `internal/configs/configs.yaml` with sensible defaults and env expansion
- [x] 1.4 Extend `internal/configs` Go structs to expose `VoyageAPIKey`, `CohereAPIKey`, `PostgresDSN`, `DefaultEmbeddingModel`, `DefaultRerankModel`
- [x] 1.5 Verify `go build ./...` passes with new dependencies

## 2. MP7 — Embeddings Client (`internal/rag/embeddings/`)

- [x] 2.1 Create `internal/rag/embeddings/wire.go` with `voyageRequest`, `voyageResponse`, `voyageEmbedding`, and usage structs
- [x] 2.2 Create `internal/rag/embeddings/voyage.go` with `const DefaultModel = "voyage-3-large"`, `Client` struct, `NewClient`, and `Embed` method
- [x] 2.3 Implement L2 normalization helper used before returning vectors
- [x] 2.4 Add `internal/rag/embeddings/voyage_test.go` with table-driven tests for batching, order preservation, normalization, and missing API key
- [x] 2.5 Run `go test ./internal/rag/embeddings/...` and fix failures

## 3. MP8 — Vector Store (`internal/rag/store/`)

- [x] 3.1 Create `internal/rag/store/schema.sql` with `CREATE EXTENSION IF NOT EXISTS vector;` and `CREATE TABLE IF NOT EXISTS chunks (...)`
- [x] 3.2 Create `internal/rag/store/store.go` with `Chunk`, `SearchResult`, `Store` structs, `NewStore`, and `InitSchema`
- [x] 3.3 Implement `Store.InsertChunks(ctx, chunks)` using a single transaction and `pgx.CopyFrom` or parameterized inserts
- [x] 3.4 Implement `Store.Search(ctx, queryVec, k)` using `embedding <-> $1` ordering
- [x] 3.5 Add `internal/rag/store/store_test.go` with integration tests using `testcontainers` or `dockertest` if available, otherwise unit tests with a fake `Querier` interface
- [x] 3.6 Add `internal/rag/store/fake_test.go` with an in-memory fake implementing a `Store`-like interface for downstream tests
- [x] 3.7 Run `go test ./internal/rag/store/...`

## 4. MP9 — Chunking (`internal/rag/chunk/`)

- [x] 4.1 Create `internal/rag/chunk/chunk.go` with `Chunk`, `Document`, and `Chunker` interface
- [x] 4.2 Export `const DefaultChunkSize = 512` and `const DefaultChunkOverlap = 50`
- [x] 4.3 Create `internal/rag/chunk/recursive.go` implementing `RecursiveChunker` with separator cascade `\n\n`, `\n`, `. `, ` `, `""`
- [x] 4.4 Ensure `RecursiveChunker.Split` measures chunk length in runes via `utf8.RuneCountInString`
- [x] 4.5 Create `internal/rag/chunk/contextual.go` implementing `ContextualChunker` that wraps a `Chunker` and uses `outbound.Provider.Chat` for one-sentence summaries
- [x] 4.6 Add `internal/rag/chunk/chunk_test.go` with table-driven tests for short docs, paragraph splits, overlap, metadata, and context prepending
- [x] 4.7 Run `go test ./internal/rag/chunk/...`

## 5. MP10 — Hybrid Search (`internal/rag/search/`)

- [x] 5.1 Create `internal/rag/search/result.go` with `Result` struct
- [x] 5.2 Create `internal/rag/search/vector.go` with `VectorSearcher` delegating to `store.Store`
- [x] 5.3 Create `internal/rag/search/bm25.go` with `BM25Searcher` using `blevesearch/bleve/v2` default mapping
- [x] 5.4 Implement `BM25Searcher.Index(ctx, chunks)` building an in-memory Bleve index
- [x] 5.5 Implement `BM25Searcher.Search(ctx, query, k)` returning `[]Result`
- [x] 5.6 Create `internal/rag/search/hybrid.go` with `HybridSearcher` implementing RRF merge of vector and BM25 ranks
- [x] 5.7 Set default `kRRF = 60` and allow override via constructor
- [x] 5.8 Add `internal/rag/search/search_test.go` with tests for vector conversion, BM25 ranking, RRF fusion, and tie-breaking
- [x] 5.9 Run `go test ./internal/rag/search/...`

## 6. MP11 — Reranking (`internal/rag/rerank/`)

- [x] 6.1 Create `internal/rag/rerank/wire.go` with Cohere request/response types
- [x] 6.2 Create `internal/rag/rerank/cohere.go` with `const DefaultModel = "rerank-v3.5"`, `Client`, `Candidate`, `RankedResult`, `NewClient`, and `Rerank`
- [x] 6.3 Implement index validation and mapping from response indexes to candidate IDs
- [x] 6.4 Add `internal/rag/rerank/cohere_test.go` with tests for ranking order, `topN` truncation, empty candidates, and out-of-range index handling
- [x] 6.5 Run `go test ./internal/rag/rerank/...`

## 7. MP12 — RAG Eval Suite (`internal/rag/eval/`)

- [x] 7.1 Create `internal/rag/eval/dataset.go` with `Example`, `Dataset`, and `LoadDataset`
- [x] 7.2 Create `internal/rag/eval/metrics.go` with `PrecisionAtK`, `RecallAtK`, and `MRR`
- [x] 7.3 Create `internal/rag/eval/judge.go` with `Judge`, `NewJudge`, and `Score` using `outbound.Provider.Chat`
- [x] 7.4 Create `internal/rag/eval/evaluator.go` with `Evaluator`, `Report`, and `Run`
- [x] 7.5 Add `internal/rag/eval/eval_test.go` with tests for metrics edge cases, judge score parsing/clamping, and evaluator averaging
- [x] 7.6 Create `../../../../tests/evals/golden.jsonl` with at least 20 examples covering the Phase 2 target corpus
- [x] 7.7 Run `go test ./internal/rag/eval/...`

## 8. MP13 — RAG CLI (`cmd/rag/`)

- [x] 8.1 Create `cmd/rag/main.go` with manual subcommand dispatch, config loading, and signal context
- [x] 8.2 Create `cmd/rag/ingest.go` implementing the `ingest` subcommand with flags `-db`, `-chunk-size`, `-chunk-overlap`, `-contextual`, `-model`
- [x] 8.3 Create `cmd/rag/query.go` implementing the `query` subcommand with flags `-db`, `-top-k`, `-model`, `-rerank`, `-contextual`
- [x] 8.4 Create `cmd/rag/eval.go` implementing the `eval` subcommand with flags `-db`, `-dataset`, `-top-k`, `-judge`, `-model`
- [x] 8.5 Wire `embeddings.Client`, `store.Store`, `chunk` implementations, `search` implementations, `rerank.Client`, and `outbound.Provider` in each subcommand
- [x] 8.6 Ensure library errors are returned to `main` and mapped to `stderr` + exit code `1`; library code never calls `log.Fatal`
- [x] 8.7 Add `cmd/rag/README.md` with setup, Postgres via Docker, and usage examples

## 9. Verification

- [x] 9.1 Run `go build ./...` — all packages compile
- [x] 9.2 Run `go vet ./...` — no warnings
- [x] 9.3 Run `go test ./...` — all tests pass
- [x] 9.4 Run `go run ./cmd/rag ingest testdata/sample.txt` against a local Postgres with pgvector
- [x] 9.5 Run `go run ./cmd/rag query "sample question"` and verify a grounded answer
- [x] 9.6 Run `go run ./cmd/rag eval -dataset testdata/golden.jsonl -judge` and verify metrics output
- [x] 9.7 Update `docs/roadmap-ai-engineer-status.md` to mark Phase 2 as COMPLETE and Phase 1 dependencies as satisfied
- [x] 9.8 Run `openspec validate --change "phase-2-rag"` and fix any validation errors
