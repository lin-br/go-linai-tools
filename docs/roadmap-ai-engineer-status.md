# AI Engineer Roadmap — Status Brief for AI Agents

> Purpose: a single-file, AI-readable snapshot of the user's 18-week AI Engineer roadmap (v2, Go-first). Read this to know where the user is, what each phase needs, and what the next action is. Source of truth: `wiki/roadmap-ai-engineer-v2.md` + `wiki/journey-ai-engineer.md`.
>
> Last synced: 2026-08-04

---

## TL;DR (machine-readable)

```yaml
roadmap: "AI Engineer v2 (18 weeks, Go-first)"
north_star:
  - "Ship Financas IA — Go backend + chat UI ingesting credit-card PDFs, answering NL questions, exposed as MCP server"
  - "Ship 9 LinkedIn posts (one per phase, within 3 days of phase completion)"
language: Go (net/http over SDKs, to learn the wire protocol)
started: 2026-05-29
current_phase: 2
current_phase_status: complete
phases_complete: [0, 2]
phases_remaining: [1, 3, 4, 5, 6, 7, 8]
hours_invested: 8
app_status: not_started
next_action: "Publish LinkedIn post #2 ('RAG in Go — why your chunks matter more than your vector DB'). Phase 2 is COMPLETE: code + unit tests + live e2e (ingest/query/eval against pgvector with real Voyage/OpenRouter/Cohere keys) all verified. Then close Phase 1's LinkedIn post #1 or start Phase 3."
postgrad_decision: pending (leaning skip)
```

---

## Where I am right now

- **Phase 0 — Mental Model:** COMPLETE (2026-05-29 → 2026-06-18). Deliverable `wiki/concepts/llm-basics.md` shipped. LinkedIn post #0 published 2026-05-31.
- **Phase 1 — Hands on the API, Go-style:** IN PROGRESS (started 2026-06-23). Building `go-llm-tools/` externally. Three CLI mini-projects planned: `summarize`, `extract`, `spec-to-code`. Problems/insights/LinkedIn moment still pending. Its `outbound.Provider` abstraction, `configs` loader, and `retry` wrapper are reused by Phase 2 (dependencies satisfied).
- **Phase 2 — RAG First:** COMPLETE. All seven microphases implemented in-repo: `internal/rag/{embeddings,store,chunk,search,rerank,eval}` + `cmd/rag` (`ingest`/`query`/`eval`). Unit tests green; live end-to-end verified against a local pgvector container with real Voyage/OpenRouter/Cohere keys (ingest stored 4 chunks; query returned a grounded, source-cited answer; eval printed metrics with LLM-judge scores). `openspec validate --changes phase-2-rag` clean. Reuses Phase 1's `outbound.Provider`, `configs` loader, and `retry` wrapper (Phase 1 dependencies satisfied). LinkedIn post #2 pending.
- **Phase 3 onward:** NOT STARTED.

The user is early in Phase 1. No code deliverables have been logged as complete yet.

---

## Phase-by-phase breakdown

Each phase below carries: status, goal, deliverables (completion criteria), what's done, what's needed to close it.

### Phase 0 — Mental Model (Week 1) — COMPLETE

- **Goal:** vocabulary before code.
- **Deliverable:** `wiki/concepts/llm-basics.md` in own words (not copy-paste).
- **Done:** roadmap v2 filed; llm-basics glossary written; LinkedIn post #0 published.
- **To complete:** nothing. Closed 2026-06-18.

### Phase 1 — Hands on the API, Go-style (Weeks 2–3) — IN PROGRESS

- **Goal:** confidently call the Claude API from Go, stream responses, extract structured data.
- **Stack:** `net/http` + `encoding/json` + Go 1.22+ (compare with `liushuangls/go-anthropic` community SDK).
- **Deliverables (completion criteria):**
  1. Repo `go-llm-tools/` with `go.mod`.
  2. Three CLI commands:
     - `cmd/summarize/` — stdin → Claude → streamed summary to stdout.
     - `cmd/extract/` — free-form text → structured data via tool use.
     - `cmd/spec-to-code/` — NL feature description → structured code plan (files, functions, types).
  3. `StreamClient` struct (SSE parsing via `bufio.Scanner`, `data:` prefix).
  4. Retry wrapper with exponential backoff + `context.WithTimeout` (handles 429/529/400).
  5. Structured outputs via forced `tool_choice` (NOT JSON-mode prompting).
