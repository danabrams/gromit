---
created: 2026-02-12T00:00:00Z
decomposed: true
decomposed_at: "2026-02-12T15:55:03-05:00"
id: codex-cli-spike-findings
source_spec: codex-cli-spike-findings
---

# Codex CLI Spike Findings Implementation Plan

**Goal:** Fix the CodexProvider and agent preset to use the correct Codex CLI invocation pattern discovered during the spike — stdin delivery via `codex exec -` instead of the nonexistent `--prompt` flag.

**Architecture:** Rewrite CodexProvider's command construction and prompt delivery to match the actual Codex CLI interface (`codex exec --full-auto --cd <dir> --skip-git-repo-check --color never --model <model> -` with prompt piped via stdin). Fix the interactive agent preset to use Stdin delivery. Update model defaults to `gpt-5.3-codex`.

**Tech Stack:** Go, Codex CLI (`codex exec`), existing Gromit provider/agent packages

**Spec:** `.gromit/specs/codex-cli-spike-findings.md`

---

## Architecture

**Overview:**
The spike revealed that the current CodexProvider uses `--prompt <file>` which doesn't exist in the Codex CLI. The correct invocation is `cat <file> | codex exec --full-auto ... -` with the prompt piped via stdin. This plan fixes the foundation so the streaming parity work (already decomposed in `codex-streaming-parity` plan) builds on a correct command structure.

**Key Components:**

1. **CodexProvider** (`internal/provider/codex.go`): Restructure command construction. Remove `promptDelivery`/`promptFlag` fields (unused, per bead `gromit-2622`). Rewrite `buildCommandArgs` to produce `exec <flags> --full-auto --skip-git-repo-check --color never --model <model> -`. Change `Run()`/`StreamRun()` to pipe prompt via `cmd.StdinPipe()` instead of temp file.

2. **Agent preset** (`internal/agent/resolve.go`): Change the codex preset from `PromptFileArg` + `--prompt` to `Stdin` delivery with `exec` as first extra arg and mandatory flags.

3. **Config/model defaults** (`gromit.yaml`, `internal/provider/provider.go`): Add `gpt-5.3-codex` tier mapping and update example config.

**Integration Points:**
- CodexProvider is selected by the Router for autonomous build/validation phases
- Agent preset is used for interactive workflows (explore, review, plan)
- The `codex-streaming-parity` plan (already decomposed) builds on this corrected foundation
- Bead `gromit-2622` (remove unused promptDelivery field) is superseded by Task 1

**Data Flow:**
```
Run(prompt, tier)
  → buildCommandArgs(model) → ["exec", ...flags, "--full-auto", "--skip-git-repo-check",
                                "--color", "never", "--model", model, "-"]
  → exec.CommandContext(binary, args...)
  → cmd.StdinPipe() ← write prompt content
  → cmd.Run()
  → parse exit code, capture stdout+stderr
  → Result
```

**Files to Modify:**
- `internal/provider/codex.go` — Rewrite command construction and prompt delivery
- `internal/provider/codex_test.go` — Update all mock binaries for new invocation
- `internal/agent/resolve.go` — Fix codex preset delivery mode
- `internal/agent/resolve_test.go` — Update codex preset tests
- `internal/provider/provider.go` — Add gpt-5.3-codex tier mapping
- `gromit.yaml` — Add commented Codex provider config example

**Tradeoffs:**
- **Hardcoded operational flags**: `--full-auto`, `--skip-git-repo-check`, `--color never` are hardcoded in `buildCommandArgs` rather than configurable. These are always required for automated operation (per spike findings) and making them configurable adds complexity with no benefit.
- **WorkDir via CWD inheritance**: Rather than adding a `--cd` flag to every invocation, rely on process CWD inheritance (which Codex supports). Users can add `--cd` via the `flags` config field if needed.

## Test Strategy

