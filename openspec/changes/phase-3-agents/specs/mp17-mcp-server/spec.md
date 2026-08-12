## ADDED Requirements

### Requirement: wiki-mcp binary exposes a stdio MCP server

The system SHALL provide `cmd/wiki-mcp/main.go` that starts a stdio MCP server when invoked. The server SHALL read JSON-RPC requests from `os.Stdin` and write responses to `os.Stdout`. Diagnostic output SHALL go to `os.Stderr`. The server SHALL handle the MCP lifecycle messages `initialize` and `initialized` and SHALL shut down cleanly when stdin closes or the process receives `SIGINT`.

#### Scenario: Server starts and accepts initialize

- **WHEN** the binary is launched and a valid `initialize` JSON-RPC request is sent to stdin
- **THEN** it SHALL respond on stdout with a `InitializeResult` containing protocol version, server info, and supported capabilities

#### Scenario: Server shuts down on stdin close

- **WHEN** the parent process closes the stdin pipe
- **THEN** the server SHALL exit with code 0 without leaking goroutines

### Requirement: MCP server exposes wiki/ as a readable resource

The system SHALL register a resource with URI template `wiki://{path}` that reads markdown files from the configured wiki directory. A `resources/read` request for `wiki://notes.md` SHALL return the file contents as text. A `resources/list` request SHALL return all available wiki resources.

#### Scenario: Reading an existing wiki file

- **WHEN** a `resources/read` request is sent with URI `wiki://notes.md` and `notes.md` exists in the wiki directory
- **THEN** the server SHALL respond with the file contents as a text resource

#### Scenario: Listing wiki resources

- **WHEN** a `resources/list` request is sent
- **THEN** the server SHALL respond with a list of resources, one per `.md` file in the wiki directory

### Requirement: MCP server exposes search_notes tool

The system SHALL register a tool named `search_notes` with input schema `{query: string}`. When invoked via `tools/call`, the server SHALL search the configured wiki directory for markdown files containing the query (case-insensitive substring match) and return a list of matching notes with file path and snippet.

#### Scenario: Tool finds matching note

- **WHEN** `tools/call` is invoked with `name: "search_notes"` and arguments `{"query": "agent loop"}`
- **THEN** the server SHALL return a list of results, each containing `file` and `snippet` fields

#### Scenario: Tool handles no matches

- **WHEN** `tools/call` is invoked with a query that matches no files
- **THEN** the server SHALL return an empty results array, not an error

#### Scenario: Tool schema is advertised

- **WHEN** a `tools/list` request is sent
- **THEN** the response SHALL contain a tool named `search_notes` with a description and an input schema declaring `query` as a required string property

### Requirement: Wiki directory is configurable via CLI flag

The system SHALL accept a `--wiki-dir` string flag on `cmd/wiki-mcp` defaulting to `./wiki`. The server SHALL use this directory for both the `wiki/` resource reads and the `search_notes` tool. If the directory does not exist, the server SHALL log a warning to stderr on startup and serve empty resources/lists.

#### Scenario: Custom wiki directory

- **WHEN** the server is started with `--wiki-dir=/tmp/my-wiki`
- **THEN** resource reads and search SHALL read files from `/tmp/my-wiki`

#### Scenario: Missing directory logs warning

- **WHEN** the server is started with `--wiki-dir=/nonexistent`
- **THEN** it SHALL print a warning to stderr and continue running

### Requirement: MCP server uses mark3labs/mcp-go or hand-rolled JSON-RPC

The system SHALL use `github.com/mark3labs/mcp-go` for the stdio server unless the dependency is unavailable, in which case a hand-rolled JSON-RPC transport of approximately 200 lines is acceptable. The server SHALL parse JSON-RPC `id` fields correctly (string, number, or null) and SHALL return results with the same `id`.

#### Scenario: JSON-RPC id is echoed

- **WHEN** a request with `id: 42` is sent
- **THEN** the response SHALL have `id: 42`

#### Scenario: JSON-RPC id types are handled

- **WHEN** requests with string, number, and null `id` values are sent
- **THEN** the server SHALL handle each without panic

### Requirement: MCP server is covered by table-driven tests

The system SHALL include `cmd/wiki-mcp/main_test.go` with table-driven tests for `search_notes`, `resources/list`, and `resources/read`. Tests SHALL set up a temporary wiki directory, invoke the handler functions directly without launching a subprocess, and assert on returned JSON content.

#### Scenario: Search handler tested directly

- **WHEN** the `searchNotes` handler is called with a temporary directory containing one matching file
- **THEN** it SHALL return a result containing the file name and matching snippet

#### Scenario: List handler tested directly

- **WHEN** the `listResources` handler is called with a temporary directory containing two markdown files
- **THEN** it SHALL return two resource entries
