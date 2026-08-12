## 1. Agent loop core (MP14)

- [ ] 1.1 Create `internal/agent/types.go` — define `Tool` interface, `Turn` struct, `Options` struct, and sentinel errors (`ErrMaxTurnsExceeded`, `ErrToolNotFound`)
- [ ] 1.2 Create `internal/agent/loop.go` — define `Loop` struct, `NewLoop(provider, opts)`, `Register(tool)`, and `Run(ctx, query)` signature
- [ ] 1.3 Implement `Run` — build conversation history, include tools, call `provider.Chat`, execute tool calls serially, append results, enforce `MaxTurns`
- [ ] 1.4 Implement retry/backoff inside `Run` — exponential backoff with jitter, respect `ctx.Done()`, fail fast on non-retryable errors
- [ ] 1.5 Implement `RunStream(ctx, query) (<-chan domain.StreamEvent, error)` — spawn goroutine, stream text/tool-start/tool-end events, close channel on every exit path
- [ ] 1.6 Add `internal/agent/loop_test.go` — table-driven tests for single tool, multi tool, unknown tool, max turns, retry, context cancellation, and stream channel closure

## 2. Test tools (MP15)

- [ ] 2.1 Create `internal/agent/tools/weather.go` — implement `get_weather` tool with deterministic output and fallback for unknown locations
- [ ] 2.2 Create `internal/agent/tools/wiki.go` — implement `search_wiki` tool that scans markdown files under a configurable directory
- [ ] 2.3 Create `internal/agent/tools/calculate.go` — implement `calculate` tool with safe arithmetic expression evaluation
- [ ] 2.4 Create `internal/agent/tools/tools.go` — helper to build a default tool registry
- [ ] 2.5 Add `internal/agent/tools/*_test.go` — table-driven tests for each tool, including argument parsing, schema shape, and error cases

## 3. Agent patterns (MP16)

- [ ] 3.1 Create `internal/agent/patterns/react.go` — implement `RunReAct(ctx, loop, query)` that prepends a reasoning prompt and calls `loop.Run`
- [ ] 3.2 Create `internal/agent/patterns/plan.go` — implement `RunPlanAndExecute(ctx, loop, planner, query)` that generates a plan via `planner.Chat` and passes it to `loop.Run`
- [ ] 3.3 Create `internal/agent/patterns/reflection.go` — implement `RunReflection(ctx, loop, query)` that drafts, critiques, and finalizes in up to two loop calls
- [ ] 3.4 Add `internal/agent/patterns/*_test.go` — table-driven tests with fake loops and fake planner providers

## 4. MCP server (MP17)

- [ ] 4.1 Add `github.com/mark3labs/mcp-go` dependency (or decide to hand-roll JSON-RPC and document the decision)
- [ ] 4.2 Create `cmd/wiki-mcp/main.go` — parse `--wiki-dir` flag, start stdio MCP server, register `wiki/` resource and `search_notes` tool
- [ ] 4.3 Implement `wiki://{path}` resource read handler using the configured wiki directory
- [ ] 4.4 Implement `resources/list` handler returning all `.md` files under the wiki directory
- [ ] 4.5 Implement `search_notes` tool handler — case-insensitive substring search with snippets
- [ ] 4.6 Add `cmd/wiki-mcp/main_test.go` — table-driven tests for resource read, resource list, and search handlers using `os.MkdirTemp`
- [ ] 4.7 Run `go build ./cmd/wiki-mcp` and verify `--help` works

## 5. Slash command integration (MP18)

- [ ] 5.1 Create `.claude/mcp.json` registering `wiki-mcp` with `go run ./cmd/wiki-mcp`
- [ ] 5.2 Create `.claude/commands/wiki-search.md` with instructions to call `search_notes` on `wiki-mcp`
- [ ] 5.3 Update documentation (or add `docs/phase-3-slash-command.md`) explaining how to enable `/wiki-search` and restart Claude Code
- [ ] 5.4 Smoke test: run `go run ./cmd/wiki-mcp --help` and validate `.claude/mcp.json` JSON syntax

## 6. Verification

- [ ] 6.1 Run `go build ./...` — all packages compile
- [ ] 6.2 Run `go vet ./...` — no warnings
- [ ] 6.3 Run `go test ./internal/agent/... ./cmd/wiki-mcp/...` — all tests pass
- [ ] 6.4 Run `go test -race ./internal/agent/...` — no data races in loop/patterns
- [ ] 6.5 Manually test `go run ./cmd/wiki-mcp` with a temporary wiki directory and a `resources/read` request
- [ ] 6.6 Restart Claude Code, run `/wiki-search agent loop`, and verify it calls the local MCP server
