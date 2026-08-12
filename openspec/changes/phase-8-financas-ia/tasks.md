## 1. Project Structure and Bootstrap

- [ ] 1.1 Create `cmd/financas-ia/main.go` with config loading, `pgxpool` setup, migration runner, HTTP server wiring, and `signal.NotifyContext` graceful shutdown.
- [ ] 1.2 Create `internal/financas/domain/` package with `Statement`, `Transaction`, `Chunk`, `Embedding`, `Task` structs and sentinel errors (`ErrStatementNotFound`, `ErrTaskNotFound`).
- [ ] 1.3 Create `internal/financas/repository/repository.go` with `StatementRepository`, `ChunkRepository`, and `EmbeddingRepository` interfaces.
- [ ] 1.4 Create `internal/financas/config/config.go` to load `PORT`, `DATABASE_URL`, `REDIS_URL`, provider API keys, and embedding/rerank model names from environment variables.
- [ ] 1.5 Add required dependencies to `go.mod`: `github.com/jackc/pgx/v5`, `github.com/pgvector/pgvector-go`, `github.com/golang-migrate/migrate/v4` (optional), and update `go.sum`.

## 2. Database Layer (MP35)

- [ ] 2.1 Create Postgres migration files in `internal/financas/repository/postgres/migrations/` for `pgvector` extension and `statements`, `chunks`, `statement_embeddings` tables.
- [ ] 2.2 Implement `internal/financas/repository/postgres/postgres.go` with `NewPool` and `ApplyMigrations` functions.
- [ ] 2.3 Implement `PostgresStatementRepository` with `CreateStatement`, `GetStatementByHash`, `ListStatements`, and context-aware pgx queries.
- [ ] 2.4 Implement `PostgresChunkRepository` with `CreateChunks`, `GetChunksByStatementID`, and transactional insert.
- [ ] 2.5 Implement `PostgresEmbeddingRepository` with `CreateEmbeddings` and `SearchEmbeddings` using `pgvector` cosine similarity.
- [ ] 2.6 Add table-driven unit tests for repository methods using `pgx` fakes or testcontainers (`internal/financas/repository/postgres/*_test.go`).

## 3. Backend API (MP34)

- [ ] 3.1 Implement `internal/financas/server/server.go` with `net/http` `ServeMux`, route registration, and structured logging middleware.
- [ ] 3.2 Implement `POST /chat` handler in `internal/financas/server/chat_handler.go` that validates JSON, opens an SSE stream, and delegates to the chat orchestrator.
- [ ] 3.3 Implement `POST /ingest` handler in `internal/financas/server/ingest_handler.go` with PDF MIME/size validation, file hash computation, async task creation, and `202 Accepted` response.
- [ ] 3.4 Implement `GET /tasks/{id}` handler in `internal/financas/server/task_handler.go` that reads task state from Redis or in-memory fallback and returns status/result/error.
- [ ] 3.5 Implement `GET /health` handler that pings Postgres and performs a lightweight LLM provider check, returning `200` or `503` with per-check status.
- [ ] 3.6 Add unit tests for HTTP handlers using `httptest` and fake repositories (`internal/financas/server/*_test.go`).

## 4. Async Task Worker and State

- [ ] 4.1 Implement `internal/financas/tasks/store.go` interface with `Create`, `Update`, `Get` methods and Redis-backed implementation.
- [ ] 4.2 Add in-memory fallback store for local development when `REDIS_URL` is unset.
- [ ] 4.3 Implement `internal/financas/worker/worker.go` that reads pending tasks, runs ingestion pipeline, updates task status, and recovers from panics.
- [ ] 4.4 Start worker goroutine(s) in `cmd/financas-ia/main.go` with context-aware shutdown.

## 5. Ingestion Pipeline (MP36)

- [ ] 5.1 Implement `internal/financas/ingest/service.go` orchestrating validation → dedup → extraction → chunking → embeddings → persistence.
- [ ] 5.2 Wire `internal/document.ExtractStatement` (MP29–MP32) to extract `Statement` from uploaded PDF bytes.
- [ ] 5.3 Wire `internal/rag/chunk` (MP9) to split statement text and transactions into contextual chunks.
- [ ] 5.4 Wire `internal/rag/embeddings` (MP7) to generate 1024-dim vectors for chunks in batches.
- [ ] 5.5 Persist chunks and embeddings inside a single Postgres transaction via repository methods.
- [ ] 5.6 Implement file-hash deduplication before extraction.
- [ ] 5.7 Add unit tests for the ingestion service with fake document extractor and fake embedding client.