**Unit Tests:**
- Each arg in the invocation pattern verified individually via mock binary echoing `$@`
- Stdin delivery verified via mock binary reading and echoing stdin content
- Agent preset delivery mode checked via type assertion on `cliAgent` fields

**Integration Tests:**
- Full CodexProvider flow with realistic mock binary parsing all args and stdin

**Key Test Cases:**
- Provider: exec subcommand present, --full-auto present, --skip-git-repo-check present, --color never present, --model <model> present, `-` as final arg, prompt readable from stdin
- Agent: Stdin delivery mode, exec in extra args, mandatory flags present
- Backward compat: nil handler means no --json flag (existing test preserved for streaming parity)

**Mocking Strategy:**
- Mock bash scripts echoing args and reading stdin (existing pattern in codex_test.go)
- No real Codex CLI needed

---

## Implementation Tasks

### Task 1: Restructure CodexProvider for correct Codex CLI invocation

**Files:**
- Modify: `internal/provider/codex.go`
- Modify: `internal/provider/codex_test.go`

**What to Do:**

Remove `promptDelivery` and `promptFlag` fields from `CodexProvider` struct. Update `NewCodexProvider` to no longer accept these parameters. Delete the `createPromptFile` method.

Rewrite `buildCommandArgs(model string) []string` to produce:
```
exec <cp.flags...> --full-auto --skip-git-repo-check --color never --model <model> -
```
The `exec` subcommand comes first, then user-configurable flags (like `--cd <dir>`), then hardcoded operational flags, then `--model` with the resolved model, then `-` as the final positional arg signaling stdin input.

Rewrite `Run()` to pipe prompt via stdin:
- Create command with `exec.CommandContext`
- Get stdin pipe via `cmd.StdinPipe()`
- Start command with `cmd.Start()`
- Write prompt to stdin pipe, close pipe
- Wait for command to finish via `cmd.Wait()`
- Capture stdout and stderr via buffers set before Start

Rewrite `StreamRun()` with the same stdin approach. When `handler` is nil (no streaming), output goes to the provided writer + capture buffer as before. When `handler` is non-nil, the `--json` flag addition is deferred to the streaming parity plan — for now, behavior is unchanged (plain text capture).

Update all tests in `codex_test.go`:
- Mock binaries need to read from stdin (e.g., `cat` or `read`) instead of parsing `--prompt` flag
- Mock binaries should echo `$@` to verify correct arg structure
- Update `TestCodexProviderRunWritesPromptToTempFile` → rename to `TestCodexProviderRunPipesPromptViaStdin`
- Update `TestCodexProviderRunBuildsCommandWithModelFlag` to verify full arg structure
- Add `TestCodexProviderRunIncludesExecSubcommand`
- Add `TestCodexProviderRunIncludesMandatoryFlags` (--full-auto, --skip-git-repo-check, --color never)
- Add `TestCodexProviderRunIncludesStdinDash` (- as final arg)
- Update `NewCodexProvider` calls throughout tests to remove promptDelivery/promptFlag params

**Acceptance Criteria:**
- `buildCommandArgs` produces args starting with `exec` and ending with `-`, including `--full-auto`, `--skip-git-repo-check`, `--color`, `never`
- `Run()` and `StreamRun()` pipe prompt content to the command's stdin, verified by mock binary reading stdin
- `promptDelivery` and `promptFlag` fields removed from struct and constructor

**Dependencies:** None

**Notes:**
This supersedes bead `gromit-2622` (remove unused promptDelivery field) — we're removing the fields as part of fixing the invocation. The `ClaudeProvider` is unaffected since it wraps `claude.Client` which handles its own command construction. The streaming parity plan's Task 4 (add `--json` flag when handler is non-nil) builds on top of this corrected foundation.

---

### Task 2: Fix Codex agent preset for interactive workflows

**Files:**
- Modify: `internal/agent/resolve.go`
- Modify: `internal/agent/resolve_test.go`

