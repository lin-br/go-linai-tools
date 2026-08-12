## ADDED Requirements

### Requirement: ReAct wrapper adds reasoning prompt around the loop

The system SHALL provide `internal/agent/patterns/react.go` with a function `RunReAct(ctx context.Context, loop *agent.Loop, query string) (string, []agent.Turn, error)`. The wrapper SHALL prepend a system prompt instructing the model to reason step-by-step in a `Reasoning` field before choosing an action, and SHALL call `loop.Run(ctx, augmentedQuery)` where `augmentedQuery` combines the original query and the ReAct framing instructions. The returned `[]agent.Turn` SHALL include all turns produced by the underlying loop.

#### Scenario: ReAct prompt is prepended

- **WHEN** `RunReAct(ctx, loop, "what is the weather in São Paulo?")` is invoked
- **THEN** the query passed to `loop.Run` SHALL contain a ReAct instruction such as "Think step by step" or "Reasoning:" followed by "Action:"

#### Scenario: ReAct returns loop result

- **WHEN** the underlying loop returns `"Sunny, 28°C"` and a history of three turns
- **THEN** `RunReAct` SHALL return `"Sunny, 28°C"`, the same three turns, and a nil error

### Requirement: Plan-and-Execute wrapper generates a plan before execution

The system SHALL provide `internal/agent/patterns/plan.go` with a function `RunPlanAndExecute(ctx context.Context, loop *agent.Loop, planner outbound.Provider, query string) (string, []agent.Turn, error)`. The wrapper SHALL first ask `planner` (any `outbound.Provider`) to produce a numbered plan as structured text, then pass the original query plus the plan to `loop.Run`. The planner request SHALL use the same `domain.ChatRequest` shape as the loop provider.

#### Scenario: Plan is generated and included in execution

- **WHEN** `RunPlanAndExecute` is called with a query and the planner returns a plan containing steps "1. Search wiki. 2. Summarize."
- **THEN** the query passed to `loop.Run` SHALL contain the original query and the generated plan

#### Scenario: Planner error is returned immediately

- **WHEN** the planner provider returns an error before execution begins
- **THEN** `RunPlanAndExecute` SHALL return an empty string, nil turns, and the planner error wrapped with context

### Requirement: Reflection wrapper drafts, critiques, and finalizes

The system SHALL provide `internal/agent/patterns/reflection.go` with a function `RunReflection(ctx context.Context, loop *agent.Loop, query string) (string, []agent.Turn, error)`. The wrapper SHALL make up to two calls to `loop.Run`: first to produce a draft answer, then to critique and improve the draft. The final call's answer SHALL be returned as the result. The wrapper SHALL respect `ctx` cancellation between calls and SHALL not leak goroutines.

#### Scenario: Reflection produces a final improved answer

- **WHEN** `RunReflection(ctx, loop, "explain agent loops")` is called and the first loop call returns draft "A loop calls a model." and the second returns "An agent loop repeatedly calls a model, executes tools, and returns a final answer."
- **THEN** the function SHALL return the second answer and a combined turn history from both loop calls

#### Scenario: Context cancellation stops reflection

- **WHEN** `ctx` is cancelled after the first loop call but before the second
- **THEN** `RunReflection` SHALL return `ctx.Err()` without making the second loop call

### Requirement: Pattern wrappers are independent and composable

Each pattern wrapper SHALL accept an `*agent.Loop` pointer and a query string, and SHALL NOT modify the loop's internal state beyond what a normal `Run` call would do. The wrappers SHALL be testable with a fake loop that implements a minimal interface.

#### Scenario: Pattern does not corrupt loop state

- **WHEN** the same `*agent.Loop` instance is used by two different pattern wrappers in sequence
- **THEN** the second wrapper SHALL observe the same default loop behavior as the first (i.e., no leftover state from the first wrapper)

#### Scenario: Pattern wrapper uses loop interface for testability

- **WHEN** a pattern wrapper accepts any value whose type has a `Run(context.Context, string) (string, []agent.Turn, error)` method
- **THEN** the wrapper can be tested with a hand-written stub without constructing a real `*agent.Loop`
