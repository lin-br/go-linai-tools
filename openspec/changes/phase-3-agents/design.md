## Context

Phase 1–2 established the provider boundary (`outbound.Provider` with `Chat` and `ChatStream`), domain primitives (`domain.Tool`, `domain.ToolCall`, `domain.Message`), and the `tools.Extract[T]` helper for forced tool-choice extraction. Phase 3 turns those primitives into an agent: a loop that calls a model, executes any tool calls it requests, appends tool results, and repeats until the model returns a final answer or a safety limit is hit.

The north-star milestone for Phase 3 is a Claude Code slash command calling a Go MCP server. That means the phase must deliver not just a library loop, but a runnable `cmd/wiki-mcp` binary that speaks stdio JSON-RPC, registers a `wiki/` resource and a `search_notes` tool, and is wired into Claude Code via `.claude/mcp.json` as `/wiki-search`.

## Goals / Non-Goals

**Goals:**
- Provide a reusable `internal/agent/loop.go` package with `Loop.Run(ctx, query) (string, []Turn, error)` and `Loop.RunStream(ctx, query) (<-chan domain.StreamEvent, error)`.
- Enforce a hard max-turn limit (~10), retry failed provider calls with exponential backoff, and never leak the streaming goroutine.
- Define a small `agent.Tool` interface so tools are independently testable.
- Ship three test tools in `internal/agent/tools/`: `get_weather`, `search_wiki`, and `calculate`.
- Ship three pattern wrappers in `internal/agent/patterns/`: ReAct, Plan-and-Execute, and Reflection. Each wraps `agent.Loop`, not replaces it.
- Ship `cmd/wiki-mcp/`, a stdio MCP server exposing `wiki/` as a resource and `search_notes` as a tool.
- Register the server in `.claude/mcp.json` and enable the `/wiki-search` slash command.
- Cover the loop, tool dispatch, patterns, and MCP handlers with table-driven tests.

**Non-Goals:**
- Persistent agent memory or conversation storage (stateless per `Run` call).
- Multi-step planning with replanning on failure (the Plan-and-Execute wrapper is intentionally simple).
- Parallel tool execution (tools execute serially; this keeps channel ownership simple).
- HTTP/SSE MCP transport (stdio only).
- Real-time weather/wiki APIs (test tools return deterministic stubs or simple heuristics).
- Authentication, PII filtering, or cost tracking (Phase 5 concerns).
- Streaming of tool-call argument deltas (we stream text/events; tool calls are returned complete like `tools.Extract`).

## Decisions

### D1: `Loop.Run` returns `(finalAnswer string, turns []Turn, err error)`

`Run` is the blocking, non-streaming entry point. It returns the model's final text answer, the full `[]Turn` history, and an error. `Turn` captures role (`assistant`/`tool`) and content so callers can log, eval, or resume conversations.

**Why:** Returning the full history makes the loop observable and testable. A function that only returns `string` hides the reasoning trail, which is essential for debugging agent behavior and for writing evals.

**Alternative considered:** Return only `string`. Rejected — history is needed for tests and for future memory features.

### D2: Streaming is a separate method `RunStream` that returns a read-only channel

`RunStream(ctx, query) (<-chan domain.StreamEvent, error)` starts the loop in a goroutine and streams `domain.StreamEvent` values (text deltas, tool-call start/end, errors) to the caller. The channel is closed when the loop finishes. Cancellation propagates via `ctx`.

**Why:** Keeps `Run` simple and synchronous while still enabling UX that shows "thinking" or "searching..." states. The channel direction (`<-chan`) enforces that only the loop writes.

**Alternative considered:** Add a callback interface (`OnStreamEvent`). Rejected — callbacks invert control and make testing harder; channels are the idiomatic Go mechanism.

