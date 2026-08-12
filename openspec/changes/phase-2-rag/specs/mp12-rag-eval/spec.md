## ADDED Requirements

### Requirement: RAG eval package exists under internal/rag/eval

The system SHALL provide a package `internal/rag/eval` that exports functions and types to evaluate retrieval and generation quality. The package SHALL support loading a golden dataset from a `.jsonl` file, computing deterministic retrieval metrics (`precision@k`, `recall@k`, `MRR`), and running LLM-as-judge scoring via the existing `outbound.Provider`.

#### Scenario: Package exposes a Dataset type

- **WHEN** code declares a value of type `eval.Dataset`
- **THEN** it SHALL hold a slice of `eval.Example` values

#### Scenario: Example struct has required fields

- **WHEN** an `eval.Example` is created
- **THEN** it SHALL expose `Query string`, `ExpectedChunkID uuid.UUID`, and `ExpectedAnswer string` fields with appropriate JSON tags

### Requirement: Golden dataset loads from JSONL

The system SHALL implement `eval.LoadDataset(path string) (*Dataset, error)` that reads one JSON object per line. Each line SHALL deserialize into an `eval.Example`. The file SHALL be allowed to contain an optional `expected_chunk_id` string field that is parsed as a UUID.

#### Scenario: Load 20-example dataset

- **WHEN** `eval.LoadDataset("testdata/golden.jsonl")` is called with a file containing 20 valid JSON lines
- **THEN** the returned `Dataset` SHALL contain exactly 20 examples

#### Scenario: Missing file returns error

- **WHEN** `eval.LoadDataset("missing.jsonl")` is called
- **THEN** it SHALL return a filesystem error

#### Scenario: Invalid JSON line returns error

- **WHEN** the file contains a line that is not valid JSON
- **THEN** `LoadDataset` SHALL return an error indicating the line number and failure

### Requirement: Metrics functions compute precision, recall, and MRR

The system SHALL implement:
- `eval.PrecisionAtK(relevant []uuid.UUID, retrieved []uuid.UUID, k int) float64`
- `eval.RecallAtK(relevant []uuid.UUID, retrieved []uuid.UUID, k int) float64`
- `eval.MRR(relevant []uuid.UUID, retrieved []uuid.UUID) float64`

`relevant` SHALL be treated as a set. `PrecisionAtK` SHALL equal `|relevant ∩ retrieved[:k]| / k`. `RecallAtK` SHALL equal `|relevant ∩ retrieved[:k]| / |relevant|` (returning `0` if `|relevant| == 0`). `MRR` SHALL equal `1 / rank` where `rank` is the 1-based position of the first relevant result, or `0` if none are relevant.

#### Scenario: Precision at 3 with one relevant in top 3

- **WHEN** `PrecisionAtK(relevant, retrieved, 3)` is called with one relevant item in the first three retrieved results
- **THEN** it SHALL return `1.0/3.0`

#### Scenario: Recall at 3 with two relevant total and one retrieved

- **WHEN** `RecallAtK(relevant, retrieved, 3)` is called with `|relevant| == 2` and one of them in the top 3
- **THEN** it SHALL return `1.0/2.0`

#### Scenario: MRR for first-position relevant

- **WHEN** `MRR(relevant, retrieved)` is called and the first retrieved item is relevant
- **THEN** it SHALL return `1.0`

#### Scenario: MRR for no relevant results

- **WHEN** `MRR(relevant, retrieved)` is called and no retrieved item is relevant
- **THEN** it SHALL return `0.0`

### Requirement: Evaluator runs the full retrieval pipeline and reports metrics

The system SHALL implement `eval.NewEvaluator(searcher search.Searcher, judge *Judge, topK int) *Evaluator` with method `Run(ctx context.Context, dataset *Dataset) (*Report, error)`. The evaluator SHALL, for each example:
1. Call `searcher.Search(ctx, example.Query, nil, topK)` to retrieve candidate chunk IDs. (Vector/hybrid search without a precomputed query vector is invoked with nil vector for baseline retrieval; MP13 CLI supplies the query vector where available.)
2. Compute `precision@k`, `recall@k`, and `MRR` against `example.ExpectedChunkID`.
3. If `judge` is non-nil, call `judge.Score(ctx, example.Query, retrievedText, example.ExpectedAnswer)` and record the score.

The returned `Report` SHALL contain per-example results and aggregate averages.

#### Scenario: Evaluator produces averages

- **WHEN** `evaluator.Run(ctx, dataset)` completes over 3 examples
- **THEN** `Report.AvgPrecisionAtK`, `Report.AvgRecallAtK`, and `Report.AvgMRR` SHALL equal the arithmetic means of the per-example metrics

#### Scenario: Nil judge skips LLM scoring

- **WHEN** `evaluator.Run` is called with `judge == nil`
- **THEN** the report SHALL still include retrieval metrics and SHALL NOT call any LLM

### Requirement: LLM-as-judge scores answer relevance

The system SHALL implement `eval.NewJudge(provider outbound.Provider, model string) *Judge` with method `Score(ctx context.Context, query, generated, expected string) (int, error)`. The judge SHALL send a chat request via `provider.Chat` with a rubric asking the model to rate answer relevance on a 1–5 integer scale, where 5 means "fully answers the query and matches expected facts" and 1 means "completely irrelevant or wrong". The method SHALL parse the first integer found in the response and clamp it to `[1, 5]`.

#### Scenario: Judge returns parsed score

- **WHEN** the model responds `"4"`
- **THEN** `Score` SHALL return `4`

#### Scenario: Judge extracts integer from explanation

- **WHEN** the model responds `"Score: 3 because the answer is partially correct"`
- **THEN** `Score` SHALL return `3`

#### Scenario: Judge clamps out-of-range scores

- **WHEN** the model responds `"10"` or `"0"`
- **THEN** `Score` SHALL return `5` for `10` and `1` for `0`

### Requirement: Report marshals to JSON

The system SHALL implement `Report.MarshalJSON() ([]byte, error)` (or rely on the struct tags) producing a JSON object with fields `avg_precision_at_k`, `avg_recall_at_k`, `avg_mrr`, `avg_judge_score`, and `examples`.

#### Scenario: JSON output is human-readable

- **WHEN** a report is marshaled to JSON
- **THEN** the output SHALL contain the aggregate keys and be indented with 2-space indentation when using `json.MarshalIndent`
