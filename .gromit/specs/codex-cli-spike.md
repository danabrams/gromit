---
id: codex-cli-spike
source_ideas: []
created: 2026-02-11
---

# Spike: Codex CLI Invocation Modes

## Specification

A manual investigation of OpenAI's Codex CLI to verify how it behaves in non-interactive (automated) execution. The findings will directly inform the `CodexProvider` implementation required by the `multi-provider-routing` spec.

Gromit's automated build loop needs to invoke Codex as a subprocess, pass it a prompt, capture its output, detect usage-limit errors, and control its approval mode. None of these behaviors are documented well enough to build a provider without empirical testing.

### What to Test

The spike consists of a manual checklist covering seven areas. For each area, run the listed commands, record the actual output/behavior, and note any surprises or deviations from expectations.

#### 1. Authentication

Verify Codex CLI is authenticated and can make API calls.

```bash
# Check codex is on PATH and version
codex --version

# Authenticate (if needed — check codex docs for auth flow)
codex auth login    # or whatever the auth command is

# Verify auth works with a trivial prompt
codex "Say hello"
```

**Record:** Auth flow steps, any environment variables needed (OPENAI_API_KEY?), whether auth persists across sessions.

#### 2. Prompt File Delivery

Verify that `--prompt <file>` works for passing a prompt from a file, which is how Gromit's `PromptFileArg` delivery method works.

```bash
# Create a test prompt file
cat > /tmp/codex-spike-prompt.md << 'EOF'
Create a file called /tmp/codex-spike-output.txt containing the text "spike test passed".
EOF

# Test prompt_file_arg delivery
codex --prompt /tmp/codex-spike-prompt.md

# Also test: does codex accept prompt on stdin?
cat /tmp/codex-spike-prompt.md | codex

# Also test: does codex accept prompt as positional arg?
codex "Create a file called /tmp/codex-spike-output2.txt containing 'hello'"
```

**Record:** Which delivery methods work, exact flag name (`--prompt` vs `--prompt-file` vs something else), whether file path is resolved relative to CWD or absolute.

#### 3. Model Flag Format

Verify the `--model` flag format and available model names.

```bash
# List available models (if such a command exists)
codex models list    # or codex --help to find the right subcommand

# Test explicit model selection
codex --model o3 "What model are you?"
codex --model gpt-4o "What model are you?"
codex --model gpt-4o-mini "What model are you?"

# Test with invalid model name to see error format
codex --model nonexistent-model "hello"
```

**Record:** Exact flag syntax, available model names, default model when `--model` is omitted, error format for invalid models.

#### 4. Exit Codes

Verify exit code behavior for success, failure, and usage limits.

```bash
# Successful execution — expect exit code 0
codex "echo hello" ; echo "Exit code: $?"

# Invalid invocation — check error exit code
codex --invalid-flag ; echo "Exit code: $?"

# Usage limit hit — may need to trigger this intentionally or wait for natural occurrence
# Record the exit code when it happens naturally
```

**Record:** Exit code for success (expect 0), exit code for invalid usage, exit code for usage/rate limits (critical for `IsUsageLimitError()`), any stderr messages that accompany limit errors.

#### 5. Output Format

Verify what stdout/stderr look like in non-interactive execution.

```bash
# Run a simple task and capture all output
codex "List the files in the current directory" 2>/tmp/codex-stderr.txt | tee /tmp/codex-stdout.txt

# Check if there's a structured output mode (JSON, events, etc.)
codex --help | grep -i "output\|format\|json"

# If a JSON/structured mode exists, test it
# codex --output-format json "List files"   # hypothetical
```

**Record:** Is stdout plain text or structured? Does stderr contain progress/status info? Is there an equivalent to Claude's `--output-format stream-json`? What does the final output look like (just the result, or includes metadata)?

#### 6. Approval Mode Flags

Verify how to run Codex in fully automated mode (no human approval needed for file writes, command execution, etc.). This is critical for the automated build loop.

