---
id: interactive-session-flags
source_ideas: []
created: 2026-02-06
epic: developer-experience
---

# Pass claude.flags to Interactive Sessions

## Specification

Interactive Claude sessions launched by `gromit refine` and `gromit plan` should respect the `claude.flags` configuration from `gromit.yaml`. Currently, these commands build their `exec.Command` calls with only `--append-system-prompt`, ignoring the flags array entirely. Non-interactive sessions (runner, decompose) already use `claude.flags` via the `claude.Client`.

After this change, running `gromit refine` or `gromit plan` with `claude.flags: ["--dangerously-skip-permissions"]` in config will launch Claude without permission prompts, matching the behavior of non-interactive sessions.

The flags are inserted into the command arguments before the existing `--append-system-prompt` and initial message arguments. If no config file exists or `claude.flags` is empty, behavior is unchanged.

## Acceptance Criteria

- `gromit refine` passes all `claude.flags` from config to the Claude CLI invocation
- `gromit plan` passes all `claude.flags` from config to the Claude CLI invocation
- When no config exists or `claude.flags` is empty, both commands still work as before

## Decisions

1. **Reuse existing `claude.flags` for all sessions** Rather than introducing a separate `claude.interactive_flags` config key, interactive sessions use the same `claude.flags` array as non-interactive sessions. This keeps config simple and ensures consistent behavior across all Claude invocations.

2. **No CLI flag override** This is purely config-driven. Users who want `--dangerously-skip-permissions` add it to `claude.flags` in `gromit.yaml` once and it applies everywhere.

## Research & Context

### Current State

- `cmd/gromit/refine.go:166` — builds `exec.Command("claude", "--append-system-prompt", systemPrompt, ...)` with no config flags
- `cmd/gromit/plan.go:182` — same pattern, no config flags
- `cmd/gromit/decompose.go:101` — uses `claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, ...)`, already works
- `internal/claude/claude.go` — `Client` struct accepts flags and appends them to every invocation
- `internal/config/config.go` — `ClaudeConfig.Flags []string` field, loaded from `gromit.yaml`
- `gromit.yaml:57` — already has `--dangerously-skip-permissions` in `claude.flags`

### Scope

Both `refine.go` and `plan.go` already call `loadConfig()` and have access to `cfg`. The change is adding `cfg.Claude.Flags...` to the `exec.Command` args, plus handling the `cfg == nil` case (which both files already check for).
