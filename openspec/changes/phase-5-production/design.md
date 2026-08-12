## Context

By the end of Phase 3 the repo has a working agent loop and by Phase 4 it has a multi-provider `Provider` boundary. Phase 5 is about making those capabilities production-shaped: measuring quality before shipping, watching spend, spending less, sanitizing inputs/outputs, and not blocking HTTP clients while a long agent runs. All of Phase 5 sits downstream of the existing `outbound.Provider` interface (`Chat` and `ChatStream`) and should be usable by the RAG CLI, agent loop, and the Finanças IA showcase in Phase 8.

The constraint set is intentionally tight: Go 1.26.4, `net/http` over SDKs, `slog` for logging, `context.Context` propagation, and no Python. Observability is self-hosted Langfuse in Docker Compose, not a managed APM or EKS deployment.

## Goals / Non-Goals

**Goals:**
- Ship an eval package (`internal/evals/`) with deterministic unit checks, rubric-based LLM-as-judge checks, a 30-row golden dataset, and a `go test` runner.
- Instrument every provider call through a tracing wrapper that records tokens, latency, model, and cost to a self-hosted Langfuse instance.
- Provide a cost CLI (`cmd/cost/`) that reads Langfuse traces and prints spend grouped by model and feature for the current week.
- Reduce inference spend via response cache, prompt compression, and a task-to-model routing table.
- Add lightweight security: prompt-injection stripping, PII masking in `slog` output, and regex-based banned-output detection.
- Support async long-running agents with `202 Accepted` + `task_id` and `GET /tasks/:id` polling.
- Document Phase 5 learnings in `docs/concepts/llmops.md`.

**Non-Goals:**
- Managed cloud tracing (Datadog, Honeycomb, AWS X-Ray). Langfuse self-hosted only.
- Persistent queue with guaranteed delivery (Redis/RabbitMQ/SQS). Async tasks live in-memory for Phase 5.
- Full authn/authz or multi-tenancy.
- Fine-tuning or custom model training.
- Python OCR or document parsing — Phase 6 only.
- Blocking releases on 100% eval coverage; the target is a runnable eval suite, not a CI gate.

## Decisions

### D1: Trace provider calls with a decorator, not by editing implementations

Create `internal/observability/traced_provider.go` defining `TracedProvider` that wraps `outbound.Provider`. It implements `Chat` and `ChatStream`, starts a Langfuse span/observation, delegates to the inner provider, then records `input_tokens`, `output_tokens`, `model`, `latency_ms`, and `cost_usd` from `domain.Usage`.

**Why:** Keeps `OpenRouterProvider` and future providers free of observability code. Any provider can be traced by wrapping it at wiring time, matching the decorator pattern already used for `RetryProvider`.

**Alternative considered:** Emit traces inside each provider implementation. Rejected — it duplicates code across providers and makes the interface harder to test.

### D2: Langfuse self-hosted via Docker Compose

Provide `docker/langfuse/docker-compose.yml` running Langfuse with Postgres. The app connects via env vars `LANGFUSE_BASE_URL`, `LANGFUSE_PUBLIC_KEY`, `LANGFUSE_SECRET_KEY`.

**Why:** The roadmap explicitly says self-hosted, not EKS. Docker Compose is the fastest path to a working trace dashboard for a solo project.

**Alternative considered:** Cloud Langfuse. Rejected — conflicts with the self-hosted requirement and adds ongoing cost.

### D3: Cost CLI queries Langfuse HTTP API, not the database

`cmd/cost/main.go` calls `GET /api/public/traces` (or `/api/public/observations`) with basic auth using the public/secret key pair, parses JSON, aggregates `cost` by `model` and metadata `feature`, and prints a table.

**Why:** Avoids coupling the CLI to Langfuse's internal Postgres schema, which can change. The public API is documented and versioned.

**Alternative considered:** Query Postgres directly. Rejected — schema churn risk and extra connection config.

