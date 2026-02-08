---
created: 2026-02-08T00:00:00Z
decomposed: true
decomposed_at: "2026-02-08T06:27:57-05:00"
id: debug-command
source_spec: debug-command
---

# Debug Command Implementation Plan

**Goal:** Add `gromit debug` command that launches an interactive Claude session for investigating bugs in the target codebase, with graduated outcomes (trivial fix, report + plan, or report + backlog item).

**Architecture:** Follows the same interactive session pattern as `refine.go`/`plan.go` — build system prompt with embedded skill and project context, write to temp file, launch Claude binary interactively, detect artifacts post-session and offer pipeline chaining.

**Tech Stack:** Go, Cobra CLI, go:embed for skill file

**Spec:** `.gromit/specs/debug-command.md`

---

## Architecture

**Overview:**
A new Cobra command in `cmd/gromit/debug.go` that assembles project context (CLAUDE.md, RULES.md, LEARNINGS.md, validation commands) into a system prompt with an embedded debug skill, launches an interactive Claude session, then detects what happened (direct fix, report created, backlog item added) and displays a summary with optional pipeline chaining.

**Key Components:**
1. **`cmd/gromit/debug.go`** — Cobra command with `--model` flag (default opus). Loads config, builds prompt via `prompt.Renderer`, launches Claude, handles post-session artifacts.
2. **`skills/gromit-debug/SKILL.md`** — Embedded skill instructing Claude on free-form investigation, graduated outcomes, report format, and validation requirements.
3. **`skills/embed.go`** (modify) — Add `DebugSkill` embed variable.

**Integration Points:**
- `loadConfig()`, `resolveGromitDir()` — existing config helpers
- `prompt.NewRenderer()` — loads CLAUDE.md, RULES.md, LEARNINGS.md
- `cfg.Validation.Commands` — validation commands for direct-fix outcome
- `backlog.NewFile()` — detect new backlog items post-session
- `confirmPrompt()`, `execGromit()` — post-session chaining helpers

**Data Flow:**
1. User runs `gromit debug` or `gromit debug "description"` with optional `--model`
2. Load config, create `prompt.Renderer`, extract project context
3. Build system prompt: bug description + project context + validation commands + reports dir + skill instructions
4. Write prompt to `.gromit/tmp/debug-prompt-*.md`, launch Claude interactively
5. After exit: scan `.gromit/reports/` for new files, check backlog for new items, check for new plans
6. Display summary, offer chaining (plan → decompose) if plan was created

**Files to Create:**
- `cmd/gromit/debug.go` — Command implementation
- `cmd/gromit/debug_test.go` — Unit tests for helper functions
- `skills/gromit-debug/SKILL.md` — Debug skill instructions

**Files to Modify:**
- `skills/embed.go` — Add `DebugSkill` embed

**Tradeoffs:**
- Reuse `prompt.Renderer` for context loading despite not needing templates/specs dirs — avoids duplicating file-loading logic
- `--model` flag for flexibility without config changes — opus default per spec
- Post-session artifact scanning matches refine.go pattern for reliable user feedback

## Test Strategy

**Unit Tests:**
- Report detection: finds new files in `.gromit/reports/`, ignores pre-existing, handles missing directory
- Backlog item detection: identifies items added during session
- Prompt assembly: all required context sections present

**Manual Testing:**
- `gromit debug` blank session
- `gromit debug "description"` pre-seeded session
- `--model sonnet` override
- End-to-end graduated outcomes

**Test Organization:**
- `cmd/gromit/debug_test.go` — helper function tests with temp directories

## Implementation Tasks

### Task 1: Create the debug skill file

**Files:**
- Create: `skills/gromit-debug/SKILL.md`

**What to Do:**
Write the debug skill markdown that instructs Claude on how to investigate bugs. The skill covers:
- Free-form investigation approach (read files, trace code, reproduce issues, check logs, run tests)
- Graduated outcomes with clear decision criteria:
  1. Trivial fix: fix directly + run validation commands
  2. Clear fix, no design decisions: write report to `.gromit/reports/debug-<timestamp>.md` + create plan + offer decompose
  3. Needs investigation/design: write report + add to backlog via `gromit add`
- Investigation report format (Symptom, Root Cause, Affected Code, Suggested Fix, Evidence)
- Validation command execution for direct fixes
- Report file naming convention: `debug-YYYYMMDD-HHMMSS.md`

