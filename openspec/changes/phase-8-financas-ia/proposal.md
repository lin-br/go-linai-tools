## Why

Phase 8 is the capstone of the 18-week AI Engineer roadmap: a working personal-finance assistant called Finanças IA. Earlier phases built the pieces — provider abstraction, agent loop, RAG, document extraction, evals, LLMOps — but no product integrates them. This change creates that integration inside the existing `go-linai-tools` monorepo, producing a runnable backend + frontend + eval suite that can answer natural-language questions over uploaded credit-card PDFs.

## What Changes

- Add `cmd/financas-ia/` — Go HTTP backend (`chi` or `net/http` ServeMux) exposing `POST /chat`, `POST /ingest`, `GET /tasks/:id`, `GET /health`.
- Add `internal/financas/` packages for domain models, repository interfaces, Postgres repository, ingestion orchestration, chat orchestration, and evals.
- Wire `pgx` connection pool with schema migrations for `statements`, `chunks`, and `embeddings` tables backed by `pgvector`.
- Build ingestion pipeline: PDF upload → `internal/document/` extraction → chunking → Voyage embeddings → pgvector.
- Build chat backend: SSE streaming response, agent tool calls (`search_transactions`, `list_statements`, `ingest_statement`), optional cost badge metadata.
- Build frontend: static HTML/CSS/JS chat UI with drag-and-drop PDF upload, pending-actions indicator, confirmation card for high-stakes tool calls, cost badge, and SSE rendering.
- Add eval suite: 30 intent-based evals run with `go test ./internal/financas/evals/...`, plus table-driven unit tests for repository and orchestrator layers.
- Add deployment artifacts: multi-stage `Dockerfile`, `docker-compose.yml` (app + postgres + langfuse + redis), `/health` checks covering DB and Claude.
- Add `README.md` with architecture diagram, setup instructions, eval results, and link to 1-min Loom demo.

## Capabilities

### New Capabilities

- `mp34-backend-api`: HTTP backend entry point and REST API surface (`POST /chat`, `POST /ingest`, `GET /tasks/:id`, `GET /health`) with `signal.NotifyContext` graceful shutdown.
- `mp35-database-layer`: Postgres schema migrations and `pgx`-based repository layer for statements, chunks, and embeddings.
- `mp36-ingestion-pipeline`: PDF upload → document extraction → chunking → embeddings → pgvector storage, exposed via `POST /ingest` with async task tracking.
- `mp37-chat-sse`: Chat endpoint returning SSE streaming responses, agent tool calls for finance queries, and integration with the existing `Provider` / agent-loop packages.
- `mp38-frontend-ui`: Static chat UI with drag-and-drop upload, pending-actions indicator, confirmation card, and cost badge.
- `mp39-evals-deployment`: 30 intent evals via `go test`, multi-stage Dockerfile, `docker-compose.yml`, and `/health` checks for DB + Claude.
- `mp40-readme-demo`: README with architecture diagram, setup, eval results, and 1-min Loom demo.

### Modified Capabilities

(None. Phase 8 consumes prior packages `internal/core/domain`, `internal/core/ports/outbound`, `internal/agent/`, `internal/document/`, `internal/rag/`, `internal/evals/`, `internal/observability/` without changing their contracts.)

## Impact

- **New code**: `cmd/financas-ia/main.go`, `internal/financas/*`, `web/` frontend assets, `build/Dockerfile`, `deploy/docker-compose.yml`, `docs/financas-ia/README.md`.
- **Dependencies**: adds `github.com/jackc/pgx/v5`, `github.com/pgvector/pgvector-go`, `github.com/go-chi/chi/v5` (optional; `net/http` ServeMux is acceptable), `github.com/testcontainers/testcontainers-go` for integration tests (optional), existing Langfuse/Redis images in Docker Compose.
- **External services**: requires Voyage AI for embeddings, Cohere for reranking, Anthropic/OpenRouter for LLM calls, Postgres with `pgvector` extension, Langfuse self-hosted for traces, Redis for async task state.
- **No breaking changes**: purely additive to the monorepo; existing `cmd/cli` and `cmd/extract` continue to work unchanged.