- **Topics to cover:** chat completions shapes, streaming SSE, prompt caching (cache_control breakpoints), structured outputs via tool use, retries/errors, pragmatic prompt engineering.
- **Done:** kicked off implementation in external project.
- **To complete:** finish the 3 CLIs, wire StreamClient + retry, then publish LinkedIn post #1 ("Building an LLM client in Go — what the official SDKs hide").
- **Guides:** `wiki/phase-1-playbook-go-llm.md`, `wiki/phase-1-guide.md`, `wiki/phase-1-guide-continued.md`, `wiki/phase-1-guide-final.md`, `wiki/concepts/streaming-llm.md`, `wiki/concepts/structured-outputs.md`.

### Phase 2 — RAG First (Weeks 4–6) — COMPLETE

- **Goal:** working RAG pipeline over personal PDFs that retrieves the right thing, proven with numbers.
- **Deliverables:**
  1. `go-rag-lab/` repo. *(Implemented in-repo as `internal/rag/` + `cmd/rag/` per the Phase 2 OpenSpec proposal/design; monorepo decision supersedes the separate-repo plan.)*
  2. Embeddings client (Voyage API, ~40 lines; `voyage-3-large` default). — `internal/rag/embeddings/`
  3. pgvector via `pgx` + `pgvector-go` (schema: id, content, embedding vector(1024), metadata jsonb, source_path). — `internal/rag/store/`
  4. Chunking: recursive char split → then contextual chunking (Anthropic 2024 — prepend 1-sentence doc summary per chunk). — `internal/rag/chunk/`
  5. Hybrid search: BM25 (`blevesearch`) + vector → RRF merge. — `internal/rag/search/`
  6. Reranking: Cohere Rerank API. — `internal/rag/rerank/`
  7. Eval suite (no framework): `.jsonl` of 20–30 `{query, expected_doc_id}`; metrics precision@k, recall@k, MRR; LLM-as-judge. — `internal/rag/eval/` + `../tests/evals/golden.jsonl` (20 examples)
  8. CLIs: `rag query "pergunta"`, `rag eval`. — `cmd/rag/{main,ingest,query,eval}.go`
- **Anti-patterns to internalize:** RAG when model already knows; fixed-512-token chunking; no reranker; no evals.
- **Done:** all 8 deliverables shipped; unit tests + live e2e verified against a local pgvector container with real Voyage/OpenRouter/Cohere keys (`go build`/`go vet`/`go test ./...` green; `openspec validate --changes phase-2-rag` clean). Sample run: ingest stored 4 chunks; `query "what was the total income in March 2025?"` returned a grounded, source-cited answer; `eval -judge` reported `avg_precision_at_k 0.2`, `avg_recall_at_k 1`, `avg_mrr 0.75`, `avg_judge_score 5`.
- **To complete:** publish LinkedIn post #2 ("RAG in Go — why your chunks matter more than your vector DB").
- **Concepts to write:** embeddings, chunking, vector-db, reranking, rag-evals.

### Phase 3 — Tool Use, Agents & Micro-Framework (Weeks 7–9) — NOT STARTED

- **Goal:** `agent_loop.go` (~150 lines) — reusable Go package orchestrating tool-calling loops; ship the "slash command calling your own MCP server" milestone.
- **Deliverables:**
  1. `pkg/agent/` — `Loop` struct with `Run(ctx, query) (string, []Turn, error)`, max turns (hard limit ~10), retry/backoff, streaming via channel.
  2. Three test tools: `get_weather`, `search_wiki`, `calculate`.
  3. Three agent patterns as wrappers around `agent.Loop`: ReAct, Plan-and-execute, Reflection.
  4. MCP server `cmd/wiki-mcp/` (stdio transport, `mark3labs/mcp-go` or hand-rolled JSON-RPC ~200 lines) exposing `wiki/` as resource + `search_notes` tool.
  5. Claude Code slash command `/wiki-search` calling the MCP server. Registered in `.claude/mcp.json`.
