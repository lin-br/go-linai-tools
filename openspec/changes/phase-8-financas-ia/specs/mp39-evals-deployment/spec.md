## ADDED Requirements

### Requirement: 30 intent evals run via go test

The system SHALL provide at least 30 intent evaluation cases in `internal/financas/evals/intent_eval_test.go`. Each case SHALL include `name`, `query` (Portuguese natural language), `expected_tool` (e.g., `search_transactions`, `list_statements`), optional `expected_arguments` map, and optional `expected_answer_contains` string. Evals SHALL be run with `go test ./internal/financas/evals/...`.

#### Scenario: Spending query expects search_transactions

- **WHEN** running `go test ./internal/financas/evals/...`
- **THEN** the test case "quanto gastei com mercado em abril?" SHALL assert that `search_transactions` was invoked with `category="mercado"` and `month=4`

#### Scenario: Listing statements expects list_statements

- **WHEN** running `go test ./internal/financas/evals/...`
- **THEN** the test case "quais faturas estão disponíveis?" SHALL assert that `list_statements` was invoked

#### Scenario: Answer contains expected fact

- **WHEN** the eval case asks "qual foi o total da fatura de abril?"
- **THEN** the orchestrated answer SHALL contain a substring matching the expected total (e.g., a regex for currency value) given a seeded fake statement

### Requirement: Deterministic evals use fake repositories and fake provider

The system SHALL support deterministic intent evals by injecting fake implementations of `StatementRepository`, `EmbeddingRepository`, and `outbound.Provider`. Fake repositories SHALL return seeded `Statement` and `Chunk` records. Fake provider SHALL return fixed tool-call responses so evals do not require external API calls or network access.

#### Scenario: Eval without network

- **WHEN** running intent evals with `FAKE_PROVIDER=1` or equivalent test helper
- **THEN** all 30 cases SHALL pass without calling Voyage, Cohere, Anthropic, or Postgres

### Requirement: LLM-as-judge evals gated by build tag

The system SHALL provide behavioral evals that call an LLM to judge answer quality, gated behind `//go:build eval`. These evals SHALL run only when `go test -tags=eval ./internal/financas/evals/...` is executed. They SHALL include a rubric (1–5) for correctness, completeness, and Portuguese fluency.

#### Scenario: LLM judge runs with build tag

- **WHEN** running `go test -tags=eval ./internal/financas/evals/...`
- **THEN** the behavioral evals SHALL score a sample of intent answers and print pass/fail per rubric

#### Scenario: LLM judge skipped by default

- **WHEN** running `go test ./internal/financas/evals/...` without tags
- **THEN** the LLM-as-judge tests SHALL be skipped

### Requirement: Unit tests cover repository and orchestrator layers

The system SHALL include table-driven unit tests for repository methods (with fake pgx or testcontainers) and chat orchestrator behavior (with fake provider and fakes). Coverage targets are informative: aim for >60% of non-error paths in `internal/financas/`.

#### Scenario: Repository insert and retrieval

- **WHEN** running `go test ./internal/financas/repository/...`
- **THEN** tests for `CreateStatement`, `GetStatementByHash`, `CreateChunks`, and `SearchEmbeddings` SHALL pass

#### Scenario: Orchestrator emits expected events

- **WHEN** running `go test ./internal/financas/chat/...`
- **THEN** tests SHALL verify the orchestrator emits `tool_call`, `source`, `text`, and `done` events in the correct order

### Requirement: Multi-stage Dockerfile produces small image

The system SHALL provide a `Dockerfile` at the repository root (or `build/Dockerfile`) with a multi-stage build. The final image SHALL be based on `scratch` or `distroless` and the binary size SHALL be approximately 10 MB or smaller when compressed. The build stage SHALL use `golang:1.26.4-alpine` and the final stage SHALL copy only the compiled binary and embedded web assets.

#### Scenario: Docker build succeeds

- **WHEN** running `docker build -t financas-ia .`
- **THEN** it SHALL complete without errors and produce an image tagged `financas-ia`

#### Scenario: Image runs the server

- **WHEN** running `docker run -e DATABASE_URL=... financas-ia`
- **THEN** the container SHALL start, apply migrations, and listen on port `8080`

### Requirement: Docker Compose orchestrates full stack

The system SHALL provide `docker-compose.yml` defining services: `app`, `postgres` (with `pgvector/pgvector:pg16`), `langfuse`, and `redis`. The `app` service SHALL depend on `postgres` and `redis` with health checks. The Langfuse service SHALL use the official Langfuse local image and expose a web UI.

#### Scenario: docker compose up starts all services

- **WHEN** running `docker compose up -d`
- **THEN** all four services SHALL reach healthy state within 60 seconds

#### Scenario: App health depends on Postgres

- **WHEN** the `postgres` service is unhealthy
- **THEN** the `app` health check SHALL fail until Postgres recovers

### Requirement: Health endpoint checks DB and Claude

The system SHALL implement `GET /health` to verify Postgres connectivity and LLM provider reachability. The LLM check SHALL be lightweight (e.g., a cached model availability check or a small non-streaming request) and SHALL timeout within 5 seconds.

#### Scenario: Healthy dependencies pass health check

- **WHEN** Postgres and the LLM provider are reachable
- **THEN** `GET /health` SHALL return `200 OK` and `{"status":"healthy"}`

#### Scenario: Claude unreachable returns unhealthy

- **WHEN** the LLM provider returns a connection error
- **THEN** `GET /health` SHALL return `503 Service Unavailable` with `checks.claude: "unhealthy"`

### Requirement: Graceful shutdown in container

The system SHALL handle `SIGTERM` in the container by closing the HTTP server, draining in-flight requests for up to 30 seconds, and closing the `pgx` pool. The `Dockerfile` SHALL use `CMD ["/financas-ia"]` so Docker sends `SIGTERM` to the process directly.

#### Scenario: SIGTERM during request

- **WHEN** `docker stop` is issued while a request is in-flight
- **THEN** the container SHALL finish the request or time out after 30s and exit cleanly with code 0
