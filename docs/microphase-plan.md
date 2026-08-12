# Microphase Plan — Phases 2-8

> One-file breakdown of every microphase needed to complete the AI Engineer roadmap (v2) inside the existing `go-linai-tools` monorepo. Each microphase is sized to be a single study unit and a single implementation milestone.

---

## Conventions used in this plan

- **Provider interface**: the existing `outbound.Provider` (`Chat`, `ChatStream`) is the generic boundary. OpenRouter is the default implementation. Anthropic, OpenAI, Gemini, and AWS Bedrock providers are introduced in Phase 4.
- **Monorepo layout**: all code lives in `go-linai-tools` using subpackages and `cmd/` binaries. No separate repos.
- **Go-first**: `net/http` preferred over SDKs. The only planned Python is Phase 6 OCR via `os/exec`.
- **OpenSpec change granularity**: one OpenSpec change per phase, with one spec file per microphase under `openspec/changes/<change>/specs/`.

---

## Phase 2 — RAG First

**Goal:** working RAG pipeline over personal PDFs that retrieves the right thing, proven with numbers.

| ID | Microphase | Deliverable | Depends on |
|---|---|---|---|
| MP7 | Embeddings client | `internal/rag/embeddings/` Voyage client (`voyage-3-large`), ~40 lines | MP0-MP3 |
| MP8 | Vector store + pgvector | `internal/rag/store/` with `pgx` + `pgvector-go`; schema `id, content, embedding vector(1024), metadata jsonb, source_path` | MP7 |
| MP9 | Document chunking | `internal/rag/chunk/` recursive char split + contextual chunking (1-sentence doc summary per chunk) | — |
| MP10 | Hybrid search | `internal/rag/search/` BM25 (`blevesearch`) + vector → RRF merge | MP8, MP9 |
| MP11 | Reranking | `internal/rag/rerank/` Cohere Rerank API wrapper | MP10 |
| MP12 | RAG eval suite | `internal/rag/eval/` golden dataset (20-30 pairs), metrics `precision@k`, `recall@k`, `MRR`, LLM-as-judge | MP11 |
| MP13 | RAG CLI | `cmd/rag/` with `rag ingest <file>`, `rag query "..."`, `rag eval` | MP12 |

---

## Phase 3 — Tool Use, Agents & Micro-Framework

**Goal:** reusable Go agent loop and the "slash command calling your own MCP server" milestone.

| ID | Microphase | Deliverable | Depends on |
|---|---|---|---|
| MP14 | Agent loop package | `internal/agent/loop.go` `Loop.Run(ctx, query)` with max turns, retry/backoff, streaming via channel | MP0-MP3 |
| MP15 | Test tools | `internal/agent/tools/` `get_weather`, `search_wiki`, `calculate` | MP14 |
| MP16 | Agent patterns | `internal/agent/patterns/` ReAct, Plan-and-Execute, Reflection wrappers | MP15 |
| MP17 | MCP server | `cmd/wiki-mcp/` stdio transport exposing `wiki/` as resource + `search_notes` tool | MP14 |
| MP18 | Slash command | `.claude/mcp.json` + `/wiki-search` command calling `wiki-mcp` | MP17 |

---

## Phase 4 — Model Roulette

**Goal:** develop model-picking intuition by benchmarking the same prompt across providers.

| ID | Microphase | Deliverable | Depends on |
|---|---|---|---|
| MP19 | Multi-provider interface + Anthropic | `internal/providers/` generic factory + `AnthropicProvider` (`net/http`, Messages API) | MP0 |
| MP20 | OpenAI/Gemini/Bedrock providers | `OpenAIProvider`, `GeminiProvider`, `BedrockProvider` | MP19 |
| MP21 | Benchmark runner | `cmd/model-roulette/` 10 prompts (classification, summarization, extraction, creative, code, reasoning, translation, RAG response, tool selection, refusal boundary) | MP20 |
| MP22 | CSV output + model cheat sheet | CSV `prompt_id, model, latency_ms, tokens, cost_usd, quality_1_to_5` + `docs/model-selection.md` | MP21 |

---

## Phase 5 — Production & LLMOps

**Goal:** instrument RAG + agent with traces, evals, cost dashboard, and close the POC→product gap.

| ID | Microphase | Deliverable | Depends on |
|---|---|---|---|
| MP23 | Evals package | `internal/evals/` unit + behavioral (LLM-as-judge) + golden dataset (30 Q/A) + `go test` runner | MP12 |
| MP24 | Observability / Langfuse | `internal/observability/` Langfuse self-hosted (Docker Compose), trace tokens/latency/cost/model | MP23 |
| MP25 | Cost CLI | `cmd/cost/` reads Langfuse traces, prints `$ spent this week by model/feature` | MP24 |
| MP26 | Cost discipline | Response cache (`sha256(prompt+model+params)`), prompt compression, model routing table | MP25 |
| MP27 | Security light | Prompt-injection stripping, PII masking in `slog`, banned-output regex detection | MP26 |
| MP28 | Async tasks | `202 Accepted` + `task_id`, poll `GET /tasks/:id` | MP14 |

