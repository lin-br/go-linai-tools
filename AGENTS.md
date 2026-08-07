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

- `OPENROUTER_API_KEY` — bearer token for OpenRouter
- `DEFAULT_MODEL` — fallback model (e.g. `anthropic/claude-sonnet-4-20250514`)
- `PRO_MODEL`, `FREE_MODEL` — alternatives resolved by `configs.Models.Get()`

The app `log.Fatal`s if `OPENROUTER_API_KEY` or any resolved model is missing.

## Entry points

- `main.go` — minimal REPL; prints "What you need?", reads one line, calls the LLM, prints response, exits.
- `cmd/cli/main.go` — interactive agent; greets, loops on stdin, prints each model response.

Both construct the same wiring: `configs.LoadConfigs()` → `OpenRouterClient` → `DoSendMessageUseCase` → CLI adapter.

## Architecture

Hexagonal-ish layout:

- `internal/core/domain` — plain structs (`Request`, `Response`).
- `internal/core/ports/inbound` — `Entrypoint` interface (driving adapter contract).
- `internal/core/ports/outbound` — `ProviderModelHandler` interface (driven adapter contract).
- `internal/core/usecases` — `DoSendMessageUseCase` orchestrates model selection + provider call.
- `internal/adapters/driven/http_clients` — OpenRouter HTTP client.
- `internal/adapters/driving` — `CLI` implementation of `Entrypoint`.
- `internal/configs` — YAML + env-var config loader.

## Important quirks

- The request/response types in `internal/adapters/driven/http_clients/` are **Anthropic Messages API-shaped**, but the actual client posts to **OpenRouter** (`https://openrouter.ai/api/v1/messages`). The payload builder currently sends a simple OpenAI-compatible `model` + `messages` body, not the full Anthropic schema. Mismatch risk if you extend the Anthropic structs.
- YAML dependency is `go.yaml.in/yaml/v4` (not `gopkg.in/yaml.v3`).
- `OpenRouterClient` has a hard 5-minute HTTP timeout and zero retries/backoff.
- There are no unit tests; verify by running `go build ./...` and `go test ./...`.

## Conventions to preserve

- Go-first. Do not introduce Python/TS unless the roadmap explicitly calls for it (Phase 6 OCR is the only planned exception).
- Prefer `net/http` over SDKs to learn the wire protocol.
- Each phase has a fixed deliverables list and a LinkedIn post. Don't expand scope; close the phase, then post.

## Next action (as of last sync)

Finish Phase 1: build the 3 CLIs (`summarize`, `extract`, `spec-to-code`), add `StreamClient` (SSE), and a retry wrapper. See `docs/roadmap-ai-engineer-status.md` § Phase 1 for completion criteria.
