## ADDED Requirements

### Requirement: Async task package supports submit and status

The system SHALL provide package `internal/async` with `Queue` struct and methods `Submit(ctx context.Context, name string, task func(ctx context.Context) (any, error)) string`, `Status(id string) (TaskStatus, error)`, and `Shutdown(ctx context.Context) error`. `TaskStatus` SHALL be a string type with values `pending`, `running`, `done`, and `failed`. `Submit` SHALL return a unique `task_id`, start the task in a goroutine, and immediately return. `Status` SHALL return the current state, result, and error for the given `task_id`.

#### Scenario: Submit returns a task ID

- **WHEN** `Submit(ctx, "long-agent", func(...) (any, error) { return "ok", nil })` is called
- **THEN** it SHALL return a non-empty string `task_id` and the initial status SHALL be `pending` or `running`

#### Scenario: Completed task returns result

- **WHEN** a submitted task finishes successfully and `Status(id)` is called afterwards
- **THEN** it SHALL return `done`, the result value, and a nil error

#### Scenario: Failed task returns error

- **WHEN** a submitted task returns a non-nil error
- **THEN** `Status(id)` SHALL return `failed`, a nil result, and the original error

### Requirement: Task IDs are unique and lookup returns ErrTaskNotFound

The system SHALL generate task IDs using `crypto/rand` or a monotonic counter encoded as a string. `Status` SHALL return `ErrTaskNotFound` when the ID does not exist.

#### Scenario: Unknown task ID

- **WHEN** `Status("does-not-exist")` is called
- **THEN** it SHALL return an error equal to `ErrTaskNotFound`

### Requirement: HTTP handlers expose async tasks

The system SHALL provide `internal/async/http.go` with `Handler` struct mounting two routes: `POST /tasks` and `GET /tasks/:id`. `POST /tasks` SHALL accept a JSON body `{name, payload}` and a `task_func` registered by name, call `Queue.Submit`, and return `202 Accepted` with body `{task_id, status, poll_url}`. `GET /tasks/:id` SHALL call `Queue.Status` and return `200 OK` with `{task_id, status, result, error}` when found, or `404 Not Found` when the task ID does not exist.

#### Scenario: Submit endpoint returns 202

- **WHEN** `POST /tasks` is called with a valid registered task name
- **THEN** the response SHALL have status 202 and a JSON body containing `task_id` and `poll_url`

#### Scenario: Poll endpoint returns status

- **WHEN** `GET /tasks/{task_id}` is called for an existing task
- **THEN** the response SHALL have status 200 and a JSON body containing `task_id`, `status`, and either `result` or `error`

#### Scenario: Poll unknown task returns 404

- **WHEN** `GET /tasks/{task_id}` is called for a non-existent task
- **THEN** the response SHALL have status 404

### Requirement: Queue supports graceful shutdown

`Queue.Shutdown(ctx context.Context) error` SHALL wait for running tasks to complete or until the context is cancelled. New submissions after shutdown SHALL return `ErrQueueClosed`.

#### Scenario: Shutdown waits for running tasks

- **WHEN** a task is running and `Shutdown(ctx)` is called with a 5-second context
- **THEN** it SHALL return nil after the running task completes, or return `ctx.Err()` if the deadline expires first

#### Scenario: Submit after shutdown is rejected

- **WHEN** `Queue.Shutdown` has been called and `Submit` is called again
- **THEN** `Submit` SHALL return an empty string and `ErrQueueClosed`

### Requirement: Async tasks integrate with the agent loop

The system SHALL provide an adapter in `internal/async/agent.go` that runs `agent.Loop.Run` inside a task function. The adapter SHALL accept an initial query and the agent loop, submit it to the queue, and return the task ID. This adapter SHALL be used by `POST /tasks` when the task name is `"agent-run"`.

#### Scenario: Agent-run task returns final answer

- **WHEN** `POST /tasks` with name `"agent-run"` and payload `{"query":"what is 2+2?"}` is submitted
- **THEN** after polling to completion, the result SHALL contain the agent loop's final answer