---

## Phase 6 — Multimodal & Document AI

**Goal:** ingest a credit-card PDF and emit clean accounting JSON.

| ID | Microphase | Deliverable | Depends on |
|---|---|---|---|
| MP29 | Document package + structs | `internal/document/` `ExtractStatement(pdf []byte) (Statement, error)` + `Transaction`, `Statement` structs | MP3 |
| MP30 | PDF ingestion strategies | Claude vision on base64 PDF; OCR pipeline via `pdfplumber`/`os/exec`; hybrid (OCR + vision tables) | MP29 |
| MP31 | Extraction + validation | Tool-use `input_schema` matching `Statement`, `go-playground/validator`, dedup by `sha256(file)` | MP30 |
| MP32 | Real-world tests | 3 months Nubank + 2 months Itaú redacted statement tests | MP31 |

---

## Phase 7 — AI-Native UX Patterns

**Goal:** prototype UX patterns that separate toy from product.

| ID | Microphase | Deliverable | Depends on |
|---|---|---|---|
| MP33 | UX lab static prototype | `ux-lab/` static HTML/CSS prototype demonstrating 7 patterns (progressive disclosure, human-in-the-loop, confirmation gates, streaming actions, graceful degradation, citation/provenance, cancellation + retry) | — |

---

## Phase 8 — The Showcase: Finanças IA

**Goal:** ship working personal finance assistant — Go backend + chat UI + evals.

| ID | Microphase | Deliverable | Depends on |
|---|---|---|---|
| MP34 | Project structure + backend API | `cmd/financas-ia/` `chi` or `net/http` ServeMux, `POST /chat`, `POST /ingest`, `GET /health` | MP0-MP3, MP14 |
| MP35 | Database layer | `pgx` pool, migrations for statements/chunks/embeddings | MP34 |
| MP36 | Ingestion pipeline | PDF upload → document extraction → chunking → embeddings → pgvector | MP35, MP29 |
| MP37 | Chat backend + SSE | `POST /chat` with SSE streaming, agent tool calls for finance queries | MP36, MP16 |
| MP38 | Frontend chat UI | Static HTML/CSS/JS with drag-and-drop upload, pending-actions, confirmation card, cost badge | MP37 |
| MP39 | Evals + deployment | 30 intent evals via `go test`, multi-stage Dockerfile, `docker-compose.yml` (app + postgres + langfuse + redis), `/health` checks DB + Claude | MP38 |
| MP40 | README + demo | Architecture diagram, setup, eval results, 1-min Loom demo | MP39 |

---

## Dependency graph (high level)

```
Phase 1 (MP0-MP6)
  │
  ├── Phase 2 (MP7-MP13) ──► Phase 5 (MP23-MP28)
  │
  ├── Phase 3 (MP14-MP18) ──► Phase 5 / Phase 8
  │
  ├── Phase 4 (MP19-MP22)
  │
  ├── Phase 6 (MP29-MP32) ──► Phase 8
  │
  ├── Phase 7 (MP33)
  │
  └── Phase 8 (MP34-MP40)
```

---

## OpenSpec mapping

| OpenSpec change | Phase | Specs inside change |
|---|---|---|
| `phase-2-rag` | 2 | `mp7-embeddings-client`, `mp8-vector-store`, `mp9-chunking`, `mp10-hybrid-search`, `mp11-reranking`, `mp12-rag-eval`, `mp13-rag-cli` |
| `phase-3-agents` | 3 | `mp14-agent-loop`, `mp15-test-tools`, `mp16-agent-patterns`, `mp17-mcp-server`, `mp18-slash-command` |
| `phase-4-model-roulette` | 4 | `mp19-multi-provider`, `mp20-provider-implementations`, `mp21-benchmark-runner`, `mp22-csv-cheat-sheet` |
| `phase-5-production` | 5 | `mp23-evals-package`, `mp24-observability`, `mp25-cost-cli`, `mp26-cost-discipline`, `mp27-security-light`, `mp28-async-tasks` |
| `phase-6-document-ai` | 6 | `mp29-document-structs`, `mp30-pdf-strategies`, `mp31-extraction-validation`, `mp32-real-world-tests` |
| `phase-7-ux-patterns` | 7 | `mp33-ux-lab-prototype` |
| `phase-8-financas-ia` | 8 | `mp34-backend-api`, `mp35-database-layer`, `mp36-ingestion-pipeline`, `mp37-chat-sse`, `mp38-frontend-ui`, `mp39-evals-deployment`, `mp40-readme-demo` |