- **To complete:** package + patterns + MCP server + slash command working end-to-end; publish LinkedIn post #3 ("I built an agent loop in Go — 150 lines, zero frameworks").
- **Concepts to write:** tool-use, agents, mcp.

### Phase 4 — Model Roulette (Week 10) — NOT STARTED

- **Goal:** develop model-picking intuition; benchmark same prompt across 3+ providers.
- **Deliverables:**
  1. `LLMClient` interface (`Chat`, `ChatStream`) + 3 implementations: `ClaudeClient`, `OpenAIClient`, `GeminiClient` (each ~80 lines, all `net/http`).
  2. `cmd/model-roulette/` benchmark: 10 prompts (classification, summarization, structured extraction, creative, code, reasoning, translation, RAG response, tool selection, refusal boundary).
  3. CSV output: prompt_id, model, latency_ms, prompt_tokens, completion_tokens, cost_usd, quality_1_to_5.
  4. `wiki/concepts/model-selection.md` — permanent model-picker cheat sheet (refresh every 3 months).
- **To complete:** interface + 3 impls + CSV + concept page; publish LinkedIn post #4 ("I ran the same 10 prompts on Claude, GPT-4o, and Gemini — here's what surprised me").

### Phase 5 — Production & LLMOps (Weeks 11–12) — NOT STARTED

- **Goal:** instrument RAG lab + agent loop with traces, evals, cost dashboard. Close the POC→product gap.
- **Deliverables:**
  1. `pkg/evals/` — unit evals (deterministic) + behavioral evals (LLM-as-judge with rubric) + golden dataset (30 Q/A pairs) + `cmd/eval/` runner integrated into `go test`.
  2. `pkg/observability/` — Langfuse self-hosted (Docker Compose, NOT EKS); instrument `LLMClient` wrapper with traces (tokens, latency, cost, model).
  3. `cmd/cost/` — reads Langfuse traces, prints `$ spent this week by model/feature`.
  4. Cost discipline: response cache by `sha256(prompt+model+params)`, prompt compression (Haiku-summarize before Sonnet), model routing table (classification→Haiku, extraction→Sonnet, reasoning→Sonnet/Opus).
  5. Security light: prompt-injection stripping, PII masking in logs (`slog.LogAttrs`), regex-based banned-output detection.
  6. Async/queues for long agents: `202 Accepted` + `task_id`, poll `GET /tasks/:id`.
  7. `wiki/concepts/llmops.md`.
- **To complete:** packages + Langfuse dashboard + cost CLI; publish LinkedIn post #5 ("LLMOps for a solo project — what's worth it and what's overkill").
- **Concepts to write:** evals-llm, llmops, prompt-injection.

### Phase 6 — Multimodal & Document AI (Week 13) — NOT STARTED

- **Goal:** Go module that ingests a credit-card PDF and emits clean accounting JSON. Core data pipeline for the showcase.
- **Deliverables:**
  1. `pkg/document/` with `ExtractStatement(pdf []byte) (Statement, error)` + test suite with real redacted PDFs.
  2. Test 3 strategies: Claude vision directly on PDF (base64); OCR pipeline (`pdfplumber` via `os/exec` — the only Python dep in the roadmap); hybrid (OCR text + Claude vision for tables).
  3. Go structs: `Transaction` (date, description, amount, category), `Statement` (bank, period, transactions).
  4. Tool-use with `input_schema` matching `Statement` → guaranteed valid JSON; validate with `go-playground/validator`; dedup by `sha256(file bytes)`.
  5. Real-world test: 3 months Nubank + 2 months Itaú statements.
- **To complete:** module + tests across both bank formats; publish LinkedIn post #6 ("PDF extraction with LLMs — when OCR beats vision").
- **Concepts to write:** document-ai.

