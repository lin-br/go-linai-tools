# AGENTS.md — go-linai-tools

> Compact context for OpenCode sessions. Source of truth for project intent: `docs/roadmap-ai-engineer-status.md`.

## What this repo is

Personal learning project for an 18-week AI Engineer roadmap (Go-first). Currently in **Phase 1 — Hands on the API, Go-style**. Read `docs/roadmap-ai-engineer-status.md` before doing any non-trivial work; it defines the current phase, deliverables, and the single next action.

## Build & run

```bash
go build ./...          # compile all packages
go test ./...           # currently no tests; returns [no test files]
go run .                # root main.go: one-shot stdin prompt
go run ./cmd/cli        # interactive CLI agent that loops on stdin
```

No Makefile, no CI, no lint/typecheck config exists yet. Use standard `go` tooling.

## Configuration

Runtime config lives in `internal/configs/configs.yaml` and is loaded with `os.ExpandEnv`. Required env vars:

- `OPENROUTER_API_KEY` — bearer token for OpenRouter (required when `provider: openrouter`)
- `DEFAULT_MODEL` — fallback model (e.g. `anthropic/claude-sonnet-4-20250514`)
- `PRO_MODEL`, `FREE_MODEL` — alternatives resolved by `configs.Models.Get()`

The app `log.Fatal`s if the active provider's credentials or any resolved model is missing.

## Entry points

- `main.go` — minimal REPL; prints "What you need?", reads one line, calls the LLM, prints response, exits.
- `cmd/cli/main.go` — interactive agent; greets, loops on stdin, prints each model response.

Both construct the same wiring: `configs.LoadConfigs()` → `OpenRouterProvider` → `DoSendMessageUseCase` → CLI adapter.

## Architecture

Hexagonal-ish layout:

- `internal/core/domain` — provider-agnostic structs (`ChatRequest`, `ChatResponse`, `Message`, `Tool`, `ToolCall`, `ToolChoice`, `Usage`, `StreamEvent`).
- `internal/core/ports/inbound` — `Entrypoint` interface (`StartAgent` with `context.Context`).
- `internal/core/ports/outbound` — `Provider` interface (`Chat` and `ChatStream` with `context.Context`).
- `internal/core/usecases` — `DoSendMessageUseCase` orchestrates model selection + provider call.
- `internal/adapters/driven/http_clients` — OpenRouter HTTP client.
- `internal/adapters/driving` — `CLI` implementation of `Entrypoint`.
- `internal/configs` — YAML + env-var config loader.

## Important quirks

- The `OpenRouterProvider` posts to **OpenRouter** (`https://openrouter.ai/api/v1/chat/completions`) using OpenAI-compatible wire types (`ChatCompletionRequest`, `ChatCompletionResponse`, `ChatCompletionChunk`) defined in `internal/adapters/driven/http_clients/openai_wire.go`. The existing `anthropic_request.go` / `anthropic_response.go` types are preserved for a future `AnthropicProvider` and are not used by the OpenRouter adapter.
- YAML dependency is `go.yaml.in/yaml/v4` (not `gopkg.in/yaml.v3`).
- `OpenRouterProvider` uses `http.NewRequestWithContext` and has no hardcoded timeout — timeouts and retries are managed via `context.Context` and will be added in MP2.
- There are no unit tests; verify by running `go build ./...` and `go test ./...`.

## Conventions to preserve

- Go-first. Do not introduce Python/TS unless the roadmap explicitly calls for it (Phase 6 OCR is the only planned exception).
- Prefer `net/http` over SDKs to learn the wire protocol.
- Each phase has a fixed deliverables list and a LinkedIn post. Don't expand scope; close the phase, then post.

## Next action (as of last sync)

Finish Phase 1: build the 3 CLIs (`summarize`, `extract`, `spec-to-code`), add `StreamClient` (SSE), and a retry wrapper. See `docs/roadmap-ai-engineer-status.md` § Phase 1 for completion criteria.
