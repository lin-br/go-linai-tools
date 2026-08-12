## 1. Provider package skeleton

- [ ] 1.1 Create `internal/providers/` package directory
- [ ] 1.2 Create `internal/providers/provider.go` with `Provider` interface, `New(kind string, cfg Config) (Provider, error)`, and sentinel errors `ErrMissingCredential`, `ErrStreamingNotImplemented`
- [ ] 1.3 Create `internal/providers/config.go` with `Config` struct and `Validate` method
- [ ] 1.4 Add `var _ outbound.Provider = (*OpenRouterProvider)(nil)` compile-time check placeholder if needed; keep `OpenRouterProvider` in `http_clients` unchanged

## 2. AnthropicProvider

- [ ] 2.1 Create `internal/providers/anthropic_request.go` with Messages API request wire structs
- [ ] 2.2 Create `internal/providers/anthropic_response.go` with Messages API response wire structs
- [ ] 2.3 Create `internal/providers/anthropic.go` with `AnthropicProvider` struct, constructor `NewAnthropicProvider(apiKey string) *AnthropicProvider`, `Chat`, and `ChatStream`
- [ ] 2.4 Implement `toWire` and `fromWire` helpers mapping `domain.ChatRequest` <-> `MessagesRequest` and `MessageResponse` <-> `domain.ChatResponse`
- [ ] 2.5 Add `var _ outbound.Provider = (*AnthropicProvider)(nil)` compile-time check

## 3. OpenAIProvider

- [ ] 3.1 Create `internal/providers/openai_wire.go` with OpenAI request/response wire structs
- [ ] 3.2 Create `internal/providers/openai.go` with `OpenAIProvider` struct, constructor, `Chat`, and `ChatStream`
- [ ] 3.3 Implement `toWire` and `fromWire` helpers mapping `domain.ChatRequest` <-> `ChatCompletionRequest` and `ChatCompletionResponse` <-> `domain.ChatResponse`
- [ ] 3.4 Implement SSE parsing for `ChatStream` using `bufio.Scanner` and `data:` prefix
- [ ] 3.5 Add `var _ outbound.Provider = (*OpenAIProvider)(nil)` compile-time check

## 4. GeminiProvider

- [ ] 4.1 Create `internal/providers/gemini_wire.go` with `GenerateContentRequest`, `GenerateContentResponse`, and related structs
- [ ] 4.2 Create `internal/providers/gemini.go` with `GeminiProvider` struct, constructor, `Chat`, and `ChatStream`
- [ ] 4.3 Implement `toWire` and `fromWire` helpers mapping `domain.ChatRequest` <-> `GenerateContentRequest` and `GenerateContentResponse` <-> `domain.ChatResponse`
- [ ] 4.4 Add `var _ outbound.Provider = (*GeminiProvider)(nil)` compile-time check

## 5. BedrockProvider

- [ ] 5.1 Create `internal/providers/bedrock_wire.go` with Bedrock Converse request/response structs
- [ ] 5.2 Create `internal/providers/bedrock.go` with `BedrockProvider` struct, constructor, `Chat`, and `ChatStream`
- [ ] 5.3 Implement SigV4 signing using `crypto/hmac`, `crypto/sha256`, and the standard library
- [ ] 5.4 Read `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, and `AWS_REGION` from environment at construction time
- [ ] 5.5 Add `var _ outbound.Provider = (*BedrockProvider)(nil)` compile-time check

## 6. Provider tests

- [ ] 6.1 Create `internal/providers/providers_test.go` with table-driven tests for factory success/error paths
- [ ] 6.2 Create `internal/providers/anthropic_test.go` with `httptest.Server` tests for `Chat`, `ChatStream`, and error handling
- [ ] 6.3 Create `internal/providers/openai_test.go` with `httptest.Server` tests for `Chat`, `ChatStream`, and error handling
- [ ] 6.4 Create `internal/providers/gemini_test.go` with `httptest.Server` tests for `Chat` and error handling
- [ ] 6.5 Create `internal/providers/bedrock_test.go` with `httptest.Server` tests for `Chat`, SigV4 header presence, missing credentials, and error handling
- [ ] 6.6 Run `go test ./internal/providers/...` and fix failures

## 7. Benchmark CLI prompts and pricing

- [ ] 7.1 Create `cmd/model-roulette/prompts.go` with `BenchmarkPrompt` struct and `AllPrompts()` returning ten prompt categories
- [ ] 7.2 Create `cmd/model-roulette/pricing.go` with static `map[string]ModelPricing` for known models
- [ ] 7.3 Create `cmd/model-roulette/writer.go` with `Result` struct, `CSVWriter`, `NewCSVWriter(io.Writer)`, and `WriteResult(Result) error`
- [ ] 7.4 Create `cmd/model-roulette/writer_test.go` with table-driven tests for header, success row, error row, and writer error

## 8. Benchmark runner

- [ ] 8.1 Create `cmd/model-roulette/runner.go` with `Runner` struct and `Run(ctx context.Context) error`
- [ ] 8.2 Implement prompt/model nested loops and `domain.ChatRequest` building
- [ ] 8.3 Implement latency measurement with `time.Since(start).Milliseconds()`
- [ ] 8.4 Implement cost estimation from the static price table
- [ ] 8.5 Implement error recording without aborting the run
- [ ] 8.6 Implement context cancellation handling
- [ ] 8.7 Create `cmd/model-roulette/runner_test.go` with a fake provider, verifying row count, latency, cost, errors, and cancellation

## 9. Benchmark CLI entry point

- [ ] 9.1 Create `cmd/model-roulette/main.go` using the `flag` package for `-providers`, `-models`, `-output`, `-runs`, `-timeout`
- [ ] 9.2 Wire `configs.LoadConfigs()` → provider factory → runner
- [ ] 9.3 Add `signal.NotifyContext(context.Background(), os.Interrupt)` and pass context to `Runner.Run`
- [ ] 9.4 Validate flags and print errors to stderr with appropriate exit codes

## 10. Documentation and verification

- [ ] 10.1 Create `docs/model-selection.md` with required sections: "When to pick which model", "Latency vs quality trade-offs", "Cost per 1M tokens", "Prompt category notes", "Interpreting the CSV", and "Refresh log"
- [ ] 10.2 Run `go build ./...` and fix compile errors
- [ ] 10.3 Run `go test ./...` and fix failures
- [ ] 10.4 Run `go run ./cmd/model-roulette -output /tmp/roulette.csv` manually and verify CSV header and rows
- [ ] 10.5 Update `docs/roadmap-ai-engineer-status.md` Phase 4 status to COMPLETE with completion date and LinkedIn post #4 angle
