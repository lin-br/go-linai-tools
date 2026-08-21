## ADDED Requirements

### Requirement: Voyage embeddings client package exists under internal/rag/embeddings

The system SHALL provide a package `internal/rag/embeddings` that exports a `Client` struct capable of calling the Voyage AI embeddings endpoint (`https://api.voyageai.com/v1/embeddings`) using `net/http`. The package SHALL NOT depend on the Voyage SDK. The default embedding model SHALL be `voyage-3-large` and SHALL produce 1024-dimensional vectors.

#### Scenario: Client constructor accepts API key and HTTP client

- **WHEN** code calls `embeddings.NewClient(apiKey, httpClient)` with a non-empty API key
- **THEN** it SHALL return a non-nil `*embeddings.Client` configured with the provided API key and HTTP client

#### Scenario: Client produces float32 vectors

- **WHEN** code calls `client.Embed(ctx, []string{"hello world"})`
- **THEN** the method SHALL return a `[][]float32` where each inner slice has length 1024 and the outer slice length matches the input batch size

#### Scenario: API key missing

- **WHEN** code calls `embeddings.NewClient("", nil)`
- **THEN** it SHALL return an error indicating that the API key is required

### Requirement: Embed method batches inputs and returns dense embeddings

The system SHALL implement `Client.Embed(ctx context.Context, inputs []string) ([][]float32, error)`. The method SHALL JSON-encode a request containing `input`, `model`, and optional `input_type` fields, POST it to the Voyage embeddings endpoint, decode the response, and extract the embedding list. The method SHALL respect context cancellation and propagate HTTP and JSON errors.

#### Scenario: Single input embedding

- **WHEN** `client.Embed(ctx, []string{"single sentence"})` is called and the Voyage API returns a 200 response with one embedding array
- **THEN** it SHALL return `[][]float32{embedding}` with one 1024-element slice

#### Scenario: Batched inputs preserved order

- **WHEN** `client.Embed(ctx, []string{"a", "b", "c"})` is called and the Voyage API returns three embeddings
- **THEN** the output embeddings SHALL appear in the same order as the inputs (`a`, then `b`, then `c`)

#### Scenario: Context cancellation aborts request

- **WHEN** the provided context is cancelled before the HTTP response completes
- **THEN** `client.Embed` SHALL return the context error and SHALL NOT block indefinitely

### Requirement: Wire types are defined in a separate file

The system SHALL define request/response wire types in `internal/rag/embeddings/wire.go`. The request type SHALL be named `voyageRequest` (or `VoyageRequest` if exported) with fields `Input []string`, `Model string`, and `InputType string`. The response type SHALL be named `voyageResponse` (or `VoyageResponse`) with a slice of `voyageEmbedding` structs, each holding a `[]float32` embedding.

#### Scenario: Wire request serializes correctly

- **WHEN** a `voyageRequest{Input: []string{"x"}, Model: "voyage-3-large", InputType: "document"}` is JSON-encoded
- **THEN** the resulting JSON SHALL contain `{"input":["x"],"model":"voyage-3-large","input_type":"document"}`

#### Scenario: Wire response deserializes correctly

- **WHEN** a JSON response `{"data":[{"embedding":[0.1,...]}],"model":"voyage-3-large","usage":{"total_tokens":2}}` is decoded into the response type
- **THEN** `response.Data[0].Embedding` SHALL contain the decoded slice

### Requirement: Client normalizes vectors

The system SHALL normalize returned vectors to unit length using L2 normalization before returning them from `client.Embed`. This allows vector search in pgvector to use inner product or L2 distance consistently with the Voyage model defaults.

#### Scenario: Vector length is 1.0 after normalization

- **WHEN** the client receives any non-zero embedding from the API
- **THEN** the returned vector SHALL have an L2 norm of 1.0 within floating-point tolerance

### Requirement: Constructor supports functional options or explicit HTTP client override

The system SHALL provide `embeddings.NewClient(apiKey string, httpClient *http.Client) *Client` as the constructor. Passing `nil` for `httpClient` SHALL use `http.DefaultClient`.

#### Scenario: nil HTTP client falls back to default

- **WHEN** `embeddings.NewClient("sk-...", nil)` is called
- **THEN** the returned client SHALL use `http.DefaultClient` for requests

#### Scenario: Custom HTTP client is used

- **WHEN** `embeddings.NewClient("sk-...", customClient)` is called
- **THEN** the returned client SHALL use `customClient` for requests, allowing test doubles

### Requirement: Package exports a default model constant

The system SHALL export `const DefaultModel = "voyage-3-large"` from `internal/rag/embeddings`.

#### Scenario: Default model is voyage-3-large

- **WHEN** code references `embeddings.DefaultModel`
- **THEN** its value SHALL be exactly `"voyage-3-large"`
