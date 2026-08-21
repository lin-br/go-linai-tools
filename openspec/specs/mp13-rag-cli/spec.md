# Purpose

TBD

# Requirements

## Requirement: RAG CLI entry point exists at cmd/rag/main.go

The system SHALL provide a CLI binary at `cmd/rag/main.go` that dispatches to subcommands `ingest`, `query`, and `eval`. The CLI SHALL use the standard `flag` package (no Cobra) and exit with code `0` on success and code `1` on any error. Diagnostic output SHALL go to `stderr`; command output (JSON, plain text, eval report) SHALL go to `stdout`.

### Scenario: No subcommand prints usage

- **WHEN** the user runs `go run ./cmd/rag`
- **THEN** it SHALL print usage text to `stderr` and exit with code `1`

### Scenario: Unknown subcommand prints error

- **WHEN** the user runs `go run ./cmd/rag unknown`
- **THEN** it SHALL print `"unknown subcommand: unknown"` to `stderr` and exit with code `1`

## Requirement: ingest subcommand loads a text file, chunks, embeds, and stores

The system SHALL implement `rag ingest <file>` with the following behavior:
1. Read the file at the given path into memory.
2. Chunk it using the configured chunker (recursive by default, contextual when `-contextual` is true).
3. Embed all chunks via `embeddings.Client`.
4. Insert the chunks into `store.Store`.

Flags: `-db` (Postgres DSN, env `POSTGRES_DSN`), `-chunk-size` (int, default `chunk.DefaultChunkSize`), `-chunk-overlap` (int, default `chunk.DefaultChunkOverlap`), `-contextual` (bool, default `false` to use the chat provider for summary), `-model` (chat model for contextual summary, resolved from config).

### Scenario: Ingest a text file

- **WHEN** the user runs `go run ./cmd/rag ingest notes.txt`
- **THEN** the file SHALL be read, chunked, embedded, and stored, and a summary line `"stored N chunks from notes.txt"` SHALL be printed to `stdout`

### Scenario: Contextual chunking flag

- **WHEN** the user runs `go run ./cmd/rag ingest notes.txt -contextual`
- **THEN** the system SHALL use `chunk.ContextualChunker` with the configured chat provider to generate per-document summaries

### Scenario: Missing file returns error

- **WHEN** the user runs `go run ./cmd/rag ingest missing.txt`
- **THEN** it SHALL print an error to `stderr` and exit with code `1`

## Requirement: query subcommand answers a question using hybrid search and reranking

The system SHALL implement `rag query "<question>"` with the following behavior:
1. Embed the query with `embeddings.Client`.
2. Load all chunks from `store.Store` and rebuild the `BM25Searcher` index.
3. Run `HybridSearcher.Search(ctx, question, queryVec, topK*4)` to retrieve candidates.
4. Rerank candidates with `rerank.Client.Rerank`, taking `topN = topK`.
5. Build a chat request with a system prompt instructing the model to answer using only the provided context and cite the source paths.
6. Call `outbound.Provider.Chat` and print the assistant's content to `stdout`.

Flags: `-db`, `-top-k` (int, default `5`), `-model` (chat model, default from config), `-rerank` (bool, default `true`; when false, skip reranking), `-contextual` (bool, default `false`; when true, expect summary-prefixed chunks from contextual chunking).

### Scenario: Query returns an answer

- **WHEN** the user runs `go run ./cmd/rag query "what was the budget?"`
- **THEN** the system SHALL print a plain-text answer to `stdout`

### Scenario: Reranker can be disabled

- **WHEN** the user runs `go run ./cmd/rag query "what was the budget?" -rerank=false`
- **THEN** the system SHALL skip Cohere reranking and use the hybrid fused ordering directly

### Scenario: Empty corpus returns helpful error

- **WHEN** the user runs `go run ./cmd/rag query "anything"` on an empty database
- **THEN** it SHALL print `"no chunks found; run 'rag ingest <file>' first"` to `stderr` and exit with code `1`

## Requirement: eval subcommand runs the golden dataset and prints metrics

The system SHALL implement `rag eval -dataset <path>` with the following behavior:
1. Load the dataset with `eval.LoadDataset`.
2. Embed each query with `embeddings.Client`.
3. For each example, run the full pipeline (hybrid search + optional rerank) and collect `search.Result.ID` values.
4. Compute `precision@k`, `recall@k`, and `MRR` against `example.ExpectedChunkID`.
5. Optionally call the LLM judge if `-judge` is true.
6. Print the `eval.Report` as indented JSON to `stdout`.

Flags: `-db`, `-dataset` (string, default `"testdata/golden.jsonl"`), `-top-k` (int, default `5`), `-judge` (bool, default `false`), `-model` (chat model for judge, default from config).

### Scenario: Eval prints JSON report

- **WHEN** the user runs `go run ./cmd/rag eval -dataset testdata/golden.jsonl`
- **THEN** it SHALL print a JSON object with `avg_precision_at_k`, `avg_recall_at_k`, `avg_mrr`, and `examples` to `stdout`

### Scenario: Judge flag enables LLM scoring

- **WHEN** the user runs `go run ./cmd/rag eval -dataset testdata/golden.jsonl -judge`
- **THEN** the report SHALL include an `avg_judge_score` field

## Requirement: CLI wiring loads configuration and API keys

The system SHALL load `internal/configs/configs.yaml` via `configs.LoadConfigs()` and read the following env vars: `OPENROUTER_API_KEY`, `VOYAGE_API_KEY`, `COHERE_API_KEY`, and `POSTGRES_DSN`. Missing required keys SHALL cause the CLI to print a clear error to `stderr` and exit with code `1`. Library code (`internal/rag/...`) SHALL NOT call `log.Fatal`.

### Scenario: Missing Voyage key aborts ingest/query/eval

- **WHEN** `VOYAGE_API_KEY` is unset and the user runs `rag ingest notes.txt`
- **THEN** the CLI SHALL print `"VOYAGE_API_KEY is required"` to `stderr` and exit with code `1`

### Scenario: Missing Postgres DSN aborts ingest/query/eval

- **WHEN** `POSTGRES_DSN` is unset
- **THEN** the CLI SHALL print `"POSTGRES_DSN is required"` to `stderr` and exit with code `1`

## Requirement: Signal context propagates cancellation

The system SHALL create a root context via `signal.NotifyContext(context.Background(), os.Interrupt)` and pass it through all subcommands so that Ctrl+C cancels in-flight HTTP requests and database queries.

### Scenario: Ctrl+C cancels query

- **WHEN** the user presses Ctrl+C while a query is waiting for the chat provider
- **THEN** the context SHALL be cancelled, the request SHALL abort, and the CLI SHALL exit without printing a partial answer

## Requirement: No external CLI framework

The system SHALL implement subcommand dispatch manually using `os.Args` and the `flag` package. The CLI SHALL NOT import `github.com/spf13/cobra`, `github.com/urfave/cli`, or similar frameworks.

### Scenario: Dependency graph excludes Cobra

- **WHEN** `go.mod` is inspected
- **THEN** it SHALL NOT contain `github.com/spf13/cobra`
