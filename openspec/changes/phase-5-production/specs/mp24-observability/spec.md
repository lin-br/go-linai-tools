## ADDED Requirements

### Requirement: Observability package provides a traced provider decorator

The system SHALL provide package `internal/observability` with `TracedProvider` struct implementing `outbound.Provider`. `NewTracedProvider(inner outbound.Provider, cfg LangfuseConfig) *TracedProvider` SHALL wrap `inner` and emit a trace/observation for every `Chat` and `ChatStream` call. The decorator SHALL record `model`, `input_tokens`, `output_tokens`, `total_tokens`, `latency_ms`, and `cost_usd` from `domain.Usage`.

#### Scenario: Chat call emits a trace

- **WHEN** `TracedProvider.Chat(ctx, req)` is called and the inner provider returns a `ChatResponse` with `Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}`
- **THEN** a Langfuse observation SHALL be created with attributes `input_tokens=10`, `output_tokens=5`, and `total_tokens=15`

#### Scenario: ChatStream emits a trace after the stream closes

- **WHEN** `TracedProvider.ChatStream(ctx, req)` is called and the consumer reads all stream events to completion
- **THEN** a Langfuse observation SHALL be created after the channel closes with aggregated token counts and model name

#### Scenario: Errors are recorded on the observation

- **WHEN** `TracedProvider.Chat(ctx, req)` returns a non-nil error
- **THEN** the observation SHALL record `error: true` and the error message, and the error SHALL be propagated unchanged

### Requirement: Langfuse client uses net/http and env-based auth

The system SHALL provide `internal/observability/langfuse/client.go` with `Client` struct and methods `CreateTrace(ctx, name string) (string, error)` and `CreateObservation(ctx, traceID string, obs Observation) error`. The client SHALL use `net/http.NewRequestWithContext`, set `Authorization: Basic {base64(publicKey + ":" + secretKey)}`, and send JSON bodies. `LangfuseConfig` SHALL read `LANGFUSE_BASE_URL`, `LANGFUSE_PUBLIC_KEY`, and `LANGFUSE_SECRET_KEY` from environment variables.

#### Scenario: Missing env vars return an error

- **WHEN** `LoadConfig()` is called and `LANGFUSE_BASE_URL` is empty
- **THEN** it SHALL return an error equal to `ErrMissingLangfuseConfig`

#### Scenario: CreateTrace posts JSON and returns the trace ID

- **WHEN** `CreateTrace(ctx, "chat-request")` is called against a mock HTTP server returning `{"id":"trace-123"}`
- **THEN** the method SHALL return `"trace-123"` and no error

### Requirement: Docker Compose runs Langfuse locally

The system SHALL provide `docker/langfuse/docker-compose.yml` that starts Langfuse, Postgres, and Redis services. The Compose file SHALL expose Langfuse on port `3000` and SHALL persist Postgres data in a named volume. A `README.md` snippet SHALL document `docker compose -f docker/langfuse/docker-compose.yml up -d`.

#### Scenario: Compose starts all services

- **WHEN** the user runs `docker compose -f docker/langfuse/docker-compose.yml up -d`
- **THEN** Langfuse SHALL be reachable at `http://localhost:3000` and the Postgres container SHALL be running

### Requirement: Wiring composes traced provider with retry provider

The system SHALL construct the provider chain as `configs.LoadConfigs()` → `OpenRouterProvider` → `RetryProvider` → `TracedProvider` (or `OpenRouterProvider` → `TracedProvider` → `RetryProvider` if retries should also be traced). The chosen order SHALL be documented in `internal/observability/doc.go` and applied consistently in `cmd/cli/main.go` and any future entry points.

#### Scenario: CLI uses traced provider

- **WHEN** `go run ./cmd/cli` starts with valid Langfuse env vars
- **THEN** every provider call SHALL create a Langfuse trace visible in the dashboard

### Requirement: Tracing is opt-in and safe when disabled

The system SHALL allow `LANGFUSE_ENABLED=false` (or unset) to make `NewTracedProvider` return the inner provider unchanged. When disabled, `TracedProvider` SHALL delegate without making HTTP calls and SHALL NOT require Langfuse credentials.

#### Scenario: Tracing disabled

- **WHEN** `LANGFUSE_ENABLED=false` and a provider call is made
- **THEN** no Langfuse HTTP request SHALL be sent and the call SHALL behave exactly like the inner provider
