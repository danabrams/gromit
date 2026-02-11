---
created: 2026-02-11T00:00:00Z
decomposed: true
decomposed_at: "2026-02-11T09:04:17-05:00"
id: codex-cli-spike
source_spec: codex-cli-spike
---

# Codex CLI Spike Implementation Plan

**Goal:** Manually investigate OpenAI's Codex CLI to verify how it behaves in automated execution, producing findings that directly inform the `CodexProvider` implementation.

**Architecture:** Manual exploration spike — no Go code written. Execute seven test sections against the real Codex CLI, record observations, and compile a findings document mapping observations to `CodexProvider` implementation decisions.

**Tech Stack:** Codex CLI (OpenAI), bash

**Spec:** `.gromit/specs/codex-cli-spike.md`

---

## Architecture

This is an observation-only spike. The deliverable is a structured findings document that answers seven questions the `CodexProvider` needs answered before implementation can begin.

**Investigation Flow:**
1. Authentication → Prompt Delivery → Model Flags → Exit Codes → Output Format → Approval Mode → Working Directory
2. Authentication must be verified first; remaining sections are largely independent
3. All findings compiled into a single document with implementation recommendations

**Output Artifact:** Findings recorded inline in bead close messages or as a standalone document, answering all seven questions from the spec plus a blockers section.

**Integration Points:**
- Findings feed into `CodexProvider` implementation (part of `multi-provider-routing` spec)
- Validates or corrects the Codex agent preset assumptions in `internal/agent/resolve.go:173-175`
- Informs whether `StreamRun()`, `RunValidation()`, and `IsUsageLimitError()` are feasible for Codex

**Key codebase context for the investigator:**
- `internal/agent/resolve.go:172-175` — Current Codex preset assumes binary `codex`, prompt flag `--prompt`, no static flags
- `internal/claude/claude.go` — Claude client with `Run()`, `StreamRun()`, `RunValidation()` that CodexProvider must mirror
- `internal/runner/interfaces.go:39-44` — `ClaudeClient` interface the runner depends on (3 methods)
- `.gromit/specs/multi-provider-routing.md` — Provider interface with `IsUsageLimitError()` method

## Test Strategy

No automated tests — this is pure manual observation.

**Observation Protocol:** For each test area, record exact commands, stdout/stderr output, exit codes, and any deviations from assumptions.

**Sufficiency Test:** Findings are sufficient when someone could implement all four `Provider` interface methods (`Run`, `StreamRun`, `RunValidation`, `IsUsageLimitError`) without additional Codex CLI experimentation.

**Blocker Criteria:** Flag as blocker if Codex lacks full-auto mode, has no parseable output format, doesn't support `--model`, or any behavior that makes `CodexProvider` impossible.

**What could go wrong:**
- Codex CLI not installed → document installation steps
- Auth requires interactive OAuth → document whether API key env var works
- Usage limits not triggerable on demand → document from help/docs, flag for production observation
- No structured output mode → document that StreamRun needs plain-text passthrough

## Implementation Tasks

### Task 1: Investigate Codex CLI core invocation behavior

**What to Do:**
Verify authentication, prompt delivery methods, model flag syntax, and exit code behavior. These are the foundational CLI mechanics that determine how `CodexProvider.Run()` and `CodexProvider.RunValidation()` will invoke Codex.

**Sections covered:** 1 (Authentication), 2 (Prompt File Delivery), 3 (Model Flag Format), 4 (Exit Codes)

**Commands to run:**

Section 1 — Authentication:
```bash
codex --version
codex --help    # discover auth flow
# Try a trivial prompt to verify API access
codex "Say hello"
```
Record: auth flow steps, env vars needed, whether auth persists.

Section 2 — Prompt File Delivery:
```bash
# Create test prompt
cat > /tmp/codex-spike-prompt.md << 'EOF'
Create a file called /tmp/codex-spike-output.txt containing the text "spike test passed".
EOF

# Test --prompt flag (assumed by agent preset)
codex --prompt /tmp/codex-spike-prompt.md

# Test stdin delivery
cat /tmp/codex-spike-prompt.md | codex

# Test positional arg
codex "Create a file called /tmp/codex-spike-output2.txt containing 'hello'"
```
Record: which delivery methods work, exact flag name, path resolution behavior.

Section 3 — Model Flag:
```bash
codex --help | grep -i model
codex --model o3 "What model are you?"
codex --model gpt-4o "What model are you?"
codex --model gpt-4o-mini "What model are you?"
codex --model nonexistent-model "hello"   # error format
```
Record: flag syntax, available models, default model, error format.

Section 4 — Exit Codes:
```bash
codex "echo hello" ; echo "Exit code: $?"
codex --invalid-flag ; echo "Exit code: $?"
```
Record: exit codes for success, invalid usage, and (if observed) usage limits.