### Phase 7 — AI-Native UX Patterns (Week 14) — NOT STARTED

- **Goal:** learn/prototype the UX patterns that separate toy from product when the model is unpredictable.
- **Deliverables:**
  1. `ux-lab/` — static HTML/CSS prototype (not hooked to real LLM) demonstrating 7 patterns:
     - Progressive disclosure (stream text first, reveal tool calls when complete, sources as footnotes).
     - Human-in-the-loop (confirmation card for high-stakes actions).
     - Confirmation gates (pause before destructive tool calls).
     - Streaming actions (show "Searching transactions..." while tool runs).
     - Graceful degradation (useful fallback, not red error box).
     - Citation/provenance (clickable source links).
     - Cancellation + retry (`context.Cancel` → abort LLM, show "Response cancelled", allow retry with edited prompt).
  2. Anti-patterns documented: spinner with no progress; "I can't do that" with no alternative; hallucinated buttons; raw JSON in chat.
- **To complete:** prototype covering all 7 patterns; publish LinkedIn post #7 ("7 UX patterns that make or break an AI product").
- **Concepts to write:** ai-ux-patterns.

### Phase 8 — The Showcase: Finanças IA (Weeks 15–18) — NOT STARTED

- **Goal:** ship working personal finance assistant — Go backend + chat UI + evals. Finances only (not calendar/reminders/notes).
- **Architecture:** Browser (fetch + SSE) → Go API (`chi` or `net/http` ServeMux: `POST /chat`, `POST /ingest`, `GET /tasks/:id`, `GET /health`) → Postgres + pgvector (statements, chunks, embeddings) → Claude (Sonnet queries, Haiku classification), Voyage (embeddings), Cohere (reranking). Backend also exposes an MCP server so Claude Code queries finances.
- **Deliverables:**
  1. Repo `financas-ia/` reusing `pkg/agent/`, `pkg/document/`, `pkg/evals/`, `pkg/observability/`.
  2. Backend: `chi`/ServeMux, `pgx` pool, MCP server layer, graceful shutdown (`signal.NotifyContext`).
  3. Frontend: chat UI + SSE streaming, drag-and-drop PDF upload, pending-actions indicator, confirmation card, optional cost badge.
  4. Evals: 30 intents (e.g., "quanto gastei com mercado em abril?" → `search_transactions`, category `mercado`), run via `go test ./...`.
  5. Deployment: multi-stage `Dockerfile` (scratch ~10MB), `docker-compose.yml` (app + postgres + langfuse + redis), `/health` checks DB + Claude. Cloud deploy optional — milestone is "runs on `docker compose up`."
  6. `README.md` with architecture diagram, setup, eval results. 1-min Loom demo.
- **Explicitly out of scope (future extensions):** calendar, reminders, notes RAG, mobile, multi-user/auth, fine-tuning.
- **To complete:** full project shipped + README + demo video; publish LinkedIn post #8 ("I built a personal finance assistant in Go + Claude — here's the architecture").
- **Concepts to write:** personal-assistant-architecture.

---

## Parallel track — Claude Code / Cursor Mastery (~2h/week)

Runs alongside the main schedule, reordered for maximum dogfooding. Not a phase — an ongoing thread.

1. AGENTS.md / CLAUDE.md (Week 1) — workspace-level instructions. **Done** (this wiki is the example).
2. Memory (Week 1) — `~/.claude/CLAUDE.md` cross-session persistence.
3. Custom slash commands (Week 2) — add as needed while building `go-llm-tools/`.
4. MCP servers (Week 8) — connect Claude Code to wiki MCP. **Milestone:** first slash command calling own MCP server.
5. Hooks (Week 3) — lint/format on save via `.claude/settings.json`.
6. Skills (Week 4) — Go project scaffolding skill.
7. Subagents (Week 6) — `go-reviewer` subagent.
8. Plan mode (Week 7) — `/plan` before `agent_loop.go`.
9. Output styles + statusline (Week 5) — QoL.
10. Cursor (ongoing) — `.cursor/rules/*.mdc`, agent mode, MCP.