### D4: Response cache keyed by `sha256(prompt+model+params)`

`internal/cost/cache.go` defines `Cache` with `Get(ctx, key string) (*domain.ChatResponse, bool)` and `Set(ctx, key string, resp *domain.ChatResponse)`. The key is `hex(sha256(prompt + "\x00" + model + "\x00" + paramsJSON))`. Cache is in-memory `map[string]cacheEntry` protected by `sync.RWMutex`.

**Why:** Deterministic, fast, no external dependency. The separator prevents collisions between different field compositions.

**Alternative considered:** Use a persistent cache (Redis). Rejected — adds infrastructure for a Phase 5 learning milestone. In-memory is enough to prove the concept.

### D5: Prompt compression via a cheap model

`internal/cost/compress.go` defines `Compressor` that sends the original prompt to a cheap model (default `anthropic/claude-haiku-4-20250514` or `openrouter/quen-2.5-coder`) with a system prompt "Summarize the following user prompt keeping only the intent and constraints." The compressed prompt is cached and used by the real call.

**Why:** Reduces token count for long prompts before an expensive model sees them. Uses the existing `Provider` interface, so it composes with cache and tracer.

**Alternative considered:** Rule-based truncation. Rejected — truncation can lose semantic information; a cheap model preserves intent better.

### D6: Model routing table maps task class to model

`internal/cost/router.go` defines `Router` with `Route(task string) string`. Hardcoded mapping: `classification` → cheap model, `extraction`/`generation` → Sonnet, `reasoning` → Sonnet/Opus. The router is called before the provider.

**Why:** Simple, explainable, and sufficient for a learning project. Keeps cost decisions explicit.

**Alternative considered:** Dynamic routing based on prompt complexity. Rejected — overkill for Phase 5 and harder to test.

### D7: PII masking in `slog` via a custom handler

`internal/security/log_sanitizer.go` defines `PIIMaskHandler` wrapping `slog.Handler`. It scans string attrs for regex patterns (CPF, email, credit card, phone) and replaces matches with `[REDACTED]`.

**Why:** Centralized masking at the logging layer means callers don't have to remember to sanitize every field.

**Alternative considered:** Sanitize at every log call site. Rejected — error-prone and repetitive.

### D8: Async tasks in-memory with simple state machine

`internal/async/queue.go` defines `Queue` with `Submit(ctx, task func(ctx context.Context) error) string` and `Status(id string) (TaskState, error)`. Tasks run in a goroutine; state transitions `pending` → `running` → `done`/`failed` are stored in a mutex-protected map. HTTP handlers return `202 Accepted` with `task_id`; `GET /tasks/:id` returns JSON `{id, state, result, error}`.

**Why:** Avoids adding Redis until Phase 8. In-memory is enough for learning async patterns and supports graceful shutdown via `Queue.Shutdown(ctx)`.

**Alternative considered:** Persist tasks to disk. Rejected — adds complexity without operational benefit for a local/learning setup.

## Risks / Trade-offs

- **[In-memory cache loses state on restart]** → Mitigation: documented limitation. For Phase 8 Finanças IA a persistent cache can replace the interface implementation without changing callers.
- **[Langfuse Docker Compose resource usage]** → Mitigation: document minimum resources (2GB RAM). Make tracing opt-in via config so builds still work without it.
- **[LLM-as-judge evals add cost]** → Mitigation: behavioral evals use a cheap model by default and can be skipped with a `-short` flag.
- **[PII regexes are imperfect]** → Mitigation: mask aggressively, document false-positive risk, and never log full prompts at `Info` level by default.
- **[Async tasks disappear on crash]** → Mitigation: scope Phase 5 to fire-and-forget long agents; Phase 8 can introduce Redis-backed persistence.
- **[Cost CLI depends on Langfuse API shape]** → Mitigation: pin Langfuse image version in `docker-compose.yml`; if API changes, only the thin `langfuse/client.go` needs updating.
