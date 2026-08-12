## Why

Phase 2 RAG and Phase 3 agents produce working code, but the project still lacks the operational scaffolding that turns a prototype into something reproducible and affordable: evals that prove quality, traces that explain spend, cost controls that prevent runaway bills, and async handling for long agent runs. Phase 5 closes the POC→product gap by shipping a lightweight LLMOps layer directly inside the monorepo, reusing the existing `outbound.Provider` boundary.

## What Changes

- Add `internal/evals/` — deterministic unit evals, LLM-as-judge behavioral evals, a 30-row golden Q/A dataset, and a `go test` runner that reports pass/fail per case.
- Add `internal/observability/` — a thin tracing wrapper around `outbound.Provider` that emits token counts, latency, cost, and model name to a self-hosted Langfuse instance via Docker Compose.
- Add `cmd/cost/` — a CLI that queries Langfuse traces and prints spend grouped by model and feature for the current week.
- Add cost-discipline primitives in `internal/cost/` — response cache keyed by `sha256(prompt+model+params)`, prompt compression by a cheaper model, and a model routing table that picks Haiku/Sonnet/Opus per task class.
- Add `internal/security/` — prompt-injection stripping, PII masking for `slog` output, and banned-output regex detection.
- Add async task support in `internal/async/` — enqueue long-running agent work behind `202 Accepted` + `task_id`, with `GET /tasks/:id` polling.
- Add `docs/concepts/llmops.md` summarizing the Phase 5 learnings.

## Capabilities

### New Capabilities

- `mp23-evals-package`: Eval framework with unit, behavioral (LLM-as-judge), and golden-dataset runner integrated into `go test`.
- `mp24-observability`: Self-hosted Langfuse tracing of provider calls (tokens, latency, cost, model).
- `mp25-cost-cli`: CLI that reads Langfuse traces and prints weekly spend by model and feature.
- `mp26-cost-discipline`: Response cache, prompt compression, and model routing table to reduce inference spend.
- `mp27-security-light`: Prompt-injection stripping, PII masking in logs, and banned-output regex detection.
- `mp28-async-tasks`: Async task queue returning `202 Accepted` + `task_id` with `GET /tasks/:id` polling.

### Modified Capabilities

(None. Phase 5 only adds new packages and CLIs; it does not change existing specs for `Provider`, RAG, or the agent loop.)

## Impact

- **New files**:
  - `internal/evals/*.go` and `internal/evals/testdata/golden.jsonl`
  - `internal/observability/*.go` and `docker/langfuse/docker-compose.yml`
  - `cmd/cost/main.go`
  - `internal/cost/*.go`
  - `internal/security/*.go`
  - `internal/async/*.go`
  - `docs/concepts/llmops.md`
- **Dependencies**: self-hosted Langfuse runs via Docker Compose (no managed APM, no EKS); everything else is Go stdlib plus existing project packages.
- **No breaking changes**: `outbound.Provider` is consumed, not modified. Existing CLIs keep working without the tracing wrapper.
- **Cost model**: default tracing is opt-in via config; cache and router reduce spend when enabled.
