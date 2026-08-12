## ADDED Requirements

### Requirement: Loop exposes Run and RunStream entry points

The system SHALL provide an `internal/agent` package with a `Loop` struct constructed via `agent.NewLoop(provider outbound.Provider, opts agent.Options) *agent.Loop`. The `Loop` struct SHALL expose two methods: `Run(ctx context.Context, query string) (string, []Turn, error)` for blocking execution, and `RunStream(ctx context.Context, query string) (<-chan domain.StreamEvent, error)` for streaming execution. Both methods SHALL accept `context.Context` as the first parameter and propagate cancellation to the provider.

#### Scenario: Blocking run returns final answer and history

- **WHEN** the caller invokes `loop.Run(ctx, "what is 2+2?")` and the model returns the final text "4"
- **THEN** the method SHALL return `"4"`, a non-empty `[]Turn` containing at least one user message and one assistant message, and a nil error

#### Scenario: Streaming run returns a read-only event channel

- **WHEN** the caller invokes `loop.RunStream(ctx, "what is 2+2?")`
- **THEN** the method SHALL return a `<-chan domain.StreamEvent` and a nil error, and the channel SHALL be closed when the loop terminates

#### Scenario: Context cancellation aborts the loop

- **WHEN** the context passed to `Run` or `RunStream` is cancelled while the loop is waiting for the provider
- **THEN** the loop SHALL stop, propagate `ctx.Err()`, and not make additional provider calls

### Requirement: Loop executes tool calls requested by the model

The system SHALL register tools with `Loop` via `(*Loop).Register(tool agent.Tool)`. During each assistant response, the loop SHALL inspect `domain.ChatResponse.ToolCalls`. For every `domain.ToolCall`, the loop SHALL find the registered tool whose `Name()` matches `ToolCall.Name`, invoke `tool.Execute(ctx, []byte(toolCall.Arguments))`, and append a tool-result message to the conversation history. If no registered tool matches, the loop SHALL append a tool-result message containing the error text and continue.

#### Scenario: Single tool call is executed

- **WHEN** the model responds with one `ToolCall{Name: "calculate", Arguments: "{\"expression\":\"2+2\"}"}` and a `calculate` tool is registered
- **THEN** `calculate.Execute` SHALL be called with the arguments and the result "4" SHALL be appended to the conversation before the next provider call

#### Scenario: Multiple tool calls execute serially

- **WHEN** the model responds with two tool calls in one assistant message
- **THEN** the loop SHALL execute the first tool, append its result, execute the second tool, append its result, and only then call the provider again

#### Scenario: Unknown tool name is reported back

- **WHEN** the model requests a tool named "unknown_tool" that is not registered
- **THEN** the loop SHALL append a tool-result message stating that the tool is not available and continue the conversation

### Requirement: Loop enforces a hard maximum turn limit

The system SHALL configure `Loop` with `Options.MaxTurns int` defaulting to 10. Each provider call that follows a user or tool message counts as one turn. If the loop reaches `MaxTurns` without producing a final answer, `Run` SHALL return an empty string, the accumulated `[]Turn`, and `agent.ErrMaxTurnsExceeded`. `RunStream` SHALL emit the error on the channel and close it.

#### Scenario: Loop stops after the configured maximum

- **WHEN** a model repeatedly calls tools without returning final text and `MaxTurns` is set to 3
- **THEN** the loop SHALL halt after the third provider call and return `agent.ErrMaxTurnsExceeded`

#### Scenario: Default maximum is 10

- **WHEN** `NewLoop` is called with zero-value `Options`
- **THEN** the resulting loop SHALL have `MaxTurns` equal to 10

### Requirement: Loop retries provider calls with exponential backoff

The system SHALL retry each `provider.Chat` call on transient errors. The retry SHALL use exponential backoff with jitter, SHALL stop immediately when `ctx.Done()` is closed, and SHALL fail fast on non-retryable errors such as context cancellation or a configured non-retryable error predicate. The retry behavior SHALL be configured via `Options.RetryPolicy`.

#### Scenario: Transient provider error is retried

- **WHEN** `provider.Chat` returns a retryable network error on the first two attempts and succeeds on the third
- **THEN** `Run` SHALL eventually return the successful response without returning the transient errors

#### Scenario: Cancelled context aborts retry wait

- **WHEN** the context is cancelled during a retry backoff wait
- **THEN** the loop SHALL stop waiting and return `ctx.Err()` immediately

### Requirement: Loop builds requests that include tools and tool_choice

The system SHALL, on every provider call after the first user message, include the full conversation history plus the registered tools in the `domain.ChatRequest`. The `ToolChoice` field SHALL be omitted or set to `auto` so the model may decide whether to call a tool or answer. Tool definitions SHALL be derived from each registered tool's `Name()`, `Description()`, and `InputSchema()` methods.

#### Scenario: Tool definitions are included in the request

- **WHEN** two tools are registered and the loop makes a provider call
- **THEN** the `domain.ChatRequest.Tools` slice SHALL contain two `domain.Tool` values with matching names, descriptions, and input schemas

#### Scenario: Conversation history grows across turns

- **WHEN** the loop executes a tool call and calls the provider again
- **THEN** the second `domain.ChatRequest.Messages` SHALL contain the original user message, the assistant tool-call message, and the tool result message
