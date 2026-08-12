## Context

Phases 1–7 of the roadmap built reusable capabilities inside the `go-linai-tools` monorepo:

- Phase 1: `outbound.Provider` interface and OpenRouter implementation, `RetryProvider`, `tools.Extract[T]`, structured output via forced tool choice.
- Phase 2: `internal/rag/` — Voyage embeddings, pgvector store, recursive/contextual chunking, hybrid search, Cohere reranking.
- Phase 3: `internal/agent/` — `Loop.Run(ctx, query)` with max turns, retry/backoff, streaming channel, agent patterns (ReAct, Plan-and-Execute, Reflection), and the MCP server skeleton.
- Phase 5: `internal/evals/` (unit + behavioral evals, golden dataset, `go test` runner), `internal/observability/` (Langfuse tracing), async task pattern `202 Accepted` + `GET /tasks/:id`.
- Phase 6: `internal/document/` — `ExtractStatement(pdf []byte) (Statement, error)` with `Transaction`/`Statement` structs, extraction via Claude vision + OCR hybrid, validation with `go-playground/validator`.
- Phase 7: UX lab static prototype demonstrating 7 AI-native UX patterns.

Phase 8 integrates these into a single product: Finanças IA, a personal-finance assistant that ingests credit-card PDFs and answers Portuguese natural-language questions such as "quanto gastei com mercado em abril?".

## Goals / Non-Goals

**Goals:**

- Ship a runnable Go backend (`cmd/financas-ia/`) with four endpoints: `POST /chat`, `POST /ingest`, `GET /tasks/:id`, `GET /health`.
- Store statements, chunks, and embeddings in Postgres + pgvector via `pgx`.
- Ingest PDF statements end-to-end: upload → extract → chunk → embed → store.
- Stream chat responses to the browser via SSE with tool-call progress visible as pending actions.
- Provide a minimal static frontend covering drag-and-drop upload, chat stream, confirmation card, pending-actions indicator, and cost badge.
- Run 30 intent evals through `go test` with golden dataset + LLM-as-judge.
- Package the application for local deployment with multi-stage Dockerfile and `docker-compose.yml` including Postgres, Langfuse, and Redis.
- Publish a README with architecture diagram, setup, eval results, and a 1-minute Loom demo.

**Non-Goals:**

- Multi-user authentication/authorization (single-user, local-first).
- Mobile app or native client.
- Calendar/reminders/notes RAG (finances only).
- Fine-tuning or custom model training.
- Cloud production deployment (milestone is `docker compose up`).
- Real-time transaction sync with banks (manual PDF upload only).
- OCR via Python inside the Go process; Python is only invoked via `os/exec` in Phase 6 and remains a local tool dependency, not a runtime service.

## Decisions

### D1: Use `net/http` ServeMux, not `chi`

The backend will use the standard library `net/http` `ServeMux` for routing. `POST /chat`, `POST /ingest`, `GET /tasks/{id}`, and `GET /health` are simple enough that `ServeMux` handles them cleanly in Go 1.22+ with pattern variables (`GET /tasks/{id}`).

**Why:** The roadmap constraint is "`net/http` over SDKs" and the project avoids extra dependencies unless justified. The four endpoints do not need middleware chains beyond a small logging wrapper.

**Alternative considered:** `github.com/go-chi/chi/v5`. Rejected — it adds a dependency for routing that the standard library already provides.

### D2: `pgx/v5` direct pool, no ORM

Database access uses `github.com/jackc/pgx/v5/pgxpool` with hand-written SQL. Migrations are versioned SQL files applied by an embedded migration runner or `cmd/migrate` using `golang-migrate`.

**Why:** The roadmap explicitly prefers `pgx` and avoids ORMs. Financial data benefits from explicit schema, parameterized queries, and reviewable migrations.

**Alternative considered:** `database/sql` + lib/pq. Rejected — `pgx` is the modern Postgres driver, supports `pgvector` arrays efficiently, and is already recommended in the roadmap resources.

### D3: Repository pattern with domain errors

`internal/financas/repository` defines an interface (`StatementRepository`, `ChunkRepository`) consumed by use cases. Postgres implementations live in `internal/financas/repository/postgres`. Domain errors such as `ErrStatementNotFound` are typed sentinel errors so handlers can map them to HTTP status codes.

**Why:** Keeps orchestration testable with fakes; supports switching storage later; maps DB `sql.ErrNoRows` to domain errors using `errors.Is`.

### D4: Async ingestion with `GET /tasks/:id`

`POST /ingest` accepts the PDF, returns `202 Accepted` with a `task_id`, and runs extraction/chunking/embedding in a background goroutine. State is kept in Redis with a TTL. `GET /tasks/:id` returns `{status, message, result?, error?}`.

**Why:** PDF extraction and embedding can take 10–60 seconds, far beyond a reasonable HTTP timeout. The async pattern was prototyped in Phase 5 (MP28) and is reused here.

