## ADDED Requirements

### Requirement: Static prototype lives in `ux-lab/`

The system SHALL provide a static HTML/CSS/JS prototype in the `ux-lab/` directory at the repo root. The prototype SHALL be runnable without a build step, a Go backend, or external LLM credentials. A user SHALL be able to open `ux-lab/index.html` directly or serve the directory with any static file server and see a working landing page.

#### Scenario: Open the prototype in a browser
- **WHEN** a developer opens `ux-lab/index.html` in a browser or runs `python3 -m http.server` inside `ux-lab/` and visits the served URL
- **THEN** the page SHALL render a heading, a navigation of 7 pattern cards, and a combined chat demo without JavaScript errors in the console

#### Scenario: No build step required
- **WHEN** a developer lists the contents of `ux-lab/`
- **THEN** the directory SHALL contain only static files (`index.html`, `styles.css`, `app.js`, optional `README.md`) and no `package.json`, `node_modules/`, or compiled output

### Requirement: Demonstrate progressive disclosure

The prototype SHALL demonstrate progressive disclosure by streaming assistant text first, then revealing tool-call results only after the tool completes, and finally revealing clickable source citations as footnotes. The user SHALL see content appear in stages rather than all at once.

#### Scenario: Chat demo streams then reveals sources
- **WHEN** the user sends a query that triggers the "What did I spend on groceries in April?" mock flow
- **THEN** the assistant message SHALL first show streaming text, THEN show a "Searching transactions..." tool-call card, THEN show the final answer, and THEN show numbered footnote links to sources

#### Scenario: Tool result is hidden until completion
- **WHEN** a tool call is in the pending state
- **THEN** the raw tool result JSON or structured data SHALL NOT be visible until the card transitions to the completed state

### Requirement: Demonstrate human-in-the-loop

The prototype SHALL demonstrate human-in-the-loop by pausing the assistant flow and rendering a confirmation card before executing a high-stakes action (e.g. "Delete all April transactions" or "Approve $500 transfer"). The user SHALL be required to click "Confirm" or "Cancel" before the flow continues.

#### Scenario: Confirmation card blocks the flow
- **WHEN** the mock flow reaches a high-stakes action
- **THEN** the UI SHALL display a confirmation card with action details, a "Confirm" button, and a "Cancel" button, and the assistant SHALL NOT proceed until one is clicked

#### Scenario: Confirm resumes the flow
- **WHEN** the user clicks "Confirm" on the confirmation card
- **THEN** the card SHALL close, the action SHALL be marked as executed, and the assistant SHALL continue with the next step

#### Scenario: Cancel aborts the action
- **WHEN** the user clicks "Cancel" on the confirmation card
- **THEN** the card SHALL close, the action SHALL be marked as declined, and the assistant SHALL provide a fallback message

### Requirement: Demonstrate confirmation gates

The prototype SHALL demonstrate confirmation gates by rendering a distinct gate for destructive tool calls. The gate SHALL show the exact tool name and arguments that will be sent, and SHALL require explicit user approval before the simulated tool is invoked.

#### Scenario: Destructive tool call shows gate
- **WHEN** the assistant intends to call a destructive tool such as `delete_transactions`
- **THEN** the UI SHALL render a confirmation gate showing the tool name, arguments (e.g. month, category), and explicit "Run `delete_transactions`" and "Cancel" buttons

#### Scenario: Gate distinguishes destructive from safe tools
- **WHEN** a safe tool (e.g. `search_transactions`) runs
- **THEN** the UI MAY show a lightweight pending indicator but SHALL NOT require the same explicit confirmation gate as a destructive tool

### Requirement: Demonstrate streaming actions

The prototype SHALL demonstrate streaming actions by showing animated pending indicators with descriptive labels (e.g. "Searching transactions...", "Summarizing statement...") while mock tool calls are in flight.

#### Scenario: Pending tool call shows streaming indicator
- **WHEN** a mock tool call begins
- **THEN** the UI SHALL display a pending card containing a spinner or progress animation and a human-readable label describing what is happening

#### Scenario: Completed tool call transitions to result
- **WHEN** the mock tool call completes
- **THEN** the pending card SHALL transition to a completed card showing the tool result or a summary of the result

### Requirement: Demonstrate graceful degradation

The prototype SHALL demonstrate graceful degradation by showing a useful fallback when a simulated tool fails or returns no data, instead of displaying a red error banner alone. The fallback SHALL include a partial answer, a suggestion, or a retry affordance.