## 6. Chat Orchestrator and SSE (MP37)

- [ ] 6.1 Implement `internal/financas/chat/orchestrator.go` with `Run(ctx, query, conversationID, stream)` that wraps `internal/agent/Loop`.
- [ ] 6.2 Define finance tools: `search_transactions`, `list_statements`, `get_spending_summary` with `Tool` schemas matching `internal/core/domain` types.
- [ ] 6.3 Implement tool handlers that query `EmbeddingRepository.SearchEmbeddings` and `StatementRepository`.
- [ ] 6.4 Emit SSE event types `text`, `tool_call`, `source`, `cost`, `confirmation`, `done`, `error` with JSON envelopes.
- [ ] 6.5 Implement cost computation from provider `Usage` and static pricing, emitting `cost` events.
- [ ] 6.6 Implement confirmation gate: pause loop on destructive tools, emit `confirmation` event, resume on follow-up request.
- [ ] 6.7 Add tests for orchestrator event order and tool selection using fake provider and fakes.

## 7. Frontend UI (MP38)

- [ ] 7.1 Create `web/financas-ia/index.html` with chat panel, message input, drop zone, pending indicator, confirmation card placeholder, and cost badge slot.
- [ ] 7.2 Create `web/financas-ia/styles.css` with responsive layout, message bubbles, and progress indicator styles.
- [ ] 7.3 Create `web/financas-ia/app.js` that opens `EventSource` to `POST /chat`, renders streaming text, handles drag-and-drop upload, polls task status, and renders tool-call/cost/confirmation events.
- [ ] 7.4 Embed `web/financas-ia/` into the Go binary with `//go:embed` and serve via `net/http`.
- [ ] 7.5 Add a simple smoke test that fetches `/` and `/static/app.js` using `httptest`.

## 8. Evals Suite (MP39)

- [ ] 8.1 Create `internal/financas/evals/intent_eval_test.go` with 30 table-driven intent cases covering spending queries, statement listing, totals, categories, months, and merchants.
- [ ] 8.2 Implement fake provider and fake repositories seeded with test statements so evals run without external services.
- [ ] 8.3 Create `internal/financas/evals/behavioral_eval_test.go` gated by `//go:build eval` with LLM-as-judge rubric.
- [ ] 8.4 Run `go test ./internal/financas/...` and fix failing evals until stable.

## 9. Deployment Artifacts (MP39)

- [ ] 9.1 Create multi-stage `Dockerfile` using `golang:1.26.4-alpine` build stage and `scratch` final stage; embed web assets.
- [ ] 9.2 Create `.dockerignore` excluding `.git`, `vendor`, and local test artifacts.
- [ ] 9.3 Create `docker-compose.yml` with `postgres` (`pgvector/pgvector:pg16`), `redis`, `langfuse`, and `app` services with health checks.
- [ ] 9.4 Create `.env.example` listing all required and optional environment variables.
- [ ] 9.5 Verify `docker compose up -d` brings all services healthy and `GET /health` returns `200`.

## 10. README and Demo (MP40)

- [ ] 10.1 Create `docs/financas-ia/README.md` (or update root `README.md`) with architecture diagram, setup, usage examples, eval results, tech stack, and decisions.
- [ ] 10.2 Record a 1-minute Loom demo showing upload, chat, SSE, pending actions, confirmation card, and cost badge.
- [ ] 10.3 Add Loom link and Phase 8 LinkedIn post link to README.
- [ ] 10.4 Update `docs/roadmap-ai-engineer-status.md` to mark Phase 8 COMPLETE and record the LinkedIn post date.

## 11. Verification and Cleanup

- [ ] 11.1 Run `go build ./...` and `go test ./...` with no failures.
- [ ] 11.2 Run `go vet ./...` and `gofmt -s -w .` across new files.
- [ ] 11.3 Run `go test -race ./internal/financas/...`.
- [ ] 11.4 Run `openspec validate --change "phase-8-financas-ia"`.
- [ ] 11.5 Publish LinkedIn post #8 within 3 days of completion.
