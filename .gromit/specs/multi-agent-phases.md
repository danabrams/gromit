---
id: multi-agent-phases
source_ideas: []
created: 2026-02-08
epic: provider-ecosystem
---

# Multi-Agent Interactive Phases

## Specification

Gromit's interactive phases (refine, plan, review, and future explore) currently hardcode Claude Code as the only agent. This feature makes interactive phases agent-agnostic, allowing users to choose the best AI agent for each task — e.g., Codex for deep code reviews, Gemini for research and exploration, Claude for specification writing.

### Agent Definitions

Agents are defined in `gromit.yaml` under a new `agents` section. Gromit ships built-in presets for known CLI tools (claude, codex, gemini) and supports custom agent definitions.

```yaml
agents:
  definitions:
    claude: {}     # Built-in preset, uses cfg.Claude.Binary and cfg.Claude.Flags
    codex:         # Built-in preset
      binary: "codex"
    gemini:        # Built-in preset
      binary: "gemini"
    my-tool:       # Custom agent
      binary: "my-agent-cli"
      flags: ["--some-flag"]
      prompt_delivery: "prompt_file_arg"  # How to pass the prompt
      prompt_flag: "--instructions"       # Flag name if prompt_delivery is prompt_file_arg
```

Each agent preset encapsulates:
- **binary**: The CLI executable name/path
- **flags**: Default flags to pass
- **prompt_delivery**: How the agent receives its prompt. Options:
  - `file_ref` (default for claude): Pass a short message telling the agent to read a temp file ("Read and follow instructions in /path/to/file")
  - `prompt_file_arg`: Pass the temp file path as a flag argument (e.g., `--instructions /path/to/file`)
  - `stdin`: Pipe the prompt to stdin
- **prompt_flag**: The flag name used when `prompt_delivery` is `prompt_file_arg`

Built-in presets define practical defaults for each known agent so users can start with `codex: {}` / `gemini: {}` and override when their installed CLI differs.

Gemini-specific note from spike findings (`.gromit/plans/gemini-cli-spike-findings.md`, 2026-02-23): use `stdin` as the preferred prompt delivery default, with prompt-flag invocation retained as a fallback option. Live non-interactive Gemini behavior is now observed for `json`, `stream-json`, and model-valid/invalid paths; keep config overrides for environment-specific differences and still-unverified auth/rate-limit/transport signatures.

### Per-Phase Agent Assignment

Each interactive phase has a default agent, configurable in `gromit.yaml`:

```yaml
agents:
  phases:
    refine: claude
    plan: claude
    review: claude
    explore: claude
```

When no `agents.phases` config exists, all phases default to `claude` (backward-compatible).

### Agent Selection at Runtime

Three ways to control which agent runs:

1. **Config default**: The `agents.phases.<phase>` setting in `gromit.yaml`
2. **Flag override**: `gromit refine --agent codex` skips the picker and uses codex directly
3. **Interactive picker**: `gromit refine --choose-agent` shows a picker listing all defined agents
4. **Always-prompt config**: `agents.prompt: true` in `gromit.yaml` makes the picker appear every time (equivalent to always passing `--choose-agent`)

Priority: `--agent <name>` > `--choose-agent` / `agents.prompt` > `agents.phases.<phase>` default.

The picker looks like:

```
Select agent for refine:

  1. claude (default)
  2. codex
  3. gemini

Choice [1-3]:
```

### Affected Phases

Only interactive (TTY) phases are in scope:

| Phase | Command | Current behavior |
|-------|---------|-----------------|
| Refine | `gromit refine` | Launches claude with refine skill prompt |
| Plan | `gromit plan` | Launches claude with plan skill prompt |
| Review (interactive) | `gromit review` | Launches claude with review prompt |
| Explore | `gromit explore` | Does not exist yet (future) |

Automated phases (build, validation, scope check, precheck, analysis, learning, decompose, light review) remain Claude-only. They depend on Claude-specific output formats (stream-json, VALIDATION_PASSED markers, structured JSON) and are not candidates for this feature.

### Launch Behavior

When an interactive phase launches an agent:

