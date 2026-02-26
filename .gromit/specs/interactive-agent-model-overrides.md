---
id: interactive-agent-model-overrides
source_ideas: []
created: 2026-02-26
---

# Interactive Agent Model Overrides

## Specification

`gromit` should support consistent model selection across interactive agent-driven commands, not just Claude.

When a user runs an interactive command and specifies `--model`, Gromit must pass that model choice through to the selected agent regardless of agent type (Claude, Codex, Gemini, or custom agent definitions). This behavior must apply to commands that support interactive agent selection and launch flows.

In addition to per-invocation `--model`, Gromit should support config-based default model selection per interactive command so teams can set preferred defaults without repeating flags. These defaults apply only to interactive command sessions and do not affect run-loop model routing.

Supported scope for this spec:
- Interactive commands with agent selection: `refine`, `plan`, `explore`, `debug`, and interactive `review`
- Per-command default model configuration
- CLI override behavior for explicit `--model`
- Graceful warning-and-continue behavior when an agent does not accept model flags

Explicitly out of scope:
- The autonomous run loop (`gromit run`) and its routing/escalation model decisions
- Changing provider tier routing, escalation chain semantics, or methodology phase model selection
- Non-interactive `review` model/tier logic

### Behavior Rules

1. `--model` on interactive commands is universal
- If a user passes `--model`, Gromit attempts to launch the selected interactive agent with that model value.
- This applies even when the selected agent is non-Claude.

2. Config defaults per interactive command
- Gromit supports configuring default model values by command/phase for interactive agent commands.
- If `--model` is not provided, command-specific model defaults are used when configured.

3. Precedence
- Highest: explicit command-line `--model`
- Next: per-command interactive model default from config
- Lowest: existing built-in command default behavior

4. Unsupported model flags
- If the selected agent does not support model override at launch time, Gromit emits a warning and continues the session without model override instead of hard-failing.
- Warning output must clearly identify the agent and that model override was skipped.

5. Backward compatibility
- Existing configurations that do not set command-specific model defaults continue to work unchanged.
- Existing command behavior remains unchanged unless model override is explicitly requested or configured.

## Acceptance Criteria

- `gromit explore --agent codex --model <x>` attempts a Codex launch with model `<x>` instead of silently ignoring `ExploreInput.Model`.
- `gromit explore --agent gemini --model <x>` attempts a Gemini launch with model `<x>`.
- Interactive commands `refine`, `plan`, `explore`, `debug`, and interactive `review` all expose a `--model` override with consistent precedence behavior.
- A per-command interactive model default can be set in config and is used when `--model` is omitted.
- If `--model` is provided and the selected agent rejects model override, Gromit prints a warning and still opens/runs the session without model override.
- If both config default and `--model` are present, the CLI flag takes precedence.
- `gromit run` behavior and model routing remain unchanged by this feature.
- Existing tests for command/agent resolution continue to pass, and new tests cover: model propagation in `explore`, config-default model resolution, precedence ordering, and warning fallback behavior for unsupported agents.

## Decisions

1. **Universal model override across interactive agents**  
Model selection for interactive sessions should not be Claude-only. User intent is explicit and should be honored across Claude, Codex, Gemini, and custom agents.

2. **Warning-and-continue on unsupported agents**  
Interactive sessions should remain resilient. Unsupported model flags must not block the user from entering the session.

3. **Per-command model defaults, interactive-only**  
Defaults should map to interactive command workflows (`refine`, `plan`, `explore`, `debug`, interactive `review`) and intentionally exclude `run`, where model routing is already owned by runtime policy.

4. **CLI-first precedence**  
Explicit user input on the command line is authoritative and must override config defaults.

## Research & Context

### Current State

- `cmd/gromit/explore.go` captures `--model` and assigns it to `pipeline.ExploreInput.Model`.
- `internal/pipeline/explore.go` currently resolves an agent and calls `LaunchInDir(promptPath, "")` without using `ExploreInput.Model`, so model choice is lost.
- `cmd/gromit/debug.go` already demonstrates partial model override behavior, including warning text for non-Claude agents when the flag is ignored.
- `internal/config/config_types.go` currently has `agents.phases` for agent selection by command, but no parallel per-command model default field for interactive commands.

### Related Patterns

- Existing agent resolution precedence is already command-aware (`--agent`, picker, phase default, fallback).
- Interactive command architecture already centralizes launch points (`launch*Session` helpers), which is where consistent model override behavior must be observable.

### External CLI Capability Validation (local environment)

- `codex --help` documents `--model` support.
- Local Gemini CLI docs and source indicate startup `--model`/`-m` support.
- Custom agent support is unknown by default and must be treated as best-effort with warning fallback.
