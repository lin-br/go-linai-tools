## Why

Phase 7 of the AI Engineer roadmap is about the UX patterns that distinguish a reliable AI product from a fragile demo. Before integrating these patterns into the Finanças IA showcase, we need a cheap, static prototype that demonstrates each pattern in isolation, with mocked model responses and no real LLM dependency. This change delivers that prototype so we can iterate on interaction design and gather the LinkedIn post #7 artifacts without waiting for the backend.

## What Changes

- Add `ux-lab/` static HTML/CSS/JS prototype under the repo root, demonstrating all 7 AI-native UX patterns from the roadmap.
- Implement 7 standalone pattern views plus a demo chat flow that stitches the patterns together.
- Add a mocked backend layer in JavaScript that simulates streaming text, tool calls, and errors via `EventSource` and `setTimeout`.
- Document the anti-patterns that each pattern is designed to prevent.
- No Go code, no external backend, no real LLM calls — this is a browser-only learning artifact.

## Capabilities

### New Capabilities
- `mp33-ux-lab-prototype`: The `ux-lab/` static prototype — HTML/CSS/JS files, mocked LLM/SSE behavior, demo of all 7 AI-native UX patterns, and anti-pattern documentation.

### Modified Capabilities
- (No existing specs are modified. This is a self-contained static prototype that does not touch the Go packages, provider interfaces, or configuration.)

## Impact

- **New files**:
  - `ux-lab/index.html` — landing page with pattern navigation and combined chat demo.
  - `ux-lab/styles.css` — shared styles, responsive layout, stateful component styling.
  - `ux-lab/app.js` — mock backend, state management, SSE simulation, pattern controllers.
  - `ux-lab/patterns/` — optional individual pattern pages if `index.html` modularity is preferred.
  - `ux-lab/README.md` — how to run locally and what each pattern demonstrates.
  - `docs/ai-ux-patterns.md` — concept write-up covering the 7 patterns and anti-patterns.
- **No changes** to `internal/`, `cmd/`, `main.go`, or `go.mod`.
- **No new Go dependencies**.
- **No breaking changes**.
- **Enables** Phase 7 close and LinkedIn post #7.
