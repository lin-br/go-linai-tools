## Why

Phase 1–2 built the provider boundary, structured extraction, and streaming primitives. Phase 3 is where those pieces become an agent: a reusable loop that calls a model, executes tools, feeds results back, and finishes with an answer. The milestone is "slash command calling your own MCP server" — a Claude Code `/wiki-search` command that talks to a Go stdio MCP server exposing a local wiki resource and search tool. Without this phase, there is no agentic layer and no integration between Claude Code and the user's own tools.

## What Changes

- Add `internal/agent/` package with a `Loop` struct and `Run(ctx, query) (string, []Turn, error)` — max turns ~10, retry/backoff on provider errors, streaming via a `chan domain.StreamEvent` returned by a separate `RunStream` method.
- Add `internal/agent/tools/` with three callable test tools: `get_weather`, `search_wiki`, and `calculate`. Each tool implements the `agent.Tool` interface.
- Add `internal/agent/patterns/` with three wrappers around `agent.Loop`: ReAct, Plan-and-Execute, and Reflection.
- Add `cmd/wiki-mcp/` — a stdio-transport MCP server exposing `wiki/` as a resource and `search_notes` as a tool. Hand-rolled JSON-RPC (`net/http` style, over stdin/stdout) or `mark3labs/mcp-go` if the transport is small enough.
- Add `.claude/mcp.json` registering the MCP server, and enable the `/wiki-search` slash command in Claude Code.
- Add unit/table-driven tests for the loop, tool dispatch, pattern wrappers, and MCP server JSON-RPC shape.
- No breaking changes — additive `internal/agent/`, `cmd/wiki-mcp/`, and config files only.

## Capabilities

### New Capabilities

- `mp14-agent-loop`: The core agent loop — `Loop.Run`, max turns, retry/backoff, tool-call dispatch, streaming events, and `Turn` history.
- `mp15-test-tools`: Built-in test tools (`get_weather`, `search_wiki`, `calculate`) and the `agent.Tool` interface.
- `mp16-agent-patterns`: ReAct, Plan-and-Execute, and Reflection wrappers around `agent.Loop`.
- `mp17-mcp-server`: Stdio MCP server `cmd/wiki-mcp/` exposing `wiki/` as a resource and the `search_notes` tool.
- `mp18-slash-command`: Claude Code MCP registration (`.claude/mcp.json`) and the `/wiki-search` slash command.

### Modified Capabilities

- (No existing spec requirements are changed. Phase 3 consumes `domain.Tool`, `domain.ToolCall`, and `outbound.Provider` from earlier phases as-is.)

## Impact

- **New files**:
  - `internal/agent/loop.go`, `internal/agent/types.go`, `internal/agent/errors.go` — core loop and types.
  - `internal/agent/tools/tools.go`, `internal/agent/tools/weather.go`, `internal/agent/tools/wiki.go`, `internal/agent/tools/calculate.go` — tool interface and implementations.
  - `internal/agent/patterns/react.go`, `internal/agent/patterns/plan.go`, `internal/agent/patterns/reflection.go` — pattern wrappers.
  - `cmd/wiki-mcp/main.go` + JSON-RPC transport code (or `mark3labs/mcp-go` wiring).
  - `.claude/mcp.json` — MCP server registration.
- **Dependencies**: likely `github.com/mark3labs/mcp-go` if hand-rolling proves error-prone; otherwise only stdlib. Decision recorded in `design.md`.
- **No changes** to `internal/core/domain/`, `internal/core/ports/`, or `internal/adapters/`.
- **Enables** Phase 3 close and LinkedIn post #3: "I built an agent loop in Go — 150 lines, zero frameworks".