1. Gromit builds the prompt (unchanged from current behavior)
2. Gromit writes the prompt to a temp file (unchanged)
3. Gromit resolves the agent definition (built-in preset or custom)
4. Gromit constructs the command using the agent's prompt_delivery method
5. Gromit launches the agent with stdin/stdout/stderr connected to the terminal
6. After exit, Gromit checks for output artifacts (new spec files, plan files, etc.) as usual

The post-exit artifact detection is agent-agnostic — it checks for file creation, not agent-specific output.

## Acceptance Criteria

- Users can define custom agents in `gromit.yaml` under `agents.definitions` with binary, flags, and prompt_delivery settings
- Built-in presets for `claude` exist and work without any explicit definition (backward-compatible with existing configs that have no `agents` section)
- `agents.phases` config allows setting a default agent per interactive phase (refine, plan, review)
- `--agent <name>` flag on refine, plan, and review commands overrides the configured default
- `--choose-agent` flag on refine, plan, and review shows an interactive picker of all defined agents
- `agents.prompt: true` config option causes the picker to appear on every interactive phase launch
- When a non-claude agent is selected, Gromit launches it using the configured prompt_delivery method with the prompt temp file

## Decisions

1. **Interactive phases only** The automated phases (build, validation, decompose, etc.) are deeply coupled to Claude's output format (stream-json, exit codes, VALIDATION_PASSED markers). Making those agent-agnostic would require an adapter layer for each agent's output format, which is significantly more complex and not needed for the initial use case. The user's primary interest is in research, review, and specification phases — all interactive.

2. **CLI tools, not APIs** Agents are invoked as CLI processes, not via HTTP APIs. This avoids Gromit needing to manage API keys, billing, auth tokens, and HTTP clients for each provider. CLI tools handle their own authentication. If a user wants to use an API-based agent, they can wrap it in a CLI script and define it as a custom agent.

3. **Named presets + custom definitions** Rather than requiring users to know the exact invocation syntax for every agent, Gromit ships presets for common agents (claude, codex, gemini). The preset encapsulates binary path, default flags, and prompt delivery method. Custom agents are supported via explicit definition for tools Gromit doesn't know about.

4. **Prompt delivery abstraction** Different CLI agents accept prompts differently — Claude reads files from a "read this file" instruction, others may want a `--prompt-file` flag, others may read stdin. The `prompt_delivery` field in agent definitions abstracts this, so each phase doesn't need agent-specific code.

5. **Backward compatibility** Existing configs with no `agents` section continue to work identically — all phases use the existing `claude.binary` and `claude.flags` settings. The `agents` section is purely additive.

## Research & Context

### Current State

The interactive phases share a common pattern in `cmd/gromit/`:

- **refine.go** (lines 189-235): Builds prompt, writes to temp file, launches `exec.Command(claudeBinary, flags..., "Read and follow ... in <file>")` with TTY passthrough
- **plan.go** (lines 188-234): Identical pattern — prompt to temp file, launch claude with TTY
- **review.go** (lines 297-373): Same pattern for interactive review mode

All three use `cfg.Claude.Binary` and `cfg.Claude.Flags` from the `ClaudeConfig` struct in `internal/config/config.go`.

The `ClaudeConfig` struct (config.go:78-86) holds: binary, timeout, stall_timeout, stall_timeout_active, bead_timeout, analysis_timeout, flags. The timeout/stall fields are only used by automated phases, not interactive ones. Interactive phases only use `binary` and `flags`.

### Agent CLI Landscape

- **Claude Code** (`claude`): Accepts initial prompt as positional arg, reads files when instructed
- **OpenAI Codex** (`codex`): CLI tool for code generation and review
- **Google Gemini CLI** (`gemini`): Google's CLI agent. Prompt delivery should default to `stdin` per spike findings; keep config overrides available for local CLI differences.
- **Aider** (`aider`): Open-source AI pair programming tool
- **Cursor CLI**: Editor-based AI agent

Each has different flag conventions and prompt delivery mechanisms, reinforcing the need for agent-specific adapters rather than a one-size-fits-all approach.