---

## LinkedIn post calendar (9 posts)

| # | Phase | When | Topic |
|---|---|---|---|
| 0 | Phase 0 | Week 1 (before starting) | "I'm doing AI Engineer in public — 18 weeks, Go, shipping a real app" — **POSTED 2026-05-31** |
| 1 | Phase 1 | End of Week 3 | "Building an LLM client in Go — what the official SDKs hide" |
| 2 | Phase 2 | End of Week 6 | "RAG in Go — why your chunks matter more than your vector DB" |
| 3 | Phase 3 | End of Week 9 | "I built an agent loop in Go — 150 lines, zero frameworks" |
| 4 | Phase 4 | End of Week 10 | "10 prompts on 3 models — what surprised me" |
| 5 | Phase 5 | End of Week 12 | "LLMOps for a solo project — what's worth it and what's overkill" |
| 6 | Phase 6 | End of Week 13 | "PDF extraction with LLMs — when OCR beats vision" |
| 7 | Phase 7 | End of Week 14 | "7 UX patterns that make or break an AI product" |
| 8 | Phase 8 | End of Week 18 | "I shipped Finanças IA — Go + Claude + pgvector" |

Post rules: within 3 days of phase completion; code snippets > screenshots > text walls; link public repo when it exists; never say "I'm learning" — say "I built X. Here's what happened."; respond to technical comments.

---

## Curated resources (already vetted)

- **Books:** *AI Engineering* (Chip Huyen, 2025) ch 1–5, 7–9; *Building LLMs for Production* (Bouchard & Peysakhovich) ch 3, 5, 7.
- **Courses (DeepLearning.AI, free):** Building Systems with the ChatGPT API; Building Evaluating Advanced RAG; Functions Tools Agents with LangChain.
- **Newsletters/blogs:** Latent Space (swyx), Simon Willison, Anthropic blog, Chip Huyen.
- **Code:** `anthropic-cookbook` on GitHub (translate Python → Go as exercise).
- **Go libraries:** `liushuangls/go-anthropic`, `pgx` + `pgvector-go`, `mark3labs/mcp-go`, `langfuse-go`, `blevesearch`, `cenkalti/backoff`, `go-playground/validator`.

---

## How an AI agent should use this file

1. **To know what to do next:** read "Where I am right now" + the in-progress phase's "To complete" line. That's the next action.
2. **To know if a phase is done:** check the phase's status marker (COMPLETE / IN PROGRESS / NOT STARTED) and the deliverables list — all deliverables must be shipped.
3. **To avoid scope creep:** every phase has an explicit deliverables list. Don't add work. Don't skip the eval suite in Phase 2/5/8.
4. **To draft LinkedIn posts:** each phase has an angle in the post calendar. Post within 3 days of phase completion.
5. **To track progress:** the source of truth is `wiki/journey-ai-engineer.md` (append-only diary). This file is a snapshot — if it disagrees with the journey page, the journey page wins.
6. **Language rule:** the user is Go-first. Do not suggest Python/TS unless the phase explicitly calls for it (Phase 6 OCR is the only Python touch).
7. **Dogfooding rule:** use Claude Code / OpenCode to build every deliverable. Every time you hit friction, improve config — don't just tolerate it.

---

## Source files (canonical)

- `wiki/roadmap-ai-engineer-v2.md` — overview, north star, resources, LinkedIn calendar.
- `wiki/roadmap-ai-engineer-v2-phases-0-3.md` — Phases 0–3 detail.
- `wiki/roadmap-ai-engineer-v2-phases-4-6.md` — Phases 4–6 detail.
- `wiki/roadmap-ai-engineer-v2-phases-7-8.md` — Phases 7–8 detail.
- `wiki/journey-ai-engineer.md` — live diary (source of truth for status).
- `wiki/journey-ai-engineer-linkedin.md` — LinkedIn post pool and calendar.
- `wiki/phase-1-playbook-go-llm.md` + `wiki/phase-1-guide*.md` — Phase 1 execution guides.
