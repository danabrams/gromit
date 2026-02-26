---
id: debug-model-flag-warning
source_ideas: []
created: 2026-02-26
epic: provider-ecosystem
---

# Debug --model Flag Warning for Non-Claude Agents

## Problem

The `debug --model` flag silently has no effect when the selected agent is not Claude. `shouldOverrideDebugModel()` returns false for non-Claude agents, and the override always creates a Claude agent regardless of the active provider. Users who set `--model` while running a non-Claude agent get no feedback that their flag is being ignored.

## Approach

- In the `debug` command's model-override path (wherever `shouldOverrideDebugModel()` is evaluated), detect when `--model` is set but the active agent is not Claude
- Print a warning to stderr: `warning: --model flag is only supported for the Claude agent; ignoring for <agent-name>`
- No behavior change: the flag continues to be a no-op for non-Claude agents; this is documentation-level feedback only
- Do not modify the flag's help text (it already documents the limitation); the warning is a runtime signal for users who miss the docs

## Files to Change

- `cmd/gromit/debug.go` — add warning emission when `--model` is set and active agent is not Claude

## Acceptance Criteria

- When `--model` is provided and the active agent is not Claude, a warning is printed to stderr before the debug session starts
- Warning text names the active agent so the user understands which provider is running
- No warning is emitted when `--model` is not set, or when the active agent is Claude
- Existing debug command behavior is unchanged for Claude agents
- No new flags or config fields are introduced
