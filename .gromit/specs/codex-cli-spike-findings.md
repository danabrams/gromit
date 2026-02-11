# Codex CLI Spike Findings

Date: 2026-02-11
Codex Version: 0.98.0

## Executive Summary

OpenAI's Codex CLI provides a robust non-interactive execution mode via `codex exec` that is suitable for automated build loops. The CLI uses different command structure than Claude (no `--prompt` flag), has JSONL streaming output, and requires different model and approval handling.

---

## 1. Authentication

**Status:** ✅ Working

**Method:**
- Use `codex login` for interactive authentication
- Use `codex login --with-api-key` to read API key from stdin
- Check status with `codex login status`

**Findings:**
```bash
$ codex login status
Logged in using ChatGPT
```

**Environment Variables:**
- No explicit OPENAI_API_KEY environment variable required when using `codex login`
- Auth persists across sessions in `~/.codex/` config directory

**Implications for CodexProvider:**
- Authentication check should call `codex login status` and parse output
- No need to manage API keys in gromit config if user has already done `codex login`

---

## 2. Prompt File Delivery

**Status:** ⚠️ Different from Claude

**Methods Available:**

### Method 1: Stdin (Recommended)
```bash
cat prompt.md | codex exec --full-auto --cd /path --skip-git-repo-check -
```
- Use `-` as the positional PROMPT argument to read from stdin
- **This is the recommended method for large prompts** (avoids ARG_MAX limits)
- Works reliably, tested successfully

### Method 2: Positional Argument
```bash
codex exec --full-auto --cd /path --skip-git-repo-check "Prompt text here"
```
- Pass prompt as a positional string argument
- Subject to OS ARG_MAX limits
- Works for short prompts

### Method 3: File Reference (NOT AVAILABLE)
```bash
codex exec --prompt /path/to/prompt.md   # ❌ DOES NOT EXIST
```
- **There is no `--prompt` or `--prompt-file` flag in Codex CLI**
- The `agent.PromptFileArg` delivery method assumption in `agent.go` is incorrect for Codex

**Implications for CodexProvider:**
- Must use stdin delivery method (cat file | codex exec -)
- Cannot use `--prompt <file>` flag as currently defined in `agent.go`
- The `PromptFileArg` delivery mode needs to be reinterpreted as "stdin from file" for Codex
- May need a new delivery mode `StdinFromFile` or update `PromptFileArg` behavior based on provider

---

## 3. Model Flag Format

**Status:** ⚠️ Limited Model Availability

**Flag Syntax:** `--model <model-name>` or `-m <model-name>`

**Default Model:**
```
model: gpt-5.3-codex
```

**Available Models (with ChatGPT auth):**
- `gpt-5.3-codex` (default) - ✅ Works
- `o3` - ❌ Not supported with ChatGPT account
- `gpt-4o-mini` - ❌ Not supported with ChatGPT account
- `gpt-4o` - ❌ Not supported with ChatGPT account

**Error Format (invalid/unavailable model):**
```
2026-02-11T20:56:38.213654Z ERROR codex_api::endpoint::responses: error=http 400 Bad Request: Some("{\"detail\":\"The 'gpt-4o-mini' model is not supported when using Codex with a ChatGPT account.\"}")
ERROR: {"detail":"The 'gpt-4o-mini' model is not supported when using Codex with a ChatGPT account."}
Exit code: 1
```

**Implications for CodexProvider:**
- Default to `gpt-5.3-codex` if no model specified
- Model selection in gromit config may need to map to Codex-specific names
- ChatGPT auth only supports the default Codex model currently
- May need API key auth (not ChatGPT) to access other models
- Invalid model returns exit code 1 with JSON error on stderr

---

## 4. Exit Codes

**Status:** ✅ Documented

| Scenario | Exit Code | Example |
|----------|-----------|---------|
| Success | 0 | Normal completion |
| API Error (invalid model, rate limit) | 1 | Model not supported, usage limit |
| Invalid Usage (bad flags) | 2 | Unknown flag |

**Error Detection:**
- API errors (including usage limits) produce exit code 1
- Stderr contains both structured log line and JSON error:
  ```
  2026-02-11T20:56:38.213654Z ERROR codex_api::endpoint::responses: error=http 400 Bad Request: Some("{\"detail\":\"...\"}")
  ERROR: {"detail":"..."}
  ```
- Parse the `ERROR: {json}` line to get structured error details

**Rate Limit Error (Not Observed):**
- Could not trigger during testing
- Expect similar format: `ERROR: {"detail":"Rate limit exceeded"}` with exit code 1
- `IsUsageLimitError()` should check stderr for "rate limit" or "usage limit" keywords

**Implications for CodexProvider:**
- Check exit code: 0=success, 1=API error (check stderr for details), 2=invalid usage
- Parse stderr for `ERROR: {json}` lines when exit code is 1
- Usage limit detection needs stderr inspection, not just exit code

---

## 5. Output Format

**Status:** ✅ JSONL Streaming Available

