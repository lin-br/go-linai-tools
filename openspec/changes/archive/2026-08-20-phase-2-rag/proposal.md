## Why

Phase 1 gave the project a working LLM client and structured-output plumbing; Phase 2 must turn raw text into retrievable knowledge. A RAG pipeline over personal PDFs is the core learning goal for weeks 4–6, and it is the foundation for the Finanças IA showcase (Phase 8). Without embeddings, chunking, hybrid search, reranking, and a measured eval suite, later phases have no retrieval layer to build on.

## What Changes

- Add `internal/rag/embeddings/` — direct HTTP client for Voyage AI embeddings (`voyage-3-large` default), producing `[]float32` vectors with batching.
- Add `internal/rag/store/` — `pgx` + `pgvector-go` schema and repository for chunks (`id, content, embedding vector(1024), metadata jsonb, source_path`), with insert and vector search.
- Add `internal/rag/chunk/` — recursive-character splitter plus contextual chunking (Anthropic 2024 pattern: prepend a one-sentence document summary to each chunk).
- Add `internal/rag/search/` — hybrid retriever that runs BM25 via `blevesearch` and vector search via `store`, then merges results with Reciprocal Rank Fusion (RRF).
- Add `internal/rag/rerank/` — direct HTTP client for the Cohere Rerank API, reordering candidate chunks by query relevance.
- Add `internal/rag/eval/` — golden dataset loader and retrieval metrics (`precision@k`, `recall@k`, `MRR`) plus an LLM-as-judge scorer built on `outbound.Provider`.
- Add `cmd/rag/` — CLI with subcommands `ingest <file>`, `query "..."`, and `eval`, wiring the packages above.
- Update `go.mod` with `github.com/jackc/pgx/v5`, `github.com/pgvector/pgvector-go`, `github.com/blevesearch/bleve/v2`, and `github.com/cenkalti/backoff/v4`.
- Update `internal/configs/configs.yaml` with new optional keys: `voyage_api_key`, `cohere_api_key`, `postgres_dsn`, `default_embedding_model`, `default_rerank_model`.

## Capabilities

### New Capabilities

- `mp7-embeddings-client`: Direct HTTP Voyage embeddings client with batching, `net/http`, no SDK.
- `mp8-vector-store`: pgvector-backed chunk repository using `pgx` + `pgvector-go`.
- `mp9-chunking`: Recursive-character and contextual chunking for raw documents.
- `mp10-hybrid-search`: BM25 + vector search merged via RRF.
- `mp11-reranking`: Cohere Rerank API wrapper for result reordering.
- `mp12-rag-eval`: Retrieval eval suite with deterministic metrics and LLM-as-judge.
- `mp13-rag-cli`: `rag` CLI entry point with `ingest`, `query`, and `eval` subcommands.

### Modified Capabilities

(No existing specs are modified. Phase 2 adds new packages and a new CLI; it consumes the existing `outbound.Provider` interface and `domain.ChatRequest` types without changing them.)

## Impact

- **New files/directories**:
  - `internal/rag/embeddings/` — Voyage client (`voyage.go`) and wire types.
  - `internal/rag/store/` — Postgres repository (`store.go`), schema SQL (`schema.sql`), and DTOs.
  - `internal/rag/chunk/` — chunker interface and implementations (`recursive.go`, `contextual.go`).
  - `internal/rag/search/` — hybrid search (`hybrid.go`), BM25 index (`bm25.go`), RRF merge (`rrf.go`).
  - `internal/rag/rerank/` — Cohere client (`cohere.go`) and wire types.
  - `internal/rag/eval/` — metrics (`metrics.go`), golden dataset loader (`dataset.go`), LLM judge (`judge.go`).
  - `cmd/rag/` — `main.go`, `ingest.go`, `query.go`, `eval.go`, and shared wiring.
- **Updated files**:
  - `go.mod` / `go.sum` — new dependencies.
  - `internal/configs/configs.yaml` — new API keys and connection strings.
- **No breaking changes** — purely additive. Existing CLIs (`cmd/cli`, `cmd/summarize`, `cmd/extract`) are untouched.
- **Enables** Phase 2 completion and provides the retrieval layer reused by Phase 5 (LLMOps) and Phase 8 (Finanças IA).
