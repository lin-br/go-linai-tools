## ADDED Requirements

### Requirement: HTTP server exposes required endpoints

The system SHALL provide a `cmd/financas-ia/main.go` entry point that starts an HTTP server on the port defined by `PORT` environment variable (default `8080`) and registers handlers for `POST /chat`, `POST /ingest`, `GET /tasks/{id}`, and `GET /health`. The server SHALL use `net/http` `ServeMux` and SHALL NOT depend on external HTTP routers such as `chi` or `gin`.

#### Scenario: Server starts and listens

- **WHEN** the application is launched with `go run ./cmd/financas-ia`
- **THEN** it SHALL bind to `0.0.0.0:8080` (or `PORT`) and serve the four endpoints without panicking

#### Scenario: Unknown endpoint returns 404

- **WHEN** an HTTP client sends `GET /unknown`
- **THEN** the server SHALL respond with HTTP status `404 Not Found`

### Requirement: POST /chat accepts a chat request and returns a stream

The system SHALL implement `POST /chat` to accept a JSON body containing `message string` and optional `conversation_id string`, validate the request, and stream the assistant response via Server-Sent Events (SSE). The response SHALL have `Content-Type: text/event-stream` and `Cache-Control: no-cache`.

#### Scenario: Valid chat request streams SSE

- **WHEN** a client sends `POST /chat` with JSON body `{"message":"quanto gastei com mercado em abril?"}`
- **THEN** the server SHALL respond with HTTP `200 OK`, header `Content-Type: text/event-stream`, and a stream of SSE events

#### Scenario: Empty message returns 400

- **WHEN** a client sends `POST /chat` with JSON body `{"message":""}`
- **THEN** the server SHALL respond with HTTP status `400 Bad Request` and a JSON error body `{"error":"message is required"}`

#### Scenario: Non-JSON body returns 400

- **WHEN** a client sends `POST /chat` with `text/plain` body
- **THEN** the server SHALL respond with HTTP status `400 Bad Request`

### Requirement: POST /ingest accepts PDF upload and returns task ID

The system SHALL implement `POST /ingest` to accept a multipart/form-data upload containing a `file` field with a PDF, validate MIME type and size, persist a task record, and return HTTP `202 Accepted` with a JSON body containing `task_id string` and `status string`. The actual extraction/chunking/embedding SHALL run asynchronously.

#### Scenario: Valid PDF upload returns 202

- **WHEN** a client sends `POST /ingest` with `multipart/form-data` containing a valid PDF under field name `file`
- **THEN** the server SHALL respond with HTTP `202 Accepted` and JSON body `{"task_id":"<uuid>","status":"pending"}`

#### Scenario: Non-PDF upload rejected

- **WHEN** a client sends `POST /ingest` with a file whose MIME type is not `application/pdf`
- **THEN** the server SHALL respond with HTTP `400 Bad Request` and JSON body `{"error":"only application/pdf files are accepted"}`

#### Scenario: Oversized PDF rejected

- **WHEN** a client sends `POST /ingest` with a PDF larger than 10 MB
- **THEN** the server SHALL respond with HTTP `413 Payload Too Large`

### Requirement: GET /tasks/{id} returns task status and result

The system SHALL implement `GET /tasks/{id}` to read the task state from Redis (or in-memory fallback) and return a JSON object with fields `task_id`, `status` (`pending`, `running`, `completed`, `failed`), optional `message`, optional `result` containing the inserted `statement_id`, and optional `error`.

#### Scenario: Pending task returned

- **WHEN** a client sends `GET /tasks/{id}` for a task that was just created
- **THEN** the server SHALL respond with HTTP `200 OK` and JSON body `{"task_id":"...","status":"pending"}`

#### Scenario: Completed task returns result

- **WHEN** a client sends `GET /tasks/{id}` for a task whose ingestion finished successfully
- **THEN** the server SHALL respond with HTTP `200 OK` and JSON body containing `status: "completed"` and `result.statement_id`

#### Scenario: Unknown task returns 404

- **WHEN** a client sends `GET /tasks/{id}` for a non-existent task
- **THEN** the server SHALL respond with HTTP `404 Not Found`

### Requirement: GET /health checks database and LLM provider

The system SHALL implement `GET /health` that checks Postgres connectivity via `pgxpool.Ping(ctx)` and LLM provider reachability via a lightweight non-streaming call (e.g., `Provider.Chat` with a minimal prompt or cached model check). It SHALL return HTTP `200 OK` with `{"status":"healthy"}` only when both checks pass; otherwise HTTP `503 Service Unavailable` with `{"status":"unhealthy","checks":{...}}`.

#### Scenario: All dependencies healthy

- **WHEN** Postgres and the LLM provider are reachable
- **THEN** `GET /health` SHALL return `200 OK` and JSON `{"status":"healthy"}`

#### Scenario: Database unreachable

- **WHEN** Postgres is unreachable
- **THEN** `GET /health` SHALL return `503 Service Unavailable` and indicate the database check failed

### Requirement: Graceful shutdown via signal.NotifyContext

The system SHALL create the root context with `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` and propagate it to the HTTP server and all background workers. On shutdown signal, `server.Shutdown(ctx)` SHALL be called, active SSE connections SHALL be closed, and background ingestion goroutines SHALL finish their current step before exiting.

#### Scenario: SIGINT triggers graceful shutdown

- **WHEN** the process receives `SIGINT`
- **THEN** it SHALL stop accepting new connections, drain in-flight requests up to the shutdown timeout, and exit cleanly

### Requirement: Structured request logging with slog

The system SHALL log every HTTP request using `slog` with structured fields: `method`, `path`, `status`, `duration_ms`, and `request_id`. Logs SHALL be written to stderr and SHALL NOT include request bodies or PII.

#### Scenario: Request logged with duration

- **WHEN** any request completes
- **THEN** a structured log line SHALL be emitted containing `method`, `path`, `status`, and `duration_ms`