**Plain Text Mode (default):**
- Stdout: Final result only (clean, human-readable)
- Stderr: All progress, commands, reasoning, metadata

Example stderr:
```
OpenAI Codex v0.98.0 (research preview)
--------
workdir: /tmp
model: gpt-5.3-codex
provider: openai
approval: never
sandbox: workspace-write [workdir, /tmp, $TMPDIR]
reasoning effort: none
reasoning summaries: auto
session id: 019c4e7d-7b03-7f51-9acd-84a3bb7608e1
--------
user
Create a file called /tmp/codex-spike-output.txt containing the text "spike test passed".

mcp startup: no servers

thinking
**Planning frequent commentary updates**
codex
Creating `/tmp/codex-spike-output.txt` now with the exact text you specified, then I'll verify its contents.
exec
/bin/zsh -lc "printf 'spike test passed' > /tmp/codex-spike-output.txt && cat /tmp/codex-spike-output.txt" in /tmp succeeded in 59ms:
spike test passed
codex
Created `/tmp/codex-spike-output.txt` with:

`spike test passed`
tokens used
1,966
```

Example stdout:
```
Created `/tmp/codex-spike-output.txt` with:

`spike test passed`
```

**JSON Streaming Mode (`--json`):**
- Stdout: JSONL event stream (similar to Claude's `--output-format stream-json`)
- Stderr: Still contains the human-readable progress

Example JSONL events:
```json
{"type":"thread.started","thread_id":"019c4e7e-7990-7e91-a7d3-f4d16c268ae7"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_0","type":"reasoning","text":"**Planning direct command execution**"}}
{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"Creating `/tmp/json-test.txt` now..."}}
{"type":"item.started","item":{"id":"item_2","type":"command_execution","command":"...","status":"in_progress"}}
{"type":"item.completed","item":{"id":"item_2","type":"command_execution","aggregated_output":"json test","exit_code":0,"status":"completed"}}
{"type":"turn.completed","usage":{"input_tokens":15887,"cached_input_tokens":14592,"output_tokens":127}}
```

**Event Types:**
- `thread.started` - Session begins
- `turn.started` - Turn begins
- `item.completed` - Reasoning, agent message, or command execution
- `turn.completed` - Includes token usage

**Implications for CodexProvider:**
- For streaming: Use `--json` flag and parse JSONL from stdout
- For non-streaming: Use default mode, capture stdout for result
- Token usage available in `turn.completed` event's `usage` field
- Similar structure to Claude's stream events, should be parseable with similar code

---

## 6. Approval Mode Flags

**Status:** ✅ Multiple Modes Available

**Flags:**

### `--full-auto` (Recommended for Build Loop)
```bash
codex exec --full-auto ...
```
- Equivalent to: `-a on-request --sandbox workspace-write`
- No interactive prompts
- Can write files in workspace and temp directories
- Cannot execute dangerous operations outside workspace
- **This is the recommended flag for automated builds**

### `--dangerously-bypass-approvals-and-sandbox`
```bash
codex exec --dangerously-bypass-approvals-and-sandbox ...
```
- Equivalent to: `-a never --sandbox danger-full-access`
- No sandboxing, no approvals
- Can execute any command, write anywhere
- **Use only in containerized/sandboxed environments**

### Manual Approval Configuration
```bash
codex exec -a <policy> --sandbox <mode> ...
```

Approval policies (`-a`):
- `untrusted` - Only run trusted commands without approval
- `on-failure` - Run all, ask approval only if command fails
- `on-request` - Model decides when to ask (default with --full-auto)
- `never` - Never ask for approval

Sandbox modes (`--sandbox`):
- `read-only` - No writes (default without --full-auto)
- `workspace-write` - Can write in workspace + temp (default with --full-auto)
- `danger-full-access` - No restrictions

**Default (without flags):**
```
approval: never
sandbox: read-only
```

**Implications for CodexProvider:**
- Use `--full-auto` for normal build/validation phases
- Use `--dangerously-bypass-approvals-and-sandbox` only if user explicitly configures unsafe mode
- No need to handle interactive approval in automated mode with --full-auto

---

## 7. Working Directory and File Access

**Status:** ✅ Well-Behaved

**Working Directory:**
- Codex exec inherits parent process CWD by default
- Use `--cd <dir>` flag to override: `codex exec --cd /path/to/project ...`
- The CWD is reflected in the "workdir" line in stderr output
- Equivalent to Claude's `--project-dir` flag

**File System Access:**
- With `--full-auto` (sandbox=workspace-write):
  - ✅ Can read files anywhere on filesystem
  - ✅ Can write files in workspace (workdir)
  - ✅ Can write files in /tmp and $TMPDIR
  - ❌ Cannot write files outside workspace/temp (sandboxed)

- With `--dangerously-bypass-approvals-and-sandbox`:
  - ✅ Can read/write anywhere (no restrictions)

**Required Flag: `--skip-git-repo-check`**
- Codex exec requires running inside a git repository by default
- Use `--skip-git-repo-check` to bypass this requirement
- **Must be included in all CodexProvider invocations** unless working in a git repo

**Implications for CodexProvider:**
- Always pass `--cd <projectDir>` to set working directory
- Always pass `--skip-git-repo-check` to avoid git repo requirement
- File access restrictions match expectations for a sandboxed build environment
- Read access is unrestricted (good for reading specs, learnings, etc.)

---

## 8. Other Important Flags

### `--color <mode>`
- Options: `always`, `never`, `auto`
- Controls ANSI color codes in output
- Set to `never` for clean parsing in automated scripts

### `--output-last-message <file>` (partial in help)
- Appears to write final message to a file
- Not fully documented in help output
- May be useful for capturing final result

### `-c, --config <key=value>`
- Override config values from `~/.codex/config.toml`
- Example: `-c model="o3"`
- Repeatable for multiple overrides

### `--add-dir <dir>`
- Add additional writable directories beyond workspace
- Useful if build outputs go to specific paths

---

## Blockers and Risks

### ❌ Blocker 1: No `--prompt <file>` Flag
**Impact:** High
**Issue:** The `agent.PromptFileArg` delivery mode assumes a `--prompt <file>` flag exists, but Codex CLI doesn't have this. Only stdin (`-`) or positional args work.

**Workaround:**
- Use stdin delivery: `cat prompt.md | codex exec -`
- Reinterpret `PromptFileArg` as "stdin from file" for Codex provider
- Add new delivery mode if needed: `StdinFromFile`

### ⚠️ Risk 1: Limited Model Selection with ChatGPT Auth
**Impact:** Medium
**Issue:** Only `gpt-5.3-codex` model is available when authenticated via ChatGPT. Cannot test o3 or other models.

**Workaround:**
- Default to `gpt-5.3-codex` for Codex provider
- Document that model selection requires API key auth (not ChatGPT)
- Consider making model selection optional for Codex in config

### ⚠️ Risk 2: Git Repo Check Requirement
**Impact:** Low
**Issue:** Must pass `--skip-git-repo-check` unless working in a git repo.

**Workaround:**
- Always include `--skip-git-repo-check` flag in CodexProvider
- Document in config that Codex has this requirement

### ⚠️ Risk 3: Cannot Empirically Test Rate Limiting
**Impact:** Low
**Issue:** Could not trigger rate limit during spike to observe exact error format.

**Workaround:**
- Assume similar error format to invalid model: `ERROR: {"detail":"rate limit exceeded"}`
- Implement `IsUsageLimitError()` with multiple keyword checks: "rate limit", "usage limit", "quota"
- Test with real rate limit when it occurs naturally

---

## Recommended CodexProvider Implementation

### Command Structure
```bash
cat <prompt-file> | codex exec \
  --cd <project-dir> \
  --skip-git-repo-check \
  --full-auto \
  --json \
  --color never \
  -
```

### Key Differences from Claude
1. **Prompt delivery:** Use stdin (`-`), not `--prompt <file>`
2. **No project-dir flag:** Use `--cd <dir>` instead
3. **Git check:** Must include `--skip-git-repo-check`
4. **Approval:** Use `--full-auto` instead of approval modes
5. **Streaming flag:** `--json` instead of `--output-format stream-json`

### Provider Interface Mapping
- `Run()` → `codex exec --full-auto --cd <dir> --skip-git-repo-check <prompt>`
- `StreamRun()` → `codex exec --full-auto --cd <dir> --skip-git-repo-check --json -`
- `RunValidation()` → Same as Run(), parse stdout for pass/fail
- `IsUsageLimitError()` → Check stderr for "rate limit" / "usage limit" keywords

### Config Additions Needed
```yaml
models:
  codex:
    validation: "gpt-5.3-codex"
    build_haiku: "gpt-5.3-codex"
    build_sonnet: "gpt-5.3-codex"
    build_opus: "gpt-5.3-codex"
```

---

## Success Criteria Completion

✅ **1. Prompt delivery:** Use stdin with `-` positional arg: `cat file | codex exec -`
✅ **2. Model selection:** `--model <name>` or `-m <name>`, default is `gpt-5.3-codex`
✅ **3. Exit codes:** 0=success, 1=API error, 2=invalid usage. Parse stderr for error details.
✅ **4. Output capture:** Use `--json` for JSONL events, or default for plain text on stdout
✅ **5. Automation flags:** Use `--full-auto` for sandboxed auto-execution
✅ **6. Working directory:** Use `--cd <dir>`, always include `--skip-git-repo-check`
✅ **7. Blockers:** One design mismatch (no --prompt flag), workable with stdin delivery

---

## Next Steps

1. Update `internal/agent/agent.go` to support stdin delivery for Codex
2. Implement `internal/codex/codex.go` (mirror of `internal/claude/claude.go`)
3. Add Codex provider to `internal/agent/provider.go`
4. Update config schema to include Codex model names
5. Add tests for Codex provider (may need mocks since ChatGPT auth limits testing)
6. Document Codex setup in README (requires `codex login` first)