**Acceptance Criteria:**
- Authentication verified and env var requirements documented
- At least one prompt delivery method confirmed working with exact flag name recorded
- Model flag syntax and at least two available model names documented
- Exit codes for success and failure recorded

**Dependencies:** None (first task)

**Notes:**
- Check `codex --help` liberally to discover exact flag names — don't assume the spec's guesses are correct
- If Codex CLI is not installed, first finding is the installation procedure
- Adapt commands in real-time when they don't work as expected — that's the point of a spike

### Task 2: Investigate Codex CLI automation suitability

**What to Do:**
Verify output format, approval mode flags, and working directory behavior. These determine how `CodexProvider.StreamRun()` will capture output, whether Codex can run without human interaction, and how it handles the filesystem.

**Sections covered:** 5 (Output Format), 6 (Approval Mode Flags), 7 (Working Directory)

**Commands to run:**

Section 5 — Output Format:
```bash
# Capture stdout/stderr separately
codex "List the files in the current directory" 2>/tmp/codex-stderr.txt | tee /tmp/codex-stdout.txt

# Check for structured output modes
codex --help | grep -i "output\|format\|json"

# If JSON/structured mode exists, test it
```
Record: stdout structure (plain text vs structured), stderr content, any streaming/JSON mode.

Section 6 — Approval Mode:
```bash
# Find approval-related flags
codex --help | grep -i "approve\|auto\|suggest\|full"

# Test full-auto mode (adjust flag based on help output)
codex --approval-mode full-auto "Create a file /tmp/codex-auto-test.txt with 'auto test'"

# Test suggest mode — does it block?
codex --approval-mode suggest "Create a file /tmp/codex-suggest-test.txt with 'suggest test'"
```
Record: exact full-auto flag, whether suggest mode blocks, default approval mode, safety restrictions.

Section 7 — Working Directory:
```bash
cd /tmp && codex "What is your current working directory? Write it to /tmp/codex-cwd.txt"
codex "Read the file /tmp/codex-spike-prompt.md and summarize its contents"
codex "List files in /etc"
```
Record: CWD inheritance, sandbox restrictions, file access outside CWD, any `--project-dir` equivalent.

**Acceptance Criteria:**
- Output format documented with stdout/stderr samples
- Full-auto approval flag identified and verified working (or flagged as blocker)
- Working directory behavior confirmed — CWD inheritance and file access scope documented

**Dependencies:** Task 1 (authentication must be working)

**Notes:**
- The approval mode section is the most critical for automated build loop feasibility — if there's no full-auto mode, that's a potential showstopper
- If no structured output mode exists, note that `StreamRun()` will need to pass through plain text (simpler than Claude's stream-json parsing)

### Task 3: Compile findings and write implementation recommendations

**What to Do:**
Compile all observations from Tasks 1 and 2 into a structured findings document. Map each finding to a concrete `CodexProvider` implementation decision. Flag any blockers with proposed workarounds. The document should be sufficient for someone to implement `CodexProvider` without running any additional Codex experiments.

**Findings document structure:**

1. **Summary** — One-paragraph overview of spike results
2. **Per-section findings** — For each of the seven sections:
   - What was tested (exact commands)
   - What was observed (output, exit codes, behavior)
   - Implementation implication (how this maps to `CodexProvider`)
3. **CodexProvider Implementation Notes:**
   - `Run()`: How to invoke, what flags, how to capture output
   - `StreamRun()`: Whether streaming is possible, fallback if not
   - `RunValidation()`: How to structure validation prompts
   - `IsUsageLimitError()`: What exit code/error pattern indicates usage limits
4. **Agent Preset Corrections** — Whether `internal/agent/resolve.go:173-175` assumptions are correct
5. **Blockers** — Any showstoppers with workaround proposals
6. **Open Questions** — Anything that couldn't be determined from the spike

**Acceptance Criteria:**
- Findings document answers all seven questions from the spec's "Success Criteria" section
- Each finding maps to a concrete `CodexProvider` implementation decision
- Blockers (if any) include workaround proposals

**Dependencies:** Task 1, Task 2

---

## Notes

- This spike produces **findings only** — no Go code is written
- Findings feed directly into the `multi-provider-routing` plan for `CodexProvider` implementation
- If showstoppers are found, they should be escalated before proceeding with `multi-provider-routing`
- The spec's current Codex preset (`internal/agent/resolve.go:172-175`) may need updating based on findings
- The Codex CLI may require `OPENAI_API_KEY` environment variable — check during authentication section
- If Codex CLI is not installed, first finding is the installation procedure
- Usage limit errors may not be triggerable on demand — document whatever error patterns are observable and note if the limit error format remains unknown
