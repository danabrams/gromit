---
id: explore-model-forwarding-non-claude
source_ideas: []
created: 2026-02-27
accepted: true
---

# Explore Model Forwarding For Non-Claude Agents

## Specification

`gromit explore` should honor the user's explicit model choice across non-Claude interactive agents instead of silently dropping it.

When a user runs `gromit explore --model <value>`, the selected agent launch should receive that model intent whether the selected agent is `codex`, `gemini`, or a custom agent definition. If the selected agent cannot accept a model override at launch, `gromit` should continue the explore session and emit a clear warning that model forwarding was skipped for that agent.

Scope for this spec is limited to the `explore` command's interactive launch path:
- CLI parsing and propagation of `ExploreInput.Model`
- Agent launch argument construction for non-Claude presets
- Best-effort model forwarding for custom agent definitions
- Warning-and-continue behavior when forwarding is unsupported

Out of scope:
- `gromit run` model routing, escalation, and provider selection
- Adding or changing global per-command model defaults
- Refine/plan/debug/review model override semantics

### Behavior Rules

1. `gromit explore --model <x>` must attempt to pass `<x>` into the selected non-Claude agent launch path.
2. If `--model` is omitted, existing explore defaults remain unchanged.
3. If model forwarding is unsupported for the selected agent, `gromit` prints a warning identifying the agent and continues launching without the model override.
4. Custom agents are treated as best-effort for model forwarding with the same warning-and-continue fallback.

## Acceptance Criteria

- Running `gromit explore --agent codex --model gpt-5.3-codex "topic"` results in a Codex launch attempt that includes the selected model instead of ignoring `ExploreInput.Model`.
- Running `gromit explore --agent gemini --model gemini-2.5-pro "topic"` results in a Gemini launch attempt that includes the selected model instead of ignoring `ExploreInput.Model`.
- Running `gromit explore --agent <custom> --model <x> "topic"` attempts to forward model `<x>`; if unsupported, a warning is emitted and explore still launches.
- `gromit explore --model <x>` with Claude-selected flows remains compatible with existing behavior.
- Existing explore command delegation coverage still passes, and new tests cover model propagation for Codex/Gemini plus warning fallback behavior for unsupported/custom agent launches.

## Decisions

1. **Explore-only scope first**  
This spec intentionally isolates the explore path to close the immediate gap where `ExploreInput.Model` is captured but not forwarded.

2. **Best-effort custom-agent support**  
Custom agents vary in CLI surface area, so model forwarding should be attempted but not allowed to block explore sessions.

3. **Warning over hard failure**  
Interactive exploration should remain available even when model override cannot be applied for a specific agent.

## Research & Context

### Current State

- [`cmd/gromit/explore.go`](/home/dabrams/gromit/.-gromit-refine-1772157495324595675/cmd/gromit/explore.go) captures `--model` and assigns it to `pipeline.ExploreInput.Model`.
- [`internal/pipeline/explore.go`](/home/dabrams/gromit/.-gromit-refine-1772157495324595675/internal/pipeline/explore.go) resolves the selected agent and launches it via `LaunchInDir(promptPath, "")` without using `ExploreInput.Model`.
- [`internal/agent/resolve.go`](/home/dabrams/gromit/.-gromit-refine-1772157495324595675/internal/agent/resolve.go) has static preset construction for `codex` and `gemini`; no explore-time model injection path is currently wired.

### Related Specs

- [`interactive-agent-model-overrides.md`](/home/dabrams/gromit/.-gromit-refine-1772157495324595675/.gromit/specs/interactive-agent-model-overrides.md) tracks broader multi-command model override behavior. This spec is a focused slice that addresses the explore-specific defect first.