**What to Do:**

Replace the shared `resolvePromptFileArgPreset` call for codex with a codex-specific preset function. The codex preset should create an agent with:
- `binary`: `"codex"`
- `promptDelivery`: `Stdin` (not `PromptFileArg`)
- `promptFlag`: `""` (not needed for stdin)
- `extraArgs`: `[]string{}` (no extra args after prompt)
- `flags`: `[]string{"exec", "--full-auto", "--skip-git-repo-check", "--color", "never"}` — the `exec` subcommand and mandatory flags go in the flags slice since they precede any prompt-related args

The agent's `Launch()` method already handles `Stdin` delivery by reading the prompt file and piping it to the command's stdin (see `agent.go:103-118`). The `-` positional arg for Codex is not needed in the agent system because the agent launches interactively — Codex reads from stdin directly when no positional prompt arg is given in `exec` mode.

Keep gemini unchanged on `resolvePromptFileArgPreset`.

Update `TestResolveCodexPreset` in `resolve_test.go` to verify:
- `promptDelivery` is `Stdin` (not `PromptFileArg`)
- `flags` includes `exec`, `--full-auto`, `--skip-git-repo-check`, `--color`, `never`
- `promptFlag` is empty

**Acceptance Criteria:**
- Codex agent preset uses `Stdin` delivery mode with `exec` and mandatory flags
- Gemini agent preset unchanged (still uses `PromptFileArg` with `--prompt`)
- Existing tests for claude and gemini presets pass without modification

**Dependencies:** None (parallel with Task 1)

**Notes:**
The agent system is separate from the provider system. Agents are for interactive workflows (review, explore, plan) where the CLI tool runs in the foreground. The provider is for autonomous workflows (build, validation) where output is captured programmatically. Both need the correct invocation pattern.

---

### Task 3: Update Codex model defaults and example config

**Files:**
- Modify: `internal/provider/provider.go`
- Modify: `gromit.yaml`

**What to Do:**

In `provider.go`, add `gpt-5.3-codex` to the `TierFromLegacyModel` mapping. Since this is the only model available with ChatGPT auth and serves all tiers, map it to `TierMedium` (sensible default — it's the general-purpose tier).

In `gromit.yaml`, add a commented-out providers section showing the recommended Codex configuration:
```yaml
# providers:
#   openai:
#     binary: "codex"
#     models:
#       high: "gpt-5.3-codex"
#       medium: "gpt-5.3-codex"
#       low: "gpt-5.3-codex"
```

Add this after the existing `routing` commented section (around line 30).

**Acceptance Criteria:**
- `TierFromLegacyModel("gpt-5.3-codex")` returns `TierMedium`
- `gromit.yaml` includes a commented example showing Codex provider configuration with `gpt-5.3-codex`
- Existing tier mappings for Claude and OpenAI models unchanged

**Dependencies:** None (parallel with Tasks 1-2)

**Notes:**
The spike found that only `gpt-5.3-codex` works with ChatGPT auth. Other models (`o3`, `gpt-4o`, `gpt-4o-mini`) require API key auth. The example config reflects this by using `gpt-5.3-codex` for all tiers. Users with API key auth can substitute other models.

---

## Notes

- **Parallel execution**: All three tasks are independent and can proceed in parallel.
- **Supersedes bead `gromit-2622`**: Task 1 removes the `promptDelivery` and `promptFlag` fields as part of fixing the command structure, making that bead redundant.
- **Foundation for streaming parity**: The `codex-streaming-parity` plan (already decomposed into beads) adds `--json` streaming, RunValidation, and shared helpers on top of this corrected invocation. Those beads should execute after this plan's tasks.
- **No authentication task**: The spike documents `codex login status` for auth checking, but this is user-facing setup, not something the provider needs to implement. If auth fails, the Codex CLI returns exit code 1 with an error message, which the provider already handles.
