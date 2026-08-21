## ADDED Requirements

### Requirement: Hybrid search package exists under internal/rag/search

The system SHALL provide a package `internal/rag/search` that exports a `Searcher` interface with method `Search(ctx context.Context, query string, queryVec []float32, k int) ([]Result, error)`. A `Result` struct SHALL contain `ID uuid.UUID`, `Content string`, `SourcePath string`, `Metadata map[string]any`, and `Score float64`. Two implementations SHALL be provided: `VectorSearcher` (delegates to `store.Store`) and `BM25Searcher` (uses `blevesearch`).

#### Scenario: Searcher interface compiles

- **WHEN** code declares `var _ search.Searcher = (*search.VectorSearcher)(nil)` and `var _ search.Searcher = (*search.BM25Searcher)(nil)`
- **THEN** the program SHALL compile

#### Scenario: Result struct carries required fields

- **WHEN** a `search.Result` value is created
- **THEN** it SHALL expose `ID`, `Content`, `SourcePath`, `Metadata`, and `Score`

### Requirement: VectorSearcher delegates to store.Store

The system SHALL implement `search.NewVectorSearcher(store *store.Store) *VectorSearcher`. Its `Search` method SHALL call `store.Search(ctx, queryVec, k)` and convert each `store.SearchResult` into a `search.Result` where `Score = 1 / (1 + Distance)` (higher is better).

#### Scenario: VectorSearcher returns converted results

- **WHEN** `vectorSearcher.Search(ctx, "ignored", queryVec, 5)` is called and the store returns one chunk with distance `0.5`
- **THEN** the returned result SHALL have `Score == 1.0/1.5` and preserve `Content` and `SourcePath`

### Requirement: BM25Searcher builds an in-memory Bleve index from chunks

The system SHALL implement `search.NewBM25Searcher() *BM25Searcher` with methods `Index(ctx context.Context, chunks []chunk.Chunk) error` and `Search(ctx context.Context, query string, k int) ([]Result, error)`. It SHALL use `github.com/blevesearch/bleve/v2` with a default mapping indexing the `Content` field. The `Search` method SHALL run a Bleve query and return the top `k` results with scores mapped from Bleve's score.

#### Scenario: BM25 index and search round-trip

- **WHEN** `bm25.Index(ctx, chunks)` is called and then `bm25.Search(ctx, "budget", 3)` is called
- **THEN** it SHALL return results ordered by BM25 relevance, with documents matching "budget" ranked higher

#### Scenario: Empty index returns empty results

- **WHEN** `bm25.Search(ctx, "anything", 5)` is called before `Index` is called
- **THEN** it SHALL return an empty slice and no error

### Requirement: Hybrid merges BM25 and vector results with RRF

The system SHALL implement `search.NewHybridSearcher(bm25 *BM25Searcher, vector *VectorSearcher, kRRF int) *HybridSearcher`. Its `Search(ctx, query, queryVec, k)` method SHALL:
1. Fetch `k*4` results from `BM25Searcher`.
2. Fetch `k*4` results from `VectorSearcher`.
3. Assign each result a reciprocal rank score `1 / (rank + kRRF)` where `rank` starts at 1.
4. Sum scores for documents appearing in both lists by `ID`.
5. Return the top `k` results by fused score, descending.

`kRRF` SHALL default to 60 when zero is passed to the constructor.

#### Scenario: Fused results combine both retrievers

- **WHEN** `hybrid.Search(ctx, "q", queryVec, 3)` is called and both retrievers return overlapping candidate sets
- **THEN** the result list SHALL have at most 3 items and documents appearing in both lists SHALL outrank documents appearing in only one

#### Scenario: Default RRF constant is 60

- **WHEN** `search.NewHybridSearcher(bm25, vector, 0)` is called
- **THEN** the internal `kRRF` SHALL be `60`

#### Scenario: Ties broken by vector rank

- **WHEN** two documents have the same fused RRF score
- **THEN** the one with the better (lower) vector-search rank SHALL appear first

### Requirement: Index rebuild is exposed for hybrid search

The system SHALL provide `HybridSearcher.Index(ctx context.Context, chunks []chunk.Chunk) error` which forwards the chunks to the `BM25Searcher`. This allows the CLI to load chunks from Postgres and rebuild the keyword index per run.

#### Scenario: Hybrid index forwards to BM25

- **WHEN** `hybrid.Index(ctx, chunks)` is called
- **THEN** the underlying `BM25Searcher` SHALL be indexed with those chunks and subsequent keyword searches SHALL find them

### Requirement: Search results preserve source metadata

Both `VectorSearcher` and `BM25Searcher` SHALL copy `Metadata` from the source chunk/store result into `search.Result.Metadata` without mutation.

#### Scenario: Metadata survives search

- **WHEN** a search result is returned by any searcher
- **THEN** its `Metadata["index"]` and `Metadata["strategy"]` keys (from chunking) SHALL be present and unchanged
