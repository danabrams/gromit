---
id: provider-neutral-wording
source_ideas: []
created: 2026-02-26
epic: provider-ecosystem
---

# Provider-Neutral Wording in User-Facing Messages

## Problem

Since multi-provider support was added, several user-facing runtime messages still say "Claude" where they mean "the active agent". This creates confusion for users running Codex, Gemini, or other providers who see Claude-specific language in heartbeat and log output.

## Approach

- Replace "Claude" with "agent" in heartbeat messages in `internal/runner/heartbeat.go` (e.g., "Waiting for Claude..." → "Waiting for agent...")
- Replace "Claude" with "agent" in handler log strings in `internal/runner/handler.go` that describe agent activity generically
- Keep flag help text that specifically references the Claude agent unchanged — text like "override model for Claude agent" is accurate and intentional
- Keep internal type names (`ClaudeAgent`, `claudeProvider`, etc.) unchanged — these are implementation identifiers, not user-facing messages
- Keep any error messages that specifically describe Claude CLI behavior (exit codes, stream JSON parsing) unchanged — those are Claude-specific facts

## Files to Change

- `internal/runner/heartbeat.go` — replace generic "Claude" references with "agent"
- `internal/runner/handler.go` — replace generic "Claude" references with "agent" in log strings

## Acceptance Criteria

- Heartbeat status messages do not contain "Claude" when referring to agent activity generically
- Handler log strings use "agent" where the message applies to any provider
- Flag help text that specifically describes the Claude agent limitation is unchanged
- Internal type names and struct fields are unchanged
- All existing tests pass without modification
