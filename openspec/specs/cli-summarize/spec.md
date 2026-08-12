# Requirements

### Requirement: SummarizeUseCase struct and constructor

The system SHALL define a `SummarizeUseCase` struct in `internal/core/usecases` that holds a `outbound.Provider`. The system SHALL provide a constructor `NewSummarizeUseCase(provider outbound.Provider) *SummarizeUseCase`. The use case SHALL NOT depend on `configs.Config` directly — the model and system prompt are passed as arguments to the streaming method, resolved by the caller.

#### Scenario: Construction with a provider

- **WHEN** `NewSummarizeUseCase(provider)` is called with a `Provider` implementation
- **THEN** the returned `*SummarizeUseCase` SHALL hold the provider and be ready to call `Stream`

#### Scenario: Usable with RetryProvider

- **WHEN** a `RetryProvider` (from MP2) is passed as the `Provider` to `NewSummarizeUseCase`
- **THEN** the `SummarizeUseCase` SHALL accept it without type assertion or special handling, because `RetryProvider` implements the `Provider` interface

### Requirement: Default summarization system prompt

The system SHALL define a package-level constant `DefaultSummarizeSystemPrompt` in `internal/core/usecases` containing a focused summarization directive. The prompt SHALL instruct the model to summarize concisely, capture key points, decisions, and action items, and produce no preamble.

#### Scenario: Default prompt content

- **WHEN** the `DefaultSummarizeSystemPrompt` constant is inspected
- **THEN** it SHALL be a non-empty string that directs the model to produce a concise, structured summary without preamble

### Requirement: SummarizeUseCase.Stream builds a streaming ChatRequest

The `SummarizeUseCase` SHALL provide a method `Stream(ctx context.Context, model, systemPrompt, input string, out io.Writer) error`. It SHALL construct a `*domain.ChatRequest` with: `Model` set to the `model` argument, `System` set to `systemPrompt`, `Stream` set to `true`, and a single `Messages` entry with `Role: "user"` and `Content` set to `input`. The `ChatRequest` SHALL NOT set `Tools`, `ToolChoice`, `MaxTokens`, `Temperature`, or `TopP` (all zero-valued).

#### Scenario: Request has streaming enabled

- **WHEN** `Stream(ctx, "anthropic/claude-sonnet-4-20250514", prompt, input, out)` is called
- **THEN** the `ChatRequest` passed to `provider.ChatStream` SHALL have `Stream` equal to `true`

#### Scenario: Request has single user message

- **WHEN** `Stream` is called with `input` set to `"This is a long text to summarize"`
- **THEN** the `ChatRequest.Messages` SHALL contain exactly one message with `Role: "user"` and `Content: "This is a long text to summarize"`

#### Scenario: Request carries the system prompt

- **WHEN** `Stream` is called with `systemPrompt` set to `"Summarize concisely"`
- **THEN** the `ChatRequest.System` SHALL equal `"Summarize concisely"`

### Requirement: SummarizeUseCase.Stream consumes the event channel and writes deltas

After calling `provider.ChatStream(ctx, req)`, the `Stream` method SHALL range over the returned `<-chan domain.StreamEvent`. For each event with `Type == StreamEventTypeText`, it SHALL write the `Delta` string to the `io.Writer` and flush immediately so the output is visible without waiting for stream completion. The method SHALL return `nil` after the channel closes with no error events.

#### Scenario: Text deltas written in order

- **WHEN** the provider emits `StreamEvent{Type: Text, Delta: "Hello"}` followed by `StreamEvent{Type: Text, Delta: " world"}`
- **THEN** the writer SHALL receive `"Hello"` then `" world"` in order, and each write SHALL be flushed before the next delta is processed

#### Scenario: Finish event does not write output

- **WHEN** the provider emits `StreamEvent{Type: StreamEventTypeFinish, StopReason: "stop", Usage: ...}`
- **THEN** nothing SHALL be written to the writer for this event, and the method SHALL continue until the channel closes

#### Scenario: Empty delta is not written

- **WHEN** the provider emits `StreamEvent{Type: StreamEventTypeText, Delta: ""}`
- **THEN** nothing SHALL be written to the writer for this event

### Requirement: SummarizeUseCase.Stream handles initial connection error

When `provider.ChatStream` returns a non-nil error (and a nil channel), the `Stream` method SHALL return that error immediately without writing anything to the `io.Writer`.

#### Scenario: Provider connection failure

- **WHEN** `provider.ChatStream` returns `(nil, errors.New("401 unauthorized"))`
- **THEN** `Stream` SHALL return the error and SHALL NOT write to the `io.Writer`

### Requirement: SummarizeUseCase.Stream handles mid-stream error events

When a `StreamEvent{Type: StreamEventTypeError, Err: err}` arrives on the channel, the `Stream` method SHALL stop ranging, return the `Err` from the event. Any text deltas received before the error event SHALL already have been written to the `io.Writer` and flushed — they are not rolled back.

#### Scenario: Mid-stream failure after partial output

- **WHEN** the provider emits a text delta `"Summar"`, then `StreamEvent{Type: StreamEventTypeError, Err: errors.New("connection reset")}`
- **THEN** `"Summar"` SHALL have been written and flushed to the writer, and `Stream` SHALL return the `connection reset` error

### Requirement: SummarizeUseCase.Stream respects context cancellation

The `Stream` method SHALL pass the `context.Context` to `provider.ChatStream` unmodified. If the context is cancelled while ranging over the channel, the provider SHALL close the channel (per the MP1 contract), and `Stream` SHALL return the context error or the provider's error, whichever surfaces.

