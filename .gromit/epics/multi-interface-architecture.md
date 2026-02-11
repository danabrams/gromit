---
epic_id: multi-interface-architecture
created: 2026-02-11
---

# Multi-Interface Architecture

## Problem

Gromit's business logic is embedded in CLI command handlers. Users must start and exit Claude sessions repeatedly, manually poll for status with re-run commands, and shepherd ideas through the refine → plan → decompose → run pipeline by invoking separate CLI commands. There is no persistent dashboard, no way to monitor a run while exploring a new idea, and no path to non-CLI interfaces (TUI, web, mobile).

## Vision

Decompose Gromit into a **backend library** (`internal/pipeline/`) and **thin frontend clients** that communicate through a shared Go API. The library owns all workflow orchestration, state management, and event emission. Each frontend — CLI, TUI, and eventually an API server — is a presentation adapter that maps user interactions to library calls and library events to visual output.

## Architecture

```
internal/pipeline/        ← workflow orchestration, structured I/O, events
internal/events/          ← event types and emitter interface
cmd/gromit/               ← CLI client (flags → pipeline → text)
internal/tui/             ← TUI client (bubbletea → pipeline → panels)
cmd/gromitd/              ← API server (HTTP/WS → pipeline → JSON)  [future]
```

### Key Design Decisions

1. **Library-first** — The pipeline library is the source of truth. The CLI becomes a thin client, same as the TUI. No interface gets privileged access.

2. **Event-driven** — Long-running operations (run loop, decompose, retro) emit structured events rather than writing to stdout. Frontends subscribe to events and render them however they want.

3. **Graduated complexity** — Start with library extraction + CLI refactor. Build TUI on the same library. Later, wrap the library in an API server for web/mobile.

4. **Interactive sessions stay conversational** — Explore and refine use Claude CLI in streaming JSON mode, presented as a chat panel in the TUI rather than an embedded terminal.

### What the TUI Provides

- **Live dashboard** — run progress, queue depth, completed beads, current iteration. No more polling with `gromit status`.
- **Pipeline view** — see ideas flow through backlog → refined → planned → decomposed → running → done.
- **Conversation panel** — chat interface for explore/refine sessions, with Claude's tool use shown as status indicators.
- **Concurrent activities** — monitor a run while exploring a new idea in a split view.

## Phases

### Phase 1: Pipeline Extraction
Extract workflow logic from `cmd/gromit/` into `internal/pipeline/` with clean input structs, result structs, and context-based cancellation. Refactor CLI commands to be thin wrappers. This phase improves testability and code organization even without a TUI.

### Phase 2: Event System
Add structured event emission to runner and pipeline operations. Replace `fmt.Fprintf` calls with typed events. Make heartbeat and TMUX integration conditional/pluggable.

### Phase 3: TUI Foundation
Basic bubbletea application with panel layout, keyboard navigation, and a live dashboard view that subscribes to runner events.

### Phase 4: TUI Workflows
Add conversation panel for explore/refine (backed by Claude CLI streaming), pipeline management for plan/decompose, and queue/board views.

### Phase 5: API Server (Future)
Thin HTTP/WebSocket server wrapping the pipeline library. Web and mobile clients connect here.

## Current Coupling Assessment

**Already clean** (no changes needed): `bead`, `state`, `config`, `review` packages. Runner interfaces (`BeadClient`, `ClaudeClient`, `FailureAnalyzer`, `PromptRenderer`, `IterationLogger`).

**Needs extraction**: `refine.go` (310 lines), `plan.go` (266 lines), `decompose.go` (466 lines), `review.go` (602 lines) — all mix user interaction, workflow orchestration, and presentation.

**Needs abstraction**: Runner heartbeat (assumes TTY), TMUX integration (assumes terminal), chaining system (assumes subprocess CLI invocation).
