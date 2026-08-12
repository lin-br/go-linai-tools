## Context

Phase 1 established the project's provider abstraction (`outbound.Provider` with `Chat`/`ChatStream`), OpenRouter HTTP client, config loader, and three CLIs. Phase 2 builds a complete retrieval stack on top of that foundation so the project can answer questions grounded in personal PDFs instead of relying on the model's parametric knowledge.

The roadmap calls for a Go-first, `net/http`-first RAG pipeline: Voyage embeddings, pgvector storage, recursive + contextual chunking, hybrid search (BM25 + vector + RRF), Cohere reranking, and an eval suite with a `rag` CLI. All components live inside the existing monorepo under `internal/rag/` and `cmd/rag/`; there is no separate repo.

## Goals / Non-Goals

**Goals:**
- Provide a minimal but complete RAG pipeline in Go that ingests documents, chunks them, embeds them, stores them in pgvector, retrieves with hybrid search, reranks, and answers queries.
- Expose every building block as a small, testable package with explicit interfaces and constructors.
- Use `net/http` for Voyage and Cohere calls instead of their SDKs.
- Use `pgx` + `pgvector-go` for Postgres access and vector storage.
- Use `blevesearch` for in-memory BM25 indexing.
- Measure retrieval quality with a hand-built eval suite: `precision@k`, `recall@k`, `MRR`, and LLM-as-judge.
- Ship a `rag` CLI with `ingest`, `query`, and `eval` subcommands.
- Preserve existing conventions: `context.Context` first, return errors from library code, table-driven tests, no external CLI framework (`flag` only), no Python.

**Non-Goals:**
- Multi-tenant or production-scale ingestion (single-user, local Postgres).
- Online schema migrations; `schema.sql` is applied manually or via a one-shot init flag.
- Streaming response generation — `rag query` uses non-streaming `Provider.Chat`.
- Persistent BM25 index across restarts; the BM25 index is rebuilt per process from pgvector rows.
- Advanced eval frameworks (RAGAS, MLflow) — metrics are hand-rolled.
- Front-end chat UI — that is Phase 8.
- Embedding other providers besides Voyage in this phase.
- Document parsing (PDF → text) — MP13 accepts text files; PDF extraction is Phase 6.

## Decisions

### D1: Package layout — `internal/rag/` per concern

Each RAG concern gets its own package: `embeddings`, `store`, `chunk`, `search`, `rerank`, `eval`. The CLI in `cmd/rag/` wires them together.

**Why:** Matches the microphase plan exactly, keeps packages small, and makes each component independently testable. Using `internal/` signals the RAG packages are not public API.

**Alternative considered:** A single `pkg/rag/` package. Rejected — it would mix HTTP clients, SQL, chunking, and eval logic in one place and make parallel work harder.

### D2: Voyage and Cohere are direct HTTP clients, not `Provider` implementations

`embeddings.Client` and `rerank.Client` use `net/http` directly and talk to Voyage and Cohere endpoints. They do not implement `outbound.Provider`.

**Why:** `outbound.Provider` models chat completion (`ChatRequest`/`ChatResponse`). Embeddings and reranking have different request/response shapes, different auth, and different base URLs. Forcing them into the chat interface would create an awkward abstraction. The `rag` CLI will hold three clients side by side: `OpenRouterProvider` (chat), `embeddings.Client` (embed), `rerank.Client` (rerank).

**Alternative considered:** Extend `Provider` with `Embed` and `Rerank` methods. Rejected — bloats the interface and couples Phase 1/3 code to RAG-specific concerns.

### D3: `pgx` + `pgvector-go` for Postgres

`store.Store` receives a `*pgxpool.Pool` and uses `pgvector-go`'s `pgvector.Vector` type to pass `[]float32` values to Postgres. Vectors are normalized by Voyage, so L2 distance (`<->`) is used for vector search.

**Why:** `pgx` is the idiomatic, high-performance PostgreSQL driver for Go. `pgvector-go` gives a ready-made scan/arg type. Using the vector extension directly avoids running a separate vector database.

**Alternative considered:** `database/sql` with `lib/pq`. Rejected — `pgx` is faster, has first-class `COPY`, better type support, and is the driver used in the Phase 8 design.

### D4: BM25 via `blevesearch` in-memory index

`search.BM25Searcher` builds an in-memory Bleve index from a slice of chunks at query time and runs keyword search.

**Why:** Bleve is pure Go, has no extra runtime dependency, and gives BM25 scoring out of the box. Because Phase 2 targets personal PDFs (hundreds to thousands of chunks), rebuilding the index per query is acceptable. It also keeps the architecture simple: no second persistent store.

**Alternative considered:** Postgres full-text search (`tsvector`). Rejected — it requires schema changes and does not map cleanly onto the "hybrid search" learning goal; Bleve demonstrates BM25 explicitly.

### D5: RRF merges ranked lists, not scores

`search.Hybrid` takes the top-`k` results from BM25 and vector search, assigns reciprocal ranks `1/(rank + k_rrf)` with `k_rrf = 60`, and returns the union sorted by fused score.

**Why:** RRF is robust to incompatible score distributions across BM25 (TF-IDF-like) and vector (cosine/L2) retrievers. It needs only rank positions, which both retrievers already produce.

