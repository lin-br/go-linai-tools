## ADDED Requirements

### Requirement: POST /chat returns SSE with event envelopes

The system SHALL stream chat responses via `text/event-stream`. Each SSE event line SHALL be prefixed with `data: ` and terminated by two newlines. Each event payload SHALL be a JSON object with a `type` field. Valid types are: `text` (delta), `tool_call` (tool started/completed), `source` (retrieved chunk citation), `cost` (token/cost metadata), `done`, and `error`.

#### Scenario: Assistant text streamed as deltas

- **WHEN** a client opens `POST /chat` with a valid message
- **THEN** it SHALL receive SSE events like `data: {"type":"text","content":"Você gastou"}` followed by more `text` deltas until completion

#### Scenario: Stream ends with done event

- **WHEN** the assistant response is complete
- **THEN** the server SHALL send `data: {"type":"done"}` and close the response body

#### Scenario: Stream errors sent as SSE error event

- **WHEN** the chat orchestrator encounters a non-retryable provider error
- **THEN** the server SHALL send `data: {"type":"error","message":"..."}` and close the stream

### Requirement: Chat orchestrator uses agent loop with finance tools

The system SHALL implement `internal/financas/chat/orchestrator.go` with a `Run(ctx context.Context, query string, conversationID string, stream chan<- Event) error` method. The orchestrator SHALL invoke the existing `internal/agent/Loop` with finance-specific tools registered: `search_transactions`, `list_statements`, and `get_spending_summary`.

#### Scenario: Spending question triggers search_transactions

- **WHEN** the user asks "quanto gastei com mercado em abril?"
- **THEN** the agent SHALL call `search_transactions` with `category="mercado"`, `month=4`, and a follow-up summary answer

#### Scenario: List statements question triggers list_statements

- **WHEN** the user asks "quais faturas estão disponíveis?"
- **THEN** the agent SHALL call `list_statements` and respond with the available statement periods and banks

### Requirement: Tool results include retriever citations

The system SHALL augment the agent loop with retrieved context from `EmbeddingRepository.SearchEmbeddings`. Each `search_transactions` result SHALL include source `chunk_id` references. The orchestrator SHALL emit `source` SSE events with `chunk_id`, `statement_id`, and `excerpt` so the frontend can render citations.

#### Scenario: Retrieved chunks emitted as sources

- **WHEN** `search_transactions` retrieves relevant chunks
- **THEN** the orchestrator SHALL emit SSE events `data: {"type":"source","chunk_id":"...","excerpt":"..."}` before the final answer

### Requirement: Pending tool calls emit streaming action events

The system SHALL emit `tool_call` SSE events when a tool is invoked and when it completes. The event SHALL include `name`, `status` (`pending`, `running`, `completed`, `failed`), and optional `payload`.

#### Scenario: Search action visible to user

- **WHEN** the agent calls `search_transactions`
- **THEN** the frontend SHALL first receive `data: {"type":"tool_call","name":"search_transactions","status":"pending"}` and later `status":"completed"`

### Requirement: Cost badge metadata streamed

The system SHALL compute approximate cost per request from token counts returned by the provider (`Usage.PromptTokens`, `Usage.CompletionTokens`) and model pricing loaded from a static `internal/financas/pricing/prices.json` or env var. It SHALL emit a `cost` SSE event with fields `model`, `prompt_tokens`, `completion_tokens`, `cost_usd`.

#### Scenario: Cost event follows assistant response

- **WHEN** a chat response completes
- **THEN** the server SHALL emit `data: {"type":"cost","model":"anthropic/claude-sonnet-4-20250514","prompt_tokens":1200,"completion_tokens":300,"cost_usd":0.0054}`

### Requirement: Context cancellation aborts LLM calls

The chat orchestrator SHALL propagate `context.Context` through the agent loop to `Provider.ChatStream`. If the client closes the SSE connection (context cancelled), the LLM request SHALL stop and no further events SHALL be emitted.

#### Scenario: Client disconnect cancels generation

- **WHEN** the client closes the SSE connection mid-stream
- **THEN** the orchestrator SHALL detect `ctx.Err()`, stop calling the provider, and return promptly

### Requirement: Confirmation gates for destructive tools

The system SHALL NOT allow the agent loop to call any destructive or side-effecting tool (e.g., `delete_statement`, `ingest_statement`) without an explicit `confirmed: true` flag in the request. The orchestrator SHALL emit a `confirmation` event with `tool_name` and `payload`, and wait for a follow-up `POST /chat` message containing `confirmation_id` and `confirmed: true`.

#### Scenario: Ingestion via chat requires confirmation

- **WHEN** an agent would invoke `ingest_statement`
- **THEN** the orchestrator SHALL emit `data: {"type":"confirmation","tool_name":"ingest_statement","payload":{"file":"..."}}` and pause until the user confirms
