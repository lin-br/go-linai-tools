## 1. Evals Package (MP23)

- [ ] 1.1 Create `internal/evals/runner.go` with `Case`, `Checker`, `Runner`, and `Report` types
- [ ] 1.2 Implement `NewExactChecker(expected string) Checker` for deterministic unit evals
- [ ] 1.3 Implement `NewLLMJudgeChecker(provider outbound.Provider, model, rubric string) Checker` that parses a 1–5 score
- [ ] 1.4 Implement `Runner.Run(ctx context.Context, cases []Case) (Report, error)` and `Report.PassRate() float64`
- [ ] 1.5 Add `internal/evals/dataset.go` with `LoadGoldenDataset(path string) ([]Case, error)` and row-to-case mapping
- [ ] 1.6 Create `internal/evals/testdata/golden.jsonl` with exactly 30 rows (`query`, `expected_answer`, `expected_tool`, `judge`)
- [ ] 1.7 Create `internal/evals/evals_test.go` with table-driven tests for checker functions and a `TestGoldenDataset` runner
- [ ] 1.8 Add support for `SKIP_LLM_JUDGE` env var to skip behavioral evals in short test runs

## 2. Observability / Langfuse (MP24)

- [ ] 2.1 Create `docker/langfuse/docker-compose.yml` with Langfuse, Postgres, and Redis services on port 3000
- [ ] 2.2 Create `internal/observability/langfuse/client.go` with `Client`, `CreateTrace`, `CreateObservation`, and env-based `LoadConfig`
- [ ] 2.3 Define `LangfuseConfig` struct reading `LANGFUSE_BASE_URL`, `LANGFUSE_PUBLIC_KEY`, `LANGFUSE_SECRET_KEY`, `LANGFUSE_ENABLED`
- [ ] 2.4 Create `internal/observability/traced_provider.go` with `TracedProvider` implementing `outbound.Provider`
- [ ] 2.5 Implement `Chat` tracing: start span, call inner provider, record tokens/latency/cost/model, handle errors
- [ ] 2.6 Implement `ChatStream` tracing: aggregate stream events and emit observation after channel close
- [ ] 2.7 Add `internal/observability/doc.go` documenting the wiring order (retry → trace → cache → provider)
- [ ] 2.8 Wire `TracedProvider` into `cmd/cli/main.go` when `LANGFUSE_ENABLED=true`
- [ ] 2.9 Add `internal/observability/langfuse/client_test.go` using `httptest.Server` for CreateTrace/CreateObservation

## 3. Cost CLI (MP25)

- [ ] 3.1 Create `internal/observability/langfuse/cost_client.go` with `CostClient` and `ListObservations(ctx, since, until)`
- [ ] 3.2 Implement pagination handling via `nextCursor` in `ListObservations`
- [ ] 3.3 Create `cmd/cost/main.go` parsing `-since`, `-until`, `-by` (`model` or `feature`), and `-output` (`table`/`json`) flags
- [ ] 3.4 Implement default `-since` as last Monday 00:00:00 UTC and default `-until` as `time.Now()`
- [ ] 3.5 Aggregate observations by model and by `feature` metadata field
- [ ] 3.6 Print table output by default and JSON output when `-output json` is provided
- [ ] 3.7 Handle zero-spend case with a user-friendly message and exit 0
- [ ] 3.8 Add `cmd/cost/main_test.go` with `httptest` mock Langfuse responses

## 4. Cost Discipline (MP26)

- [ ] 4.1 Create `internal/cost/cache.go` with `Cache` interface and `InMemoryCache` implementation using `sync.RWMutex`
- [ ] 4.2 Implement `BuildCacheKey(prompt, model string, params map[string]any) string` with `sha256` and sorted JSON
- [ ] 4.3 Create `internal/cost/cached_provider.go` with `CachedProvider` implementing `outbound.Provider`
- [ ] 4.4 Ensure `ChatStream` bypasses cache and always delegates to inner provider
- [ ] 4.5 Create `internal/cost/compress.go` with `Compressor.Compress(ctx, prompt string) (string, error)` using a cheap model
- [ ] 4.6 Implement short-prompt bypass (< 50 runes) without calling the provider
- [ ] 4.7 Create `internal/cost/router.go` with `Router.Route(taskClass string) string` and default table
- [ ] 4.8 Wire cache, compressor, and router into a representative use case or entry point with documented order
- [ ] 4.9 Add `internal/cost/cache_test.go` with concurrent access covered by `go test -race`

## 5. Security Light (MP27)

- [ ] 5.1 Create `internal/security/sanitize.go` with `StripInjections(input string) string` covering common delimiters
- [ ] 5.2 Create `internal/security/log_sanitizer.go` with `PIIMaskHandler` wrapping `slog.Handler`
- [ ] 5.3 Add regex patterns for CPF, email, credit card, and phone number in `PIIMaskHandler`
- [ ] 5.4 Create `internal/security/banned.go` with `BannedDetector` and `Detect(output string) (bool, []string)`
- [ ] 5.5 Create `internal/security/sanitized_provider.go` with `SanitizedProvider` implementing `outbound.Provider`
- [ ] 5.6 Ensure `SanitizedProvider` strips injections from request messages and runs banned detection on response messages
- [ ] 5.7 Add `ErrBannedOutput` sentinel error and `ErrInvalidPattern` for invalid regex
- [ ] 5.8 Wire `SanitizedProvider` as an optional layer gated by `SECURITY_ENABLED=true`
- [ ] 5.9 Add `internal/security/sanitize_test.go`, `banned_test.go`, and `log_sanitizer_test.go`

## 6. Async Tasks (MP28)

- [ ] 6.1 Create `internal/async/queue.go` with `Queue`, `Submit`, `Status`, `Shutdown`, and `TaskStatus` types
- [ ] 6.2 Implement unique `task_id` generation using `crypto/rand` or a monotonic counter
- [ ] 6.3 Add `ErrTaskNotFound` and `ErrQueueClosed` sentinel errors
- [ ] 6.4 Create `internal/async/http.go` with `Handler` mounting `POST /tasks` and `GET /tasks/:id`
- [ ] 6.5 Implement `POST /tasks` returning `202 Accepted` with `{task_id, status, poll_url}`
- [ ] 6.6 Implement `GET /tasks/:id` returning `200 OK` or `404 Not Found`
- [ ] 6.7 Create `internal/async/agent.go` adapter that runs `agent.Loop.Run` as a task function
- [ ] 6.8 Register `"agent-run"` task name that accepts payload `{"query":"..."}`
- [ ] 6.9 Add `internal/async/queue_test.go` and `http_test.go` with `httptest`

## 7. Documentation and Verification

- [ ] 7.1 Create `docs/concepts/llmops.md` summarizing evals, tracing, cost discipline, security, and async learnings
- [ ] 7.2 Run `go build ./...` and fix any compilation errors
- [ ] 7.3 Run `go vet ./...` and resolve warnings
- [ ] 7.4 Run `go test ./...` and ensure all deterministic tests pass
- [ ] 7.5 Run `go test -race ./internal/cost/... ./internal/security/... ./internal/async/...`
- [ ] 7.6 Start Langfuse with `docker compose -f docker/langfuse/docker-compose.yml up -d` and verify port 3000
- [ ] 7.7 Run `go run ./cmd/cost -output json` against the local Langfuse instance (with optional seed traces)