#### Scenario: Tool failure shows fallback
- **WHEN** a mock tool call returns an error or no results
- **THEN** the UI SHALL render a fallback card containing a short explanation, a useful partial answer if available, and a "Retry" or "Try again" button

#### Scenario: No raw error dump
- **WHEN** a simulated error occurs
- **THEN** the UI SHALL NOT display a raw stack trace or JSON error blob to the end user

### Requirement: Demonstrate citation/provenance

The prototype SHALL demonstrate citation/provenance by rendering clickable source links in assistant messages. Each source SHALL be numbered and linked to a simulated source card that shows the source title, excerpt, and confidence indicator.

#### Scenario: Citation links in answer
- **WHEN** the assistant produces a final answer that references sources
- **THEN** the answer text SHALL contain superscript citation numbers that are clickable

#### Scenario: Source card opens on click
- **WHEN** the user clicks a citation number
- **THEN** the UI SHALL open or scroll to a source card showing the source title, excerpt, and a confidence badge (e.g. "High", "Medium")

#### Scenario: Sources list in sidebar or footer
- **WHEN** the chat demo renders a sourced answer
- **THEN** a list of all sources SHALL be visible in the message footer or a dedicated panel

### Requirement: Demonstrate cancellation + retry

The prototype SHALL demonstrate cancellation + retry by providing a "Stop" button during streaming, showing "Response cancelled" when stopped, and allowing the user to retry with an editable version of the previous prompt.

#### Scenario: Stop button cancels streaming
- **WHEN** the user clicks "Stop" while assistant text is streaming
- **THEN** the stream SHALL halt, the partial message SHALL remain, and a "Response cancelled" status SHALL appear

#### Scenario: Retry restores editable prompt
- **WHEN** the user clicks "Retry" after a cancelled or failed response
- **THEN** the previous prompt SHALL appear in the input box, editable, and the user SHALL be able to submit a new request

#### Scenario: Retry after failure
- **WHEN** a mock request fails (e.g. simulated 503 error)
- **THEN** the UI SHALL show a retry affordance and, when clicked, repopulate the input with the last prompt and start a new mock stream

### Requirement: Mock LLM and SSE behavior

The prototype SHALL use a JavaScript mock layer to simulate LLM responses and SSE streams. The mock layer SHALL return scripted events for each demo flow, including streaming text deltas, tool-call start/complete events, and error events. The UI consumer code SHALL be structurally identical to a real SSE client.

#### Scenario: Mock stream emits text deltas
- **WHEN** the chat demo starts a mock stream
- **THEN** the mock layer SHALL emit `delta` events carrying text fragments that the UI appends to the assistant message

#### Scenario: Mock stream emits tool-call events
- **WHEN** the mock flow includes a tool call
- **THEN** the mock layer SHALL emit `tool_start`, `tool_result`, and `tool_complete` events with tool name, arguments, and result payload

#### Scenario: Mock stream emits error events
- **WHEN** a flow is configured to fail
- **THEN** the mock layer SHALL emit an `error` event carrying a user-facing message and a retry flag

### Requirement: Anti-patterns documentation

The prototype SHALL document one anti-pattern for each of the 7 patterns. The anti-patterns SHALL be visible in the UI or in `ux-lab/README.md`, explaining what the bad UX looks like and why the corresponding pattern prevents it.

#### Scenario: Anti-patterns are listed
- **WHEN** the user opens the landing page or `ux-lab/README.md`
- **THEN** the system SHALL list the 7 anti-patterns: spinner with no progress, "I can't do that" with no alternative, hallucinated buttons, raw JSON in chat, silent destructive actions, unverified claims, and unrecoverable error states

#### Scenario: Each anti-pattern maps to a pattern
- **WHEN** the user clicks a pattern card
- **THEN** the system SHALL show the anti-pattern that the pattern is designed to prevent

### Requirement: No Go or real LLM dependency

The prototype SHALL NOT import any Go packages, call any real LLM API, or require backend environment variables. All data SHALL be hardcoded or generated by JavaScript mocks.

#### Scenario: No API keys required
- **WHEN** a developer opens `ux-lab/index.html` without setting `OPENROUTER_API_KEY` or any other env var
- **THEN** the prototype SHALL still render and all demos SHALL be functional

#### Scenario: No network calls to LLM providers
- **WHEN** the user interacts with the chat demo
- **THEN** the browser SHALL NOT make HTTP requests to `openrouter.ai`, `anthropic.com`, `openai.com`, or any other external LLM endpoint
