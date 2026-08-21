# rag

RAG CLI for the go-linai-tools project (Phase 2). Ingests text files into a
pgvector-backed store, answers questions with hybrid search (BM25 + vector +
RRF) and optional Cohere reranking, and evaluates retrieval quality against a
golden dataset.

## Subcommands

```
rag ingest <file>                       Chunk, embed, and store a text file
rag query "<question>"                  Answer a question from the stored corpus
rag eval -dataset <path>                Run the golden dataset and print metrics JSON
```

## Setup

### 1. Environment variables

```bash
export OPENROUTER_API_KEY=...   # chat provider (query answer + contextual chunking + judge)
export VOYAGE_API_KEY=...       # embeddings (voyage-3-large, 1024 dims)
export COHERE_API_KEY=...       # reranking (optional for query -rerank=false; optional for eval)
export POSTGRES_DSN=...         # e.g. postgres://user:pass@localhost:5432/rag?sslmode=disable
export DEFAULT_MODEL=anthropic/claude-sonnet-4-20250514
export DEFAULT_EMBEDDING_MODEL=voyage-3-large
export DEFAULT_RERANK_MODEL=rerank-v3.5
```

`POSTGRES_DSN` and `VOYAGE_API_KEY` are required for every subcommand. Missing
keys are reported to stderr with exit code 1.

### 2. Postgres with pgvector (Docker)

```bash
docker run -d --name rag-pg \
  -e POSTGRES_PASSWORD=secret \
  -e POSTGRES_DB=rag \
  -p 5432:5432 \
  pgvector/pgvector:pg16

export POSTGRES_DSN="postgres://postgres:secret@localhost:5432/rag?sslmode=disable"
```

The schema (`CREATE EXTENSION vector; CREATE TABLE chunks (...)`) is applied
automatically on the first `ingest`/`query`/`eval` via `store.InitSchema`. See
`internal/rag/store/schema.sql` for the exact DDL.

## Usage examples

Run from the `cmd/rag` directory so the relative `testdata/` paths resolve
(equivalently, run from the repo root with explicit `cmd/rag/testdata/...` paths).

```bash
cd cmd/rag

# Ingest a text file (recursive chunking, 512 runes, 50 overlap)
go run . ingest testdata/sample.txt
# stored 12 chunks from testdata/sample.txt

# Contextual chunking (LLM one-sentence summary prepended to each chunk)
go run . ingest testdata/sample.txt -contextual

# Ask a question
go run . query "what was the budget?"
# Prints a plain-text answer citing source paths.

# Disable reranking
go run . query "what was the budget?" -rerank=false

# Evaluate retrieval quality
go run . eval -dataset testdata/golden.jsonl
# Prints JSON with avg_precision_at_k, avg_recall_at_k, avg_mrr, examples.

# Evaluate with LLM-as-judge scoring
go run . eval -dataset testdata/golden.jsonl -judge
# Adds avg_judge_score and per-example judge_score.
```

## Flags

| Subcommand | Flag           | Default                    | Description                          |
|------------|----------------|----------------------------|--------------------------------------|
| ingest     | `-db`          | `$POSTGRES_DSN`            | Postgres DSN                         |
| ingest     | `-chunk-size`  | `512`                      | Chunk size in runes                  |
| ingest     | `-chunk-overlap`| `50`                      | Chunk overlap in runes               |
| ingest     | `-contextual`  | `false`                    | Use contextual chunking              |
| ingest     | `-model`       | config default             | Chat model for contextual summary    |
| query      | `-db`          | `$POSTGRES_DSN`            | Postgres DSN                         |
| query      | `-top-k`       | `5`                        | Top-k chunks to answer from          |
| query      | `-model`       | config default             | Chat model for the answer            |
| query      | `-rerank`      | `true`                     | Enable Cohere reranking              |
| query      | `-contextual`  | `false`                    | Expect context-prefixed chunks       |
| eval       | `-db`          | `$POSTGRES_DSN`            | Postgres DSN                         |
| eval       | `-dataset`     | `../../tests/evals/golden.jsonl`    | Golden dataset path                  |
| eval       | `-top-k`       | `5`                        | Top-k for retrieval metrics          |
| eval       | `-judge`       | `false`                    | Enable LLM-as-judge scoring          |
| eval       | `-model`       | config default             | Chat model for the judge             |

## Exit codes

- `0` on success.
- `1` on any error; the error message is written to stderr. Diagnostic output
  goes to stderr; command output (answer text, eval JSON) goes to stdout.

## Notes

- BM25 index is rebuilt in-memory per process from the pgvector rows; there is
  no persistent keyword index.
- `query` uses non-streaming `Provider.Chat`; streaming is covered in Phase 1.
- PDF parsing is Phase 6; `ingest` accepts text files only.