```bash
# Check help for approval/auto-approve flags
codex --help | grep -i "approve\|auto\|suggest\|full"

# Test fully automated mode (adjust flag name based on help output)
codex --approval-mode full-auto "Create a file /tmp/codex-auto-test.txt with 'auto test'"
# or maybe:
codex --full-auto "Create a file /tmp/codex-auto-test.txt with 'auto test'"

# Test suggest mode (if it exists) — does it block waiting for input?
codex --approval-mode suggest "Create a file /tmp/codex-suggest-test.txt with 'suggest test'"
```

**Record:** Exact flag for full-auto mode, whether suggest mode blocks on stdin, default approval mode, any safety restrictions in full-auto mode (e.g., can't run arbitrary commands).

#### 7. Working Directory and File Access

Verify how Codex handles working directory and file system access.

```bash
# Test from a specific directory
cd /tmp && codex "What is your current working directory? Write it to /tmp/codex-cwd.txt"

# Test file read access
codex "Read the file /tmp/codex-spike-prompt.md and summarize its contents"

# Test if codex respects any sandbox or directory restrictions
codex "List files in /etc"
```

**Record:** Does Codex respect CWD of the parent process? Any sandbox restrictions? Can it read/write files outside CWD? Any directory flag equivalent to Claude's `--project-dir`?

### Success Criteria for the Spike

The spike is complete when all seven sections have been tested and the findings document answers these questions:

1. **Prompt delivery:** What is the exact flag/method for passing a prompt file?
2. **Model selection:** What is the exact `--model` flag syntax and which models are available?
3. **Exit codes:** What exit code indicates a usage limit, and what does the error message look like?
4. **Output capture:** How should Gromit capture and parse Codex's output?
5. **Automation flags:** What flags enable fully autonomous operation?
6. **Working directory:** Does Codex inherit CWD, and are there file access restrictions?
7. **Blockers:** Are there any showstoppers that would prevent building a CodexProvider?

## Acceptance Criteria

- All seven test sections have been executed and findings recorded
- A findings document exists (can be inline in a bead close message or a separate file) answering the seven questions above
- Any blockers or unexpected behaviors are flagged with workaround proposals
- The findings are sufficient to write the `CodexProvider` implementation without further Codex CLI experimentation

## Decisions

1. **Manual checklist, not automated tests.** This is exploratory work — the goal is to observe and document behavior, not build test infrastructure. A manual approach lets the tester adapt in real-time when commands don't work as expected.

2. **Spike scope is observation only.** The spike does not implement any Go code. It produces findings that inform the `CodexProvider` implementation, which is a separate bead.

3. **Seven test areas.** The original four areas from the backlog idea (prompt delivery, model flag, exit codes, output format) were expanded to include authentication, approval mode flags, and working directory behavior — all necessary for a correct `CodexProvider`.

## Research & Context

### Current State

- `internal/agent/agent.go` — Already implements `PromptFileArg` delivery for interactive phases. Uses `--prompt` as the default flag for Codex. This assumption needs verification.
- `internal/agent/resolve.go:173-175` — The Codex preset: `New(name, name, nil, PromptFileArg, defaultPromptFlag, nil)`. No static flags, binary is "codex", prompt flag is "--prompt".
- `internal/claude/claude.go` — The Claude client that a CodexProvider would mirror. Key methods: `Run()` (non-streaming), `StreamRun()` (streaming with event parsing), `RunValidation()` (structured validation prompt).
- `multi-provider-routing` spec — Defines the `Provider` interface and `CodexProvider` that this spike informs. The spec assumes `codex --prompt <file>` and `--model <name>` work, but this hasn't been verified.

### Related Specs

- `multi-provider-routing` — The parent feature that needs the CodexProvider. This spike de-risks the implementation.
- `multi-agent-phases` — Already implemented agent selection for interactive phases. The Codex interactive integration works; this spike is about the automated (non-interactive) path.