The skill receives these context variables in the prompt (not Go template vars — just sections in the markdown):
- Bug description (or blank)
- CLAUDE.md content
- RULES.md content
- LEARNINGS.md content
- Validation commands list
- Reports directory path
- Working directory

**Acceptance Criteria:**
- Skill file exists at `skills/gromit-debug/SKILL.md`
- Skill covers all three graduated outcomes with clear decision criteria
- Report format matches spec exactly

**Dependencies:** None

### Task 2: Add skill embedding and create debug command

**Files:**
- Modify: `skills/embed.go`
- Create: `cmd/gromit/debug.go`

**What to Do:**
Add `DebugSkill` embed to `skills/embed.go`:
```go
//go:embed gromit-debug/SKILL.md
var DebugSkill string
```

Create `cmd/gromit/debug.go` following the `refine.go` pattern:

1. Define Cobra command:
   - `Use: "debug [bug description]"`
   - `Args: cobra.MaximumNArgs(1)`
   - `--model` string flag, default "opus"

2. `runDebug` function:
   - Load config via `loadConfig()` (tolerate missing config like refine does)
   - Resolve `gromitDir`
   - Create `prompt.Renderer` to load CLAUDE.md, RULES.md, LEARNINGS.md
   - Get validation commands from `cfg.Validation.Commands`
   - Determine reports dir: `filepath.Join(gromitDir, "reports")`
   - Ensure reports dir exists (`os.MkdirAll`)
   - Record existing reports (scan directory before session)
   - Record existing backlog items (load backlog before session)
   - Record existing plan files (scan plans dir before session)
   - Build system prompt with all context sections + embedded `skills.DebugSkill`
   - Determine `--model` flag value; if provided, add `--model <value>` to Claude args
   - Write prompt to temp file in `.gromit/tmp/debug-prompt-*.md`
   - Launch Claude binary interactively (same exec.Command pattern as refine.go)
   - Handle ExitError gracefully

3. Post-session detection:
   - Scan for new report files in `.gromit/reports/`
   - Scan for new plan files in plans dir
   - Scan for new backlog items
   - Display summary:
     - New reports: print paths
     - New plans: offer `gromit decompose` chaining
     - New backlog items: print IDs
     - If none of the above: check git diff for direct fix, show changed files summary

**Acceptance Criteria:**
- `gromit debug` launches interactive Claude session with CLAUDE.md, RULES.md, LEARNINGS.md context
- `gromit debug "description"` includes bug description in prompt
- `--model` flag overrides default opus model
- Post-session detects new reports, plans, and backlog items

**Dependencies:** Task 1

### Task 3: Add unit tests for debug command helpers

**Files:**
- Create: `cmd/gromit/debug_test.go`

**What to Do:**
Write unit tests for the helper functions in `debug.go`:

1. **Report detection tests** (`TestGetNewReports` or similar):
   - Setup: create temp dir, add some pre-existing `.md` files
   - Test: new files added after snapshot are detected
   - Test: pre-existing files are not included
   - Test: handles missing directory (returns empty, no error)
   - Test: handles empty directory

2. **Backlog diff tests** (`TestGetNewBacklogItems` or similar):
   - Setup: create temp gromit dir with backlog.jsonl containing existing items
   - Test: new items added to backlog after snapshot are detected
   - Test: no new items returns empty slice

3. **Model flag defaulting test**:
   - Test: default model is "opus"
   - Test: `--model sonnet` overrides to "sonnet"

**Acceptance Criteria:**
- Tests cover report detection with new/existing/missing files
- Tests cover backlog item diff detection
- All tests pass with `go test ./cmd/gromit/ -run TestDebug`

**Dependencies:** Task 2

---

## Notes

- The debug command does NOT use `--model` as a Claude CLI flag directly. Instead, it should be passed as part of the Claude args similar to how the retro command works (positional arg or flag depending on Claude CLI interface). Check how `refine.go` handles the claude binary — it uses `claudeFlags` from config plus the initial prompt as positional arg. The `--model` flag for debug should be added to the Claude CLI args.
- The `.gromit/reports/` directory is new — no existing code references it. The `init` command may need updating in a future bead to create this directory, but for now `debug.go` creates it with `os.MkdirAll`.
- The skill file should instruct Claude to use `gromit add` (not `bd create`) for backlog items, per CLAUDE.md conventions.
- Post-session git diff detection (for direct fix summary) is best-effort — if no reports/plans/backlog items were created, show `git diff --stat` output. Don't fail if git isn't available.