### D3: Tool interface is `Execute(ctx context.Context, args json.RawMessage) (string, error)`

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() map[string]any
    Execute(ctx context.Context, args json.RawMessage) (string, error)
}
```

The loop unmarshals each `domain.ToolCall.Arguments` into `json.RawMessage` and passes it to the matching tool. The tool returns a string that is appended to the conversation as a tool result message.

**Why:** `json.RawMessage` gives tools full control over parsing and typed errors, while keeping the loop agnostic of each tool's input shape. Returning `string` matches the OpenAI/Anthropic tool-result format and avoids inventing a new domain type.

**Alternative considered:** Generic `Execute[T any](input T)` or `map[string]any`. Rejected — generics would require the loop to know the tool's input type, and `map[string]any` loses type safety for tool implementers.

### D4: Tools execute serially inside each assistant turn

If the model requests multiple tool calls in one assistant message, the loop executes them one at a time, appending each result before moving to the next provider call.

**Why:** Serial execution keeps the `Turn` history deterministic and avoids concurrency bugs in a learning project. Most test prompts produce one tool call anyway.

**Alternative considered:** Parallel tool execution with `errgroup.SetLimit(n)`. Rejected — adds complexity (ordering, partial failures, context cancellation) without a demonstrated need in Phase 3. The tool interface and message format are compatible with parallel execution later.

### D5: Retry with exponential backoff lives in the loop, not a provider wrapper

The loop retries individual `provider.Chat` calls on transient errors (network blips, 429, 5xx). Backoff uses a small `cenkalti/backoff` wrapper or a stdlib-based exponential backoff (configurable via `LoopOptions`). Retries stop on `ctx.Done()`.

**Why:** Phase 2's `RetryProvider` decorates the whole provider. The agent loop benefits from per-call retry because a single failed tool-following request should not abort the entire conversation. Keeping it inside the loop also lets the retry observe `ctx` and the current turn count.

**Alternative considered:** Reuse `RetryProvider` from Phase 1. Rejected — that wrapper retries every provider call uniformly and does not know it is inside an agent loop. A loop-local retry with turn-aware logging is cleaner.

### D6: Max turns is a hard cap; final message on exhaustion

`Loop` carries `MaxTurns int` (default 10). Each iteration increments a turn counter. If the limit is reached without a final answer, `Run` returns a non-nil error (`ErrMaxTurnsExceeded`) along with whatever history accumulated.

**Why:** Prevents runaway loops from burning tokens or hanging. Returning an error rather than a truncated answer makes the failure explicit.

**Alternative considered:** Return partial answer with an error. Rejected — the model may be mid-tool-chain; returning a half-finished message is misleading.

### D7: Patterns are thin wrappers, not loop internals

ReAct, Plan-and-Execute, and Reflection live in `internal/agent/patterns/`. Each accepts an `*agent.Loop` and a query, prepends a system/structure prompt, and returns the loop's result. They do not modify `Loop`'s internals.

**Why:** Keeps `Loop` small (~150 lines) and lets patterns be mixed, compared, and tested independently. The "zero frameworks" claim in the LinkedIn post depends on this separation.

**Alternative considered:** Embed pattern logic inside `Loop` via a `Pattern` enum. Rejected — would bloat the loop and make patterns hard to test in isolation.

### D8: MCP server uses `mark3labs/mcp-go`

`cmd/wiki-mcp/` uses `github.com/mark3labs/mcp-go` for the stdio server, resource, and tool registration. The library is small, aligns with the curated resources list, and removes the risk of hand-rolling JSON-RPC edge cases (notification IDs, batch requests, content blobs).

**Why:** The roadmap says "hand-rolled JSON-RPC ~200 lines" is acceptable, but the same line says `mark3labs/mcp-go` is an option. Using the library keeps the focus on the agent/MCP concepts rather than JSON-RPC parsing. The transport remains stdio and the server remains a single binary.

**Alternative considered:** Hand-roll JSON-RPC over stdin/stdout. Rejected — error-prone and distracts from the agent learning goal. If the dependency proves heavy, it can be replaced later without changing the resource/tool surface.

### D9: `search_notes` reads from a configurable wiki directory

The `search_notes` tool scans `.md` files under a directory configured via `--wiki-dir` (default `./wiki`). It performs a simple substring match (case-insensitive) across file content and returns matching file paths with snippets. No vector search or BM25 in Phase 3.

**Why:** Matches the Phase 3 scope. A real RAG pipeline is Phase 2/5; the point here is MCP wiring, not search quality.

**Alternative considered:** Use the Phase 2 hybrid search. Rejected — Phase 2 may not be complete yet, and the MCP server must stand alone.

### D10: `.claude/mcp.json` lives at repo root

Claude Code discovers MCP servers via `.claude/mcp.json`. The file registers `wiki-mcp` with command `go run ./cmd/wiki-mcp` and env vars needed at runtime.

**Why:** Standard Claude Code convention. Keeping it in the repo makes the slash command reproducible across machines.

## Risks / Trade-offs

- **[Streaming goroutine leak]** → `RunStream` must close the returned channel on every path. Mitigation: use `defer close(ch)`, propagate `ctx.Done()`, and add `go.uber.org/goleak` to streaming tests.
- **[Tool call argument parsing drift across providers]** → OpenRouter normalizes tool calls, but argument JSON shape can vary. Mitigation: `Execute` receives `json.RawMessage` and each tool decodes defensively; tests use realistic JSON.
- **[MCP stdio lifecycle]** → Claude Code kills/restarts the stdio process. Mitigation: server is stateless and handles `initialize`/`initialized` correctly via the library; no persistent connections.
- **[Forced tool choice not supported by some OpenRouter models]** → The loop asks the model to use tools but does not force `tool_choice: any` on every turn. Mitigation: system prompt instructs tool use; if the model returns plain text mid-loop, treat it as the final answer or loop again with a reminder.
- **[Reflection pattern may loop]** → Reflection calls the loop twice (draft → critique → final). Mitigation: hard max-turns cap still applies; reflection wrapper tracks its own iteration budget.

## Migration Plan

- Additive only. No existing `internal/core` packages change.
- After merge, run:
  ```bash
  go build ./...
  go test ./internal/agent/...
  go run ./cmd/wiki-mcp --help
  ```
- Claude Code users reload MCP with `/mcp` or restart Claude Code after `.claude/mcp.json` is present.

## Open Questions

- Should `RunStream` return assistant-text deltas as they arrive from `ChatStream`, or only high-level events (`event_tool_start`, `event_text`)? Decision deferred to implementation; spec requires at least text and tool-start/end events.
- Should the loop expose a hook for "before tool execution" so callers can implement human-in-the-loop gates? Deferred — not needed for Phase 3, but the `Tool` interface leaves room for it.
