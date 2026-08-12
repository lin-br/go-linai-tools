## ADDED Requirements

### Requirement: .claude/mcp.json registers the wiki-mcp server

The system SHALL provide a `.claude/mcp.json` file at the repository root. The file SHALL declare a server named `wiki-mcp` with command `go run ./cmd/wiki-mcp`, the current working directory set to the repository root, and any environment variables required by the Go runtime (e.g., `OPENROUTER_API_KEY` is NOT required by the server, so only `HOME`, `PATH`, or `GOPATH` if needed). The server SHALL be configured for stdio transport.

#### Scenario: Valid MCP configuration

- **WHEN** the contents of `.claude/mcp.json` are inspected
- **THEN** it SHALL contain a top-level `mcpServers` object with a `wiki-mcp` entry whose `command` array starts with `go` and includes `run`, `./cmd/wiki-mcp`

#### Scenario: Claude Code recognizes the server

- **WHEN** Claude Code loads `.claude/mcp.json`
- **THEN** it SHALL list `wiki-mcp` as an available MCP server and the server SHALL start successfully when invoked

### Requirement: /wiki-search slash command is defined in Claude Code settings

The system SHALL provide a `/wiki-search` slash command configuration. The command SHALL be defined in `.claude/commands/wiki-search.md` (or equivalent Claude Code slash command config) and SHALL instruct Claude Code to use the `wiki-mcp` server's `search_notes` tool to answer the user's query. The prompt in the command file SHALL guide the model to search the wiki and synthesize an answer.

#### Scenario: Slash command file exists

- **WHEN** `.claude/commands/wiki-search.md` is read
- **THEN** it SHALL contain instructions to call `search_notes` from the `wiki-mcp` MCP server and to answer based on the returned snippets

#### Scenario: Slash command references correct server and tool

- **WHEN** the slash command content is inspected
- **THEN** it SHALL mention the server name `wiki-mcp` and the tool name `search_notes`

### Requirement: /wiki-search accepts a natural-language query

When a user invokes `/wiki-search <query>`, Claude Code SHALL forward the query as the `query` argument to the `search_notes` tool on the `wiki-mcp` server. The server SHALL return matching notes, and Claude Code SHALL synthesize a concise answer from the snippets.

#### Scenario: User searches for a known topic

- **WHEN** the user runs `/wiki-search agent loop`
- **THEN** Claude Code SHALL call `search_notes` with `{"query": "agent loop"}` and respond with information from the returned wiki snippets

#### Scenario: No matches produce helpful fallback

- **WHEN** the user runs `/wiki-search quantum computing` and no wiki files match
- **THEN** Claude Code SHALL inform the user that no matching notes were found and suggest checking the wiki directory

### Requirement: MCP server command is executable from the repo root

The system SHALL ensure that `go run ./cmd/wiki-mcp` executes successfully from the repository root when Go is installed. The command SHALL not require additional build steps before Claude Code starts the server.

#### Scenario: Manual smoke test

- **WHEN** a developer runs `go run ./cmd/wiki-mcp --help` from the repo root
- **THEN** it SHALL print usage information and exit with code 0

#### Scenario: Build passes

- **WHEN** `go build ./cmd/wiki-mcp` is run
- **THEN** it SHALL produce a `wiki-mcp` binary without errors

### Requirement: Slash command integration is documented in the change notes

The system SHALL include a section in the change's documentation (or a `README.md` update) explaining how to enable and test the `/wiki-search` command, including the requirement to restart Claude Code after `.claude/mcp.json` is created or modified.

#### Scenario: Restart instruction is present

- **WHEN** the documentation is read
- **THEN** it SHALL state that Claude Code must be restarted or `/mcp` reloaded after `.claude/mcp.json` changes