**Trade-off:** Adds Redis as a runtime dependency. The `docker-compose.yml` includes Redis; local development can use an in-memory task store if `REDIS_URL` is unset, gated by build tag or env fallback.

### D5: SSE streaming over WebSockets

Chat responses stream to the browser via `text/event-stream` (SSE). Each SSE event is a JSON envelope: `{"type":"delta","content":"..."}`, `{"type":"tool_call","name":"...","status":"pending"}`, `{"type":"done"}`, or `{"type":"error","message":"..."}`.

**Why:** SSE is unidirectional server→client, works over HTTP, reconnects automatically, and avoids WebSocket handshake complexity. Streaming text + tool-call metadata fits one event stream.

**Alternative considered:** WebSockets. Rejected — bidirectional chat is not required; SSE is simpler and aligns with the "`net/http` over SDKs" philosophy.

### D6: Agent tool calls for finance queries

The chat orchestrator wraps `internal/agent/Loop` with finance-specific tools:

- `search_transactions(query, category?, month?, year?)` → hybrid search over chunks + optional rerank.
- `list_statements()` → list uploaded statement periods/banks.
- `ingest_statement(file)` → not invoked by chat; reserved for confirmation flow from the frontend upload.

**Why:** Reuses the agent loop from Phase 3. Finance-specific tools keep the agent focused and prevent open-ended tool selection.

### D7: Context-aware graceful shutdown

`main` creates `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`. The HTTP server uses `http.Server.Shutdown(ctx)`. Background ingestion workers listen on `ctx.Done()` and finish the current step before exiting.

**Why:** Prevents data corruption during ingestion and cleanly closes the `pgx` pool.

### D8: Evals integrated into `go test`

Intent evals are table-driven tests in `internal/financas/evals/eval_test.go`. Each case sends a query through the chat orchestrator (using fake repositories and a fake provider when possible) and asserts the expected tool call or answer contains expected facts. LLM-as-judge runs only under `//go:build eval` to keep unit tests fast.

**Why:** The roadmap requires evals to run via `go test`. Keeping LLM-as-judge behind a build tag prevents slow tests from blocking normal development.

### D9: Observability via Langfuse self-hosted

Traces are sent to a self-hosted Langfuse instance defined in `docker-compose.yml`. The `internal/observability/` wrapper from Phase 5 records spans for HTTP requests, DB queries, LLM calls, and ingestion steps.

**Why:** Reuses Phase 5 observability package; keeps cost and latency visible in a local dashboard.

### D10: Security boundaries for uploaded PDFs

Uploaded PDFs are validated by MIME type (`application/pdf`) and size limit (10 MB), stored in Postgres as `BYTEA` or on a bounded local temp path scoped by task ID, and never executed. OCR subprocess runs with the file path as a separate argument, not via shell interpolation.

**Why:** PDFs are untrusted input. File-size limits prevent DoS; path scoping prevents traversal; parameterized subprocess args prevent command injection.

## Risks / Trade-offs

- **[Long ingestion latency]** → PDF extraction + embedding can be slow. Mitigation: async task model with Redis-backed state and `GET /tasks/:id` polling.
- **[Embedding cost on repeated uploads]** → Same statement uploaded twice incurs duplicate embedding. Mitigation: dedup by `sha256(pdf bytes)` on `statements` table; skip extraction if already present.
- **[SSE reconnect loses context]** → Browser reconnects mid-stream. Mitigation: keep full assistant response in memory or DB and allow full-stream replay via `GET /chat/:id`; out of scope for MVP — documented as future work.
- **[PII in PDFs and logs]** → Credit-card statements contain sensitive data. Mitigation: no PII in `slog` (log only statement IDs and file hashes), redact full card numbers during extraction, run locally by default.
- **[Tool-call hallucination on financial answers]** → Agent may answer without retrieving. Mitigation: system prompt requires tool use before answering; eval suite asserts `search_transactions` is called for spending questions.
- **[pgvector extension not available]** → Standard Postgres image lacks pgvector. Mitigation: use `pgvector/pgvector:pg16` image in Docker Compose; migrations check `CREATE EXTENSION IF NOT EXISTS vector`.
- **[Langfuse optional]** → Tracing fails if Langfuse is down. Mitigation: observability wrapper logs errors but never crashes the request path.

## Migration Plan

Not applicable — this is a greenfield capstone addition. Existing packages and binaries are untouched. Rollback: remove `cmd/financas-ia/`, `internal/financas/`, `web/`, `deploy/`, and `docs/financas-ia/`.

## Open Questions

1. Should the frontend be served by the Go binary (`embed.FS`) or opened as a static `file://` page? Decision: served by Go under `GET /` so CORS and SSE origin are consistent.
2. Should uploaded PDF bytes be stored in Postgres `BYTEA` or local disk? Decision: Postgres `BYTEA` for portability; local disk acceptable only for dev without a database.
3. Which embedding model is default? Decision: `voyage-3-large` per Phase 2; allow override via `EMBEDDING_MODEL` env var.
