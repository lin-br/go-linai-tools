# Purpose

TBD

# Requirements

## Requirement: Cohere rerank client package exists under internal/rag/rerank

The system SHALL provide a package `internal/rag/rerank` that exports a `Client` struct capable of calling the Cohere Rerank API endpoint (`https://api.cohere.com/v2/rerank`) using `net/http`. The package SHALL NOT depend on the Cohere SDK. The default rerank model SHALL be `rerank-v3.5`.

### Scenario: Client constructor accepts API key and HTTP client

- **WHEN** code calls `rerank.NewClient(apiKey, httpClient)` with a non-empty API key
- **THEN** it SHALL return a non-nil `*rerank.Client`

### Scenario: Missing API key returns error

- **WHEN** code calls `rerank.NewClient("", nil)`
- **THEN** it SHALL return an error indicating the API key is required

## Requirement: Rerank method accepts query and candidate documents

The system SHALL implement `Client.Rerank(ctx context.Context, query string, candidates []Candidate, topN int) ([]RankedResult, error)`. A `Candidate` struct SHALL contain `ID uuid.UUID` and `Text string`. A `RankedResult` struct SHALL contain `ID uuid.UUID`, `Text string`, `Index int` (original index in `candidates`), and `Score float64`. The method SHALL return at most `topN` results ordered by descending relevance score.

### Scenario: Candidates are reranked by relevance

- **WHEN** `client.Rerank(ctx, "annual budget", candidates, 3)` is called and Cohere returns scores `[0.9, 0.1, 0.5]` for three candidates
- **THEN** the returned slice SHALL have length 3, ordered by score descending, and the first `RankedResult.Index` SHALL be the index of the 0.9-scored candidate

### Scenario: topN limits results

- **WHEN** `client.Rerank(ctx, "q", candidates, 2)` is called with 5 candidates
- **THEN** the returned slice SHALL have length 2 and contain the two highest-scoring candidates

### Scenario: Empty candidates returns empty slice

- **WHEN** `client.Rerank(ctx, "q", nil, 5)` is called
- **THEN** it SHALL return an empty slice and no error without calling the API

## Requirement: Wire types are defined in a separate file

The system SHALL define request/response wire types in `internal/rag/rerank/wire.go`. The request type SHALL contain `Query string`, `Model string`, `Documents []string`, and `TopN int`. The response type SHALL contain a slice of `rerankResult` structs, each holding `Index int` and `RelevanceScore float64`.

### Scenario: Wire request serializes correctly

- **WHEN** a rerank request with `Query="q"`, `Model="rerank-v3.5"`, `Documents=["a","b"]`, and `TopN=2` is JSON-encoded
- **THEN** the JSON SHALL contain `{"query":"q","model":"rerank-v3.5","documents":["a","b"],"top_n":2}`

### Scenario: Wire response maps indexes back to candidates

- **WHEN** a response `{"results":[{"index":2,"relevance_score":0.99}]}` is decoded
- **THEN** `response.Results[0].Index` SHALL equal `2` and `response.Results[0].RelevanceScore` SHALL equal `0.99`

## Requirement: Client maps response indexes back to candidate IDs

The system SHALL map each result's `Index` from the Cohere response to the original `Candidate` slice and produce a `RankedResult` with the matching `ID` and `Text`. If an index is out of range, the method SHALL return an error.

### Scenario: Index mapping is correct

- **WHEN** a candidate at index 1 has ID `uuid-a` and Cohere returns a result with `index:1`
- **THEN** the corresponding `RankedResult.ID` SHALL equal `uuid-a`

### Scenario: Out-of-range index returns error

- **WHEN** Cohere returns an index `99` for a 3-candidate request
- **THEN** `client.Rerank` SHALL return an error indicating an invalid index

## Requirement: Constructor supports nil HTTP client fallback

The system SHALL provide `rerank.NewClient(apiKey string, httpClient *http.Client) *Client`. Passing `nil` for `httpClient` SHALL use `http.DefaultClient`.

### Scenario: nil HTTP client falls back to default

- **WHEN** `rerank.NewClient("sk-...", nil)` is called
- **THEN** the returned client SHALL use `http.DefaultClient`

## Requirement: Package exports a default model constant

The system SHALL export `const DefaultModel = "rerank-v3.5"` from `internal/rag/rerank`.

### Scenario: Default model is rerank-v3.5

- **WHEN** code references `rerank.DefaultModel`
- **THEN** its value SHALL be exactly `"rerank-v3.5"`