**Alternative considered:** Weighted score sum. Rejected — scores are not comparable across modalities; normalizing them would introduce magic constants.

### D6: Contextual chunking uses the chat provider, not the embeddings model

Before chunking, `chunk.ContextualChunker` sends the full document (truncated to fit) to `outbound.Provider.Chat` with a one-sentence-summary instruction, then prepends that summary to every chunk produced by the underlying `RecursiveChunker`.

**Why:** The Anthropic 2024 contextual chunking technique improves retrieval by giving each chunk document-level context. It needs an LLM, not an embedding model, so it reuses the existing `Provider` abstraction.

**Alternative considered:** Use a cheap Voyage summary endpoint. Rejected — Voyage does not provide summarization; using the chat provider is the intended pattern.

### D7: `rag query` builds context from top-N reranked chunks

`cmd/rag query` retrieves `top_k * 4` candidates from hybrid search, reranks them with Cohere, takes `top_k`, concatenates chunk contents, and sends a single non-streaming chat request to `OpenRouterProvider`.

**Why:** Cohere reranking is most effective on a larger candidate set; `top_k * 4` gives the reranker room to surface the best chunks. A single chat call keeps the CLI simple and matches Phase 1's non-streaming `Chat` path.

**Alternative considered:** Stream the final answer. Rejected — streaming adds complexity without improving retrieval learning; Phase 1 already covers streaming.

### D8: Eval suite uses `jsonl` and deterministic metrics plus LLM judge

`eval.Dataset` loads a `.jsonl` file where each line is `{query, expected_chunk_id, expected_answer?}`. `eval.Metrics` computes `precision@k`, `recall@k`, and MRR. `eval.Judge` uses `outbound.Provider.Chat` with a rubric to score answer relevance 1–5.

**Why:** Deterministic retrieval metrics are cheap and objective. LLM-as-judge adds a semantic quality signal for generated answers. A plain `jsonl` file keeps the dataset editable and version-controllable.

**Alternative considered:** Integrate a third-party eval framework. Rejected — the roadmap explicitly says "no framework" for Phase 2 evals; building the metrics teaches the math.

### D9: No external CLI framework — `flag` package only

`cmd/rag` uses the standard `flag` package and manual subcommand dispatch (`os.Args[1]`). Global flags like `-db`, `-top-k`, and `-model` are parsed per subcommand.

**Why:** Matches the existing CLIs (`cmd/extract`, `cmd/summarize`) and the roadmap constraint to avoid heavy frameworks. Three subcommands do not justify `cobra`.

### D10: Connection strings and API keys from config/env

Runtime config comes from `internal/configs/configs.yaml` plus env vars: `VOYAGE_API_KEY`, `COHERE_API_KEY`, `POSTGRES_DSN`, and existing `OPENROUTER_API_KEY`. The CLI `log.Fatal`s only at startup if required keys are missing.

**Why:** Consistent with Phase 1 config strategy (`os.ExpandEnv`). Library code returns errors; `main` decides whether to fatal.

## Risks / Trade-offs

- **[Bleve index rebuilt per query]** → Query latency grows linearly with corpus size. Mitigation: Phase 2 corpus is small (personal PDFs). If latency becomes a problem, persist the Bleve index to disk or switch to an indexed search in Phase 8.
- **[Contextual chunking sends the full document to the LLM]** → Expensive and may hit context limits. Mitigation: truncate documents to a safe token budget in the summary prompt; contextual chunking is optional per chunk strategy flag.
- **[No persistent BM25 index]** → Restarting the CLI loses keyword statistics. Mitigation: acceptable for Phase 2; corpus is reloaded from Postgres each run.
- **[Voyage/Cohere direct HTTP clients must track upstream API drift]** → Request/response structs are hand-maintained. Mitigation: keep structs minimal and add version-specific documentation links in code comments.
- **[Vector dimension hard-coded to 1024]** → Matches `voyage-3-large`. Mitigation: document the coupling; if a different model is used, update `schema.sql` and `Client.Embed` together.
- **[No PDF parsing in this phase]** → Users must pass text files to `rag ingest`. Mitigation: explicitly documented; Phase 6 adds PDF extraction.
- **[pgvector requires local Postgres]** → Users must run Postgres locally. Mitigation: provide a `docker-compose.yml` snippet in `cmd/rag/README.md` (not a full compose file yet).

## Migration Plan

No migration needed — this is a new subsystem. Implementation order follows microphase dependencies:

1. MP7 embeddings client (no deps).
2. MP8 vector store (depends on MP7 for vector dimension knowledge only; can be built in parallel once schema is fixed).
3. MP9 chunking (no deps).
4. MP10 hybrid search (needs store + chunking).
5. MP11 reranking (needs hybrid search candidate set).
6. MP12 eval suite (needs reranking + metrics).
7. MP13 CLI (wires all above).

Rollback: delete `internal/rag/`, `cmd/rag/`, and revert `go.mod`/`configs.yaml`.

## Open Questions

- Should `rag ingest` accept directories recursively, or only single files? (Spec assumes single file first; directory support is a follow-up.)
- Should the eval dataset live in `cmd/rag/testdata/` or in a top-level `rag-datasets/` directory? (Spec assumes `cmd/rag/testdata/golden.jsonl`.)
- Should hybrid search also include a naive keyword baseline for comparison? (Out of scope for MP10; could be added in MP12.)
