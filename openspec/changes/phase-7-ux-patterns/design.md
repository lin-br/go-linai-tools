## Context

Phase 7's deliverable is a learning prototype, not production UI. The goal is to internalize the UX patterns that make AI products feel robust: progressive disclosure, human-in-the-loop, confirmation gates, streaming actions, graceful degradation, citation/provenance, and cancellation + retry.

The `go-linai-tools` monorepo is Go-first and backend-focused, but this microphase is explicitly UI/UX exploration. The prototype is a static site (HTML/CSS/JS) that runs from `ux-lab/index.html` via any static file server. It mocks the LLM and tool layer so the patterns can be studied without API keys, network latency, or backend wiring.

The patterns will later be reused in Phase 8 (Finanças IA) `cmd/financas-ia/` frontend, so the design should keep the HTML/CSS/JS structure clean enough to port.

## Goals / Non-Goals

**Goals:**
- Provide a single runnable `ux-lab/` directory that demonstrates all 7 patterns.
- Simulate a chat UI with mocked streaming text, tool calls, and errors.
- Show each pattern both in isolation (pattern cards/views) and in a unified chat flow.
- Keep the prototype dependency-free: no build step, no npm, no framework.
- Make the mock SSE/streaming behavior realistic enough to reason about production equivalents.
- Document anti-patterns for each of the 7 patterns.

**Non-Goals:**
- Hook the prototype to a real LLM or Go backend.
- Implement accessibility to production-grade standards (basic semantics only).
- Add tests (this is a throwaway learning artifact; manual verification is sufficient).
- Ship to a public host or add CI.
- Use React/Vue/Svelte; vanilla JS only.
- Persist state across reloads.

## Decisions

### D1: Static HTML/CSS/JS, no build system

`ux-lab/` is a vanilla static site. Files are served via `python3 -m http.server` or `npx serve`.

**Why:** The roadmap's learning objective is the patterns, not a frontend toolchain. A build step would add noise and delay.

**Alternative considered:** Use Vite + React. Rejected — overkill for a single-week learning prototype.

### D2: Mocked SSE with `EventSource` over a local EventSource polyfill

The prototype's `app.js` exposes `createMockStream()` that returns an `EventSource`-like object emitting `data:` events from a scripted array. The chat UI consumes the same events it would consume from a real `/chat` SSE endpoint.

**Why:** It lets us demonstrate streaming, cancellation, and tool-call state transitions without a real server. The consumer code is structurally identical to a real SSE client.

**Alternative considered:** Use `setInterval` directly inside the UI. Rejected — it hides the SSE abstraction and makes the streaming/cancellation pattern harder to see.

### D3: One unified chat demo plus isolated pattern views

`index.html` contains:
- A nav of 7 pattern cards, each linking to an isolated demo.
- A full chat demo that stitches patterns into a single conversation flow.

**Why:** Developers learn patterns both in isolation (to understand the mechanism) and in flow (to understand sequencing). The unified demo is the LinkedIn screenshot moment.

**Alternative considered:** One page per pattern. Rejected — a single `index.html` with anchors keeps the artifact small and easy to run.

### D4: State-driven UI classes

Each component state is represented by CSS classes on a container (e.g. `.message-streaming`, `.tool-pending`, `.confirmation-open`). JavaScript toggles classes and injects content; animations and layout are CSS-driven.

**Why:** It separates behavior (JS) from presentation (CSS), making the code easier to port to a framework later.

### D5: Cancellation simulated with an `AbortController`

The chat input creates an `AbortController` and passes its signal to the mock stream. Clicking "Stop" calls `abort()`, which the mock stream respects by ending event emission and rendering "Response cancelled."

**Why:** Mirrors the real Go `context.Cancel` → abort LLM flow described in the roadmap. Keeps the mental model consistent across frontend and backend.

### D6: Retry loads edited prompt, not original

When a request fails or is cancelled, the retry affordance puts the previous prompt in the input box, lets the user edit it, and resubmits.

**Why:** The roadmap explicitly calls for "retry with edited prompt," not a blind resend. This surfaces the pattern in the UI.

## Risks / Trade-offs

- **[Static-only means no real timing]** → The mock stream timings are hardcoded. The trade-off is acceptable because the goal is pattern demonstration, not performance tuning.
- **[No real citations]** → Sources are mocked strings. In Phase 8 this will be replaced by real chunk IDs/source paths from the RAG layer.
- **[Manual verification only]** → Without automated tests, regressions are possible. Mitigation: keep the file count low and verify by opening `index.html` in a browser.
- **[Browser compatibility]** → Uses modern APIs (`AbortController`, `EventSource`). Mitigation: target latest Chrome/Firefox/Safari; this is a local prototype, not a public product.