#### Scenario: Cancelled context aborts streaming

- **WHEN** `Stream` is called with a context that is cancelled mid-stream
- **THEN** the provider SHALL close the channel and `Stream` SHALL return without writing further deltas

### Requirement: Summarize CLI entry point

The system SHALL provide a `cmd/summarize/main.go` with a `main()` function. The CLI SHALL: load config via `configs.LoadConfigs()`, construct an `OpenRouterProvider`, wrap it in a `RetryProvider`, construct a `SummarizeUseCase` with the retry-wrapped provider, read all of stdin into a string, resolve the model, call `useCase.Stream(ctx, model, systemPrompt, input, os.Stdout)`, and exit with code 0 on success or 1 on error.

#### Scenario: Successful summarization

- **WHEN** the CLI is invoked as `echo "long text" | summarize` and the provider streams a summary
- **THEN** the summary text SHALL be printed to stdout token-by-token and the process SHALL exit with code 0

#### Scenario: Empty stdin

- **WHEN** the CLI is invoked with empty stdin (no input piped)
- **THEN** the CLI SHALL still send the empty input as the user message (the provider decides how to respond) — the CLI does not reject empty input

### Requirement: Summarize CLI flags

The CLI SHALL support a `-model string` flag that overrides the config-resolved default model. When `-model` is empty (default), the CLI SHALL resolve the model via `config.Models.Get()`. The CLI SHALL support a `-system string` flag that overrides the default system prompt. When `-system` is empty (default), the CLI SHALL use `usecases.DefaultSummarizeSystemPrompt`.

#### Scenario: Model flag overrides config

- **WHEN** the CLI is invoked as `summarize -model "openai/gpt-4o"` and config default is `anthropic/claude-sonnet-4-20250514`
- **THEN** the `ChatRequest.Model` SHALL be `"openai/gpt-4o"`, not the config default

#### Scenario: No model flag uses config default

- **WHEN** the CLI is invoked as `summarize` with no `-model` flag and `config.Models.Get()` returns `"anthropic/claude-sonnet-4-20250514"`
- **THEN** the `ChatRequest.Model` SHALL be `"anthropic/claude-sonnet-4-20250514"`

#### Scenario: System flag overrides default prompt

- **WHEN** the CLI is invoked as `summarize -system "Give me a one-line TLDR"`
- **THEN** the `ChatRequest.System` SHALL be `"Give me a one-line TLDR"`, not `DefaultSummarizeSystemPrompt`

#### Scenario: Missing model is an error

- **WHEN** the CLI is invoked with no `-model` flag and `config.Models.Get()` returns nil
- **THEN** the CLI SHALL print an error to stderr and exit with code 1

### Requirement: Summarize CLI signal handling

The CLI SHALL create a context via `signal.NotifyContext(context.Background(), os.Interrupt)` and pass it to `useCase.Stream`. When the user presses Ctrl+C (SIGINT), the context SHALL be cancelled, the stream SHALL be aborted through the provider, and the CLI SHALL exit without a panic or deadlock.

#### Scenario: Ctrl+C cancels the stream

- **WHEN** the user presses Ctrl+C while the LLM is streaming a response
- **THEN** the context SHALL be cancelled, the provider SHALL close the stream channel, the use case SHALL stop writing, and the CLI SHALL exit promptly

### Requirement: Summarize CLI error output and exit codes

The CLI SHALL print errors to `os.Stderr` (not stdout), so that stdout contains only the streamed summary. On any error returned from `useCase.Stream` or config loading, the CLI SHALL print the error message to stderr and exit with code 1. On success, the CLI SHALL exit with code 0.

#### Scenario: Connection error goes to stderr

- **WHEN** `useCase.Stream` returns an initial connection error (e.g., 401 unauthorized)
- **THEN** the error message SHALL be printed to stderr, stdout SHALL be empty, and the exit code SHALL be 1

#### Scenario: Mid-stream error goes to stderr

- **WHEN** `useCase.Stream` returns a mid-stream error after partial output
- **THEN** the partial output SHALL remain on stdout, the error message SHALL be printed to stderr, and the exit code SHALL be 1

#### Scenario: Success exits zero

- **WHEN** `useCase.Stream` completes without error
- **THEN** the CLI SHALL exit with code 0

### Requirement: Summarize CLI reads all stdin before sending

The CLI SHALL read all of stdin into memory (via `io.ReadAll(os.Stdin)`) before constructing the request. The full stdin content SHALL be used as the single user message content in the `ChatRequest`.

#### Scenario: Multi-line stdin

- **WHEN** stdin contains multiple lines of text
- **THEN** the entire text including newlines SHALL be sent as the `Content` of the single user message

#### Scenario: Piped file input

- **WHEN** the CLI is invoked as `cat notes.txt | summarize`
- **THEN** the full contents of `notes.txt` SHALL be read as the input string and passed to `useCase.Stream`

### Requirement: No external CLI framework

The summarize CLI SHALL use only the standard `flag` package for argument parsing. No third-party CLI framework (cobra, urfave/cli, kingpin) SHALL be imported. The CLI SHALL use only standard library packages plus the project's own `configs`, `domain`, `outbound`, `usecases`, and adapter packages.

#### Scenario: Standard library only

- **WHEN** `cmd/summarize/main.go` is inspected
- **THEN** all imports SHALL be from the standard library or the project's own packages — no `spf13/cobra`, `urfave/cli`, or similar
