## 1. Project scaffold

- [ ] 1.1 Create `ux-lab/` directory at the repo root
- [ ] 1.2 Create `ux-lab/index.html` with landing page skeleton, pattern navigation, and chat demo container
- [ ] 1.3 Create `ux-lab/styles.css` with base layout, message styles, pattern cards, and stateful component classes
- [ ] 1.4 Create `ux-lab/app.js` entry point with mock backend, stream consumer, and UI controllers
- [ ] 1.5 Create `ux-lab/README.md` describing how to run the prototype and the 7 patterns

## 2. Mock backend and SSE simulation

- [ ] 2.1 Implement `MockEventSource` class in `ux-lab/app.js` that emits `data:` events from a scripted array
- [ ] 2.2 Implement `createMockStream(flowId, signal)` that returns a `MockEventSource` and respects `AbortController` cancellation
- [ ] 2.3 Define scripted flows: `grocery-april` (progressive disclosure + citations), `delete-april` (human-in-the-loop + confirmation gate), `summarize-failed` (graceful degradation + retry), and `cancel-demo` (cancellation)
- [ ] 2.4 Emit event types: `delta`, `tool_start`, `tool_result`, `tool_complete`, `error`, `done`

## 3. Core chat UI

- [ ] 3.1 Implement `sendMessage(prompt)` that creates an `AbortController`, appends a user message, and starts a mock stream
- [ ] 3.2 Implement message rendering for user, assistant streaming, tool pending, tool completed, error, and cancellation states
- [ ] 3.3 Implement "Stop" button wired to `controller.abort()` and rendering of "Response cancelled"
- [ ] 3.4 Implement input box with prompt history so retry can repopulate the previous prompt

## 4. Pattern demonstrations

- [ ] 4.1 Implement progressive disclosure: stream text first, then reveal tool results, then reveal source footnotes
- [ ] 4.2 Implement human-in-the-loop: render confirmation card for high-stakes actions and pause flow until Confirm/Cancel
- [ ] 4.3 Implement confirmation gates: render gate for destructive tools showing tool name and arguments
- [ ] 4.4 Implement streaming actions: render animated pending cards with human-readable labels
- [ ] 4.5 Implement graceful degradation: render fallback card with partial answer and retry affordance on mock errors
- [ ] 4.6 Implement citation/provenance: render numbered clickable citations and source cards with excerpts and confidence badges
- [ ] 4.7 Implement cancellation + retry: halt stream on stop, allow retry with editable previous prompt

## 5. Pattern navigation and isolated views

- [ ] 5.1 Add 7 pattern cards to `index.html` landing page
- [ ] 5.2 Add isolated demo sections or anchors for each pattern
- [ ] 5.3 Add anti-pattern callout for each pattern card explaining what the bad UX looks like

## 6. Anti-patterns documentation

- [ ] 6.1 Document anti-pattern: spinner with no progress
- [ ] 6.2 Document anti-pattern: "I can't do that" with no alternative
- [ ] 6.3 Document anti-pattern: hallucinated buttons
- [ ] 6.4 Document anti-pattern: raw JSON in chat
- [ ] 6.5 Document anti-pattern: silent destructive actions
- [ ] 6.6 Document anti-pattern: unverified claims
- [ ] 6.7 Document anti-pattern: unrecoverable error states
- [ ] 6.8 Add anti-patterns section to `ux-lab/README.md`

## 7. Concept write-up

- [ ] 7.1 Create `docs/ai-ux-patterns.md` summarizing each pattern and anti-pattern in prose
- [ ] 7.2 Link `docs/ai-ux-patterns.md` to the roadmap and Phase 7 deliverables

## 8. Verification

- [ ] 8.1 Open `ux-lab/index.html` in a browser and confirm the landing page renders without console errors
- [ ] 8.2 Run the `grocery-april` flow and verify progressive disclosure + citations
- [ ] 8.3 Run the `delete-april` flow and verify human-in-the-loop + confirmation gate
- [ ] 8.4 Run the `summarize-failed` flow and verify graceful degradation + retry
- [ ] 8.5 Click "Stop" during streaming and verify cancellation behavior
- [ ] 8.6 Verify that no HTTP requests are made to external LLM endpoints
- [ ] 8.7 Run `go build ./...` and `go test ./...` to confirm no Go code was broken
