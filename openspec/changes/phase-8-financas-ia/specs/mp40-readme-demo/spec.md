## ADDED Requirements

### Requirement: README includes architecture diagram

The system SHALL provide a `README.md` at the repository root or under `docs/financas-ia/README.md` that includes a Mermaid or ASCII architecture diagram showing: Browser → Go HTTP API → Postgres + pgvector, Claude, Voyage, Cohere; async ingestion worker; Langfuse traces; Redis task state. The diagram SHALL be embedded in markdown so it renders on GitHub.

#### Scenario: README renders on GitHub

- **WHEN** the README is viewed on github.com
- **THEN** the architecture diagram SHALL render as a Mermaid diagram or a readable ASCII block

#### Scenario: Diagram shows data flow

- **WHEN** reading the diagram
- **THEN** it SHALL show the path from PDF upload through extraction/chunking/embedding to chat query retrieval

### Requirement: README documents local setup

The README SHALL contain a "Setup" section with step-by-step instructions to run locally: clone, copy `.env.example` to `.env`, set required environment variables (`DATABASE_URL`, `OPENROUTER_API_KEY`, `VOYAGE_API_KEY`, `COHERE_API_KEY`, `LANGFUSE_*`), run `docker compose up`, and access `http://localhost:8080`.

#### Scenario: New user follows setup

- **WHEN** a user follows the README setup steps on a machine with Docker installed
- **THEN** the application SHALL be reachable at `http://localhost:8080` after `docker compose up -d`

#### Scenario: Environment variables listed

- **WHEN** reading the setup section
- **THEN** the user SHALL see a table of required and optional environment variables with descriptions

### Requirement: README includes usage examples

The README SHALL provide usage examples: a sample chat question, a curl example for `POST /ingest`, and expected JSON responses for `GET /tasks/{id}` and `GET /health`.

#### Scenario: curl example for ingestion

- **WHEN** the user copies the `POST /ingest` curl command
- **THEN** it SHALL work against the running local server and return a `task_id`

#### Scenario: Chat example in Portuguese

- **WHEN** the user follows the example question
- **THEN** the README SHALL show the expected Portuguese answer pattern (e.g., "Você gastou R$ X com mercado em abril")

### Requirement: README documents eval results

The README SHALL contain an "Eval Results" section summarizing the 30 intent evals: pass count, fail count, and a representative subset of queries with their expected tool and actual outcome. Results SHALL be updated manually after each eval run.

#### Scenario: Eval summary present

- **WHEN** reading the README
- **THEN** the user SHALL see "30/30 intent evals passing" (or current result) and a link to the eval test file

#### Scenario: Representative queries listed

- **WHEN** reading the eval section
- **THEN** the user SHALL see at least 5 example queries such as "quanto gastei com mercado em abril?" with expected/actual tool calls

### Requirement: README links to 1-minute Loom demo

The README SHALL include a link to a 1-minute Loom demo video (hosted on Loom or embedded GIF) demonstrating: uploading a Nubank PDF, asking a spending question, observing SSE streaming, pending actions, and the cost badge.

#### Scenario: Demo link is accessible

- **WHEN** clicking the Loom link
- **THEN** the viewer SHALL see a 1-minute demo video of Finanças IA

#### Scenario: Demo covers all key features

- **WHEN** watching the demo
- **THEN** it SHALL show upload, ingestion progress, chat, SSE, tool-call indicator, and cost badge

### Requirement: README documents tech stack and conventions

The README SHALL list the tech stack: Go 1.26.4, `net/http`, `pgx`, `pgvector`, Voyage, Cohere, Anthropic via OpenRouter, Langfuse, Redis, vanilla HTML/CSS/JS. It SHALL also note the project's Go conventions (`context.Context` first param, `slog`, table-driven tests, no Python runtime).

#### Scenario: Tech stack visible

- **WHEN** reading the README
- **THEN** the user SHALL see the list of languages, libraries, and external services

### Requirement: README includes architecture decisions summary

The README SHALL include a short "Architecture Decisions" or "Why this stack" section summarizing key decisions: `net/http` over `chi`, `pgx` over ORM, SSE over WebSockets, async ingestion, and Langfuse for observability.

#### Scenario: Decisions are justified

- **WHEN** reading the decisions section
- **THEN** the user SHALL understand why each technology was chosen

### Requirement: README links to LinkedIn post

The README SHALL link to the Phase 8 LinkedIn post #8 as documented in `docs/roadmap-ai-engineer-status.md`, titled "I shipped Finanças IA — Go + Claude + pgvector".

#### Scenario: LinkedIn post linked

- **WHEN** the project is published
- **THEN** the README SHALL contain a link to the Phase 8 LinkedIn post
