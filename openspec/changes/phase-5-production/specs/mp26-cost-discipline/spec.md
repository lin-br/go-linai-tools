## ADDED Requirements

### Requirement: Response cache keyed by prompt/model/params hash

The system SHALL provide package `internal/cost/cache.go` with `Cache` interface and an `InMemoryCache` implementation. `Cache.Get(ctx context.Context, key string) (*domain.ChatResponse, bool)` and `Cache.Set(ctx context.Context, key string, resp *domain.ChatResponse)` SHALL store and retrieve responses. `BuildCacheKey(prompt, model string, params map[string]any) string` SHALL compute `hex(sha256(prompt + "\x00" + model + "\x00" + sortedParamsJSON))`. The cache SHALL be safe for concurrent use.

#### Scenario: Cache hit returns stored response

- **WHEN** `Set` is called with key `"k1"` and response `r1`, then `Get` is called with `"k1"`
- **THEN** `Get` SHALL return `r1` and `true`

#### Scenario: Cache miss returns nil and false

- **WHEN** `Get` is called with a key that has never been set
- **THEN** it SHALL return `nil` and `false`

#### Scenario: Concurrent access is safe

- **WHEN** multiple goroutines simultaneously call `Get` and `Set` on the same cache instance
- **THEN** the race detector SHALL report no data races

### Requirement: Cached provider decorator wraps outbound.Provider

The system SHALL provide `CachedProvider` in `internal/cost/cache.go` implementing `outbound.Provider`. `NewCachedProvider(inner outbound.Provider, cache Cache) *CachedProvider` SHALL compute the cache key from request fields, check the cache, return the cached response on hit, and delegate to `inner.Chat` on miss (storing the result before returning). For `ChatStream`, it SHALL only read the stream from the inner provider (streaming responses are not cached).

#### Scenario: Cached provider returns cached Chat response

- **WHEN** `CachedProvider.Chat(ctx, req)` is called twice with identical prompts, model, and params
- **THEN** the inner provider SHALL be invoked only once and the second call SHALL return the cached response

#### Scenario: ChatStream bypasses cache

- **WHEN** `CachedProvider.ChatStream(ctx, req)` is called
- **THEN** it SHALL always delegate to the inner provider and SHALL NOT cache events

### Requirement: Prompt compression reduces token count before main call

The system SHALL provide `internal/cost/compress.go` with `Compressor` struct and `Compress(ctx context.Context, prompt string) (string, error)`. `Compressor` SHALL call a cheap model via `outbound.Provider.Chat` with a system prompt instructing it to summarize the user prompt while preserving intent and constraints. The default cheap model SHALL be resolved from config key `models.free` and overridable via constructor parameter.

#### Scenario: Long prompt is compressed

- **WHEN** a 500-token prompt is passed to `Compress`
- **THEN** the returned prompt SHALL be shorter while retaining the original intent

#### Scenario: Short prompts bypass compression

- **WHEN** a prompt with fewer than 50 runes is passed to `Compress`
- **THEN** `Compress` SHALL return the original prompt unchanged and SHALL NOT call the provider

### Requirement: Model routing table maps task class to model

The system SHALL provide `internal/cost/router.go` with `Router` struct and `Route(taskClass string) string`. The router SHALL use a default table: `classification` → cheap model, `extraction` → pro model, `generation` → pro model, `reasoning` → pro model (or opus if configured), `default` → default model. Constructor `NewRouter(cfg RouterConfig)` SHALL accept overrides from `configs.Models`.

#### Scenario: Classification task routes to cheap model

- **WHEN** `Route("classification")` is called
- **THEN** it SHALL return the configured cheap model identifier

#### Scenario: Unknown task class falls back to default

- **WHEN** `Route("unknown")` is called
- **THEN** it SHALL return the configured default model identifier

### Requirement: Cost-discipline wiring is composable

The system SHALL document and apply the order: `RetryProvider` → `TracedProvider` → `CachedProvider` → `Compressor`/`Router` logic in the use case layer. The use case layer SHALL call `router.Route(taskClass)` to choose the model, optionally call `compressor.Compress`, then call `provider.Chat` (which may hit the cache). Cache keys SHALL include the final compressed prompt and selected model.

#### Scenario: Full pipeline reduces duplicate calls

- **WHEN** the same compressed prompt and model are sent twice through the wired provider
- **THEN** the second call SHALL hit the cache and the LLM SHALL be invoked only once
