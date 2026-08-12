## ADDED Requirements

### Requirement: Frontend is served by the Go backend

The system SHALL serve static frontend assets from `web/financas-ia/` under `GET /` and `GET /static/*`. The Go binary SHALL embed the files using `//go:embed` and `net/http.FileServerFS` so the frontend works with a single binary.

#### Scenario: Root path serves the chat UI

- **WHEN** a browser sends `GET /`
- **THEN** the server SHALL respond with `text/html` containing the chat UI

#### Scenario: Static assets served under /static

- **WHEN** a browser sends `GET /static/app.js`
- **THEN** the server SHALL respond with the contents of `web/financas-ia/app.js` and correct `Content-Type`

### Requirement: Chat UI renders messages and streaming text

The frontend SHALL display user messages and assistant responses in a scrollable chat panel. Assistant text SHALL render incrementally as SSE `text` events arrive. Each message SHALL be visually distinguished (user on the right, assistant on the left).

#### Scenario: User message appears after submit

- **WHEN** the user types a message and presses Enter or clicks Send
- **THEN** the message SHALL appear in the chat panel and the input SHALL clear

#### Scenario: Assistant response streams in

- **WHEN** SSE events `{"type":"text","content":"R$"}` then `{"type":"text","content":" 1.234"}` arrive
- **THEN** the assistant message SHALL first show "R$" and then "R$ 1.234" without re-rendering the whole panel

### Requirement: Drag-and-drop PDF upload triggers ingestion

The frontend SHALL provide a drop zone where users can drag a PDF file. On drop, it SHALL send `POST /ingest` as `multipart/form-data`, display the returned `task_id`, and poll `GET /tasks/{id}` every 2 seconds until status is `completed` or `failed`. During processing, it SHALL show a progress indicator.

#### Scenario: Dropping a PDF starts ingestion

- **WHEN** the user drops a valid PDF onto the drop zone
- **THEN** the frontend SHALL POST the file, receive `task_id`, and show "Processando extrato..."

#### Scenario: Poll completes and statement available

- **WHEN** the task status becomes `completed`
- **THEN** the frontend SHALL show "Extrato processado" and allow the user to ask questions about it

### Requirement: Pending-actions indicator shows active tool calls

The frontend SHALL render a pending-actions indicator (e.g., a status line or card) whenever it receives a `tool_call` event with `status` of `pending` or `running`. When a matching `tool_call` event with `status` `completed` or `failed` arrives, the indicator SHALL update or disappear.

#### Scenario: Search tool shows indicator

- **WHEN** the frontend receives `{"type":"tool_call","name":"search_transactions","status":"pending"}`
- **THEN** it SHALL display "Buscando transações..."

#### Scenario: Tool completes hides indicator

- **WHEN** the frontend receives `{"type":"tool_call","name":"search_transactions","status":"completed"}`
- **THEN** the pending indicator SHALL be removed or marked as done

### Requirement: Confirmation card blocks destructive actions

The frontend SHALL render a confirmation card when it receives a `confirmation` event. The card SHALL display the tool name, a human-readable summary, and Accept/Reject buttons. If the user accepts, the frontend SHALL send a follow-up message with `confirmation_id` and `confirmed: true`. If rejected, it SHALL send `confirmed: false` and the backend SHALL abort the tool.

#### Scenario: Ingestion confirmation card

- **WHEN** the frontend receives `{"type":"confirmation","tool_name":"ingest_statement","payload":{"file":"fatura.pdf"}}`
- **THEN** it SHALL show a card asking "Confirmar ingestão de fatura.pdf?" with buttons "Confirmar" and "Cancelar"

#### Scenario: User rejects confirmation

- **WHEN** the user clicks "Cancelar"
- **THEN** the frontend SHALL send `POST /chat` with `{"confirmation_id":"...","confirmed":false}` and remove the card

### Requirement: Cost badge displays per-response cost

The frontend SHALL render a small cost badge next to each assistant message when a `cost` event is received. The badge SHALL display the total cost in USD rounded to 4 decimals (e.g., "$0.0054") and show model and token counts on hover.

#### Scenario: Cost badge appears

- **WHEN** the frontend receives `{"type":"cost","cost_usd":0.0054}`
- **THEN** it SHALL display a badge with "$0.0054" next to the current assistant message

### Requirement: Cancellation and retry UI

The frontend SHALL provide a "Cancel" button during streaming and a "Retry" button after an `error` event. Clicking Cancel SHALL close the `EventSource` connection. Clicking Retry SHALL resend the original user message.

#### Scenario: Cancel stops streaming

- **WHEN** the user clicks Cancel while the assistant is streaming
- **THEN** the frontend SHALL close the EventSource and display "Resposta cancelada" inline

#### Scenario: Retry after error

- **WHEN** the frontend receives `{"type":"error","message":"model unavailable"}`
- **THEN** it SHALL show a "Tentar novamente" button that re-sends the message

### Requirement: Responsive layout for desktop and mobile

The frontend SHALL use a single-column layout that adapts to mobile widths without horizontal scroll. The chat panel, drop zone, and input SHALL remain usable on screens as narrow as 360 px.

#### Scenario: Mobile viewport usable

- **WHEN** the browser width is 360 px
- **THEN** the chat panel and input SHALL stack vertically and remain fully visible
