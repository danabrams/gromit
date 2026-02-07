---
created: 2026-02-07T00:00:00Z
decomposed: true
decomposed_at: "2026-02-07T04:50:46-05:00"
id: claude-skill
source_spec: claude-skill
---

# Claude Code Orchestrator Skill Implementation Plan

**Goal:** Add a `gromit install-skill` command that installs a `/gromit` Claude Code skill, a SessionStart hook script, and hook registration — enabling users to orchestrate the full Gromit pipeline from within Claude Code.

**Architecture:** A new Cobra command writes three artifacts: `.claude/skills/gromit.md` (orchestrator skill with embedded stage skills), `.gromit/hooks/pipeline-resume.sh` (SessionStart hook), and a hook entry in `.claude/settings.json`. A new `internal/pipeline` package provides pipeline status reading for the dashboard.

**Tech Stack:** Go, Cobra CLI, bash (hook script), Claude Code skills/hooks system

**Spec:** `.gromit/specs/claude-skill.md`

---

## Architecture

### Key Components

1. **`cmd/gromit/install_skill.go`** — New Cobra command `gromit install-skill`. Follows `init.go` pattern: creates directories, writes files idempotently, merges settings. Has `--force` flag.

2. **`skills/gromit-orchestrator/SKILL.md`** — The orchestrator skill template. Contains dashboard logic, stage dispatch instructions, state file writing instructions. Has placeholder markers where refine/plan/decompose skill content gets inlined at install time.

3. **`skills/gromit-orchestrator/pipeline-resume.sh`** — The SessionStart hook script. Reads `.gromit/pipeline-state.json`, outputs skill content + context to stdout, deletes state file. No-op when state file absent.

4. **`skills/embed.go`** — Extended with `OrchestratorSkill` and `PipelineResumeHook` embeds.

5. **`internal/pipeline/status.go`** — Reads pipeline state from backlog.jsonl, specs dir, plans dir frontmatter, and bd CLI. Returns structured `PipelineStatus` with counts and recommendation.

### Data Flow

- **Install**: `gromit install-skill` → reads embedded content → inlines refine/plan/decompose skills into orchestrator template → writes `.claude/skills/gromit.md`, `.gromit/hooks/pipeline-resume.sh` → merges hook into `.claude/settings.json`
- **Dashboard**: `/gromit` invoked → Claude reads backlog.jsonl, scans specs/, parses plan frontmatter, runs `bd ready --json` → displays pipeline status
- **Interactive dispatch**: Skill writes `.gromit/pipeline-state.json` → user `/clear` → hook reads state, outputs skill+context, deletes state → fresh session
- **Non-interactive dispatch**: Skill launches Task subagent with skill prompt → runs autonomously → summarized results
- **Simple dispatch**: Skill runs `gromit add/run/status/queue/board` via Bash

### Integration Points

- `skills/embed.go` gets two new embeds
- `.claude/settings.json` merged (not overwritten) to add SessionStart hook
- Existing `RefineSkill`, `PlanSkill`, `DecomposeSkill` strings inlined into orchestrator at install time
- Makefile gets `install-skill` target

## Test Strategy

### Unit Tests
- `internal/pipeline/status_test.go`: Pipeline status from fixture data, recommendation logic, edge cases (empty/missing)
- `cmd/gromit/install_skill_test.go`: `buildSkillContent()` includes all stage skills, `mergeHookSettings()` JSON merge correctness and idempotency

### CLI Contract Tests
- Update `cli_contract_test.go`: help golden file, flag contract (`--force`), exit code tests for `install-skill`

### Integration Tests
- `cmd/gromit/install_skill_test.go`: Run in temp dir, verify artifacts created; run twice for idempotency; `--force` overwrites; settings merge preserves existing content

### Manual Testing
- Install in real project, invoke `/gromit`, verify dashboard
- Test `/clear` + hook flow end-to-end
- Verify state file consumed after hook fires

## Implementation Tasks

### Task 1: Create pipeline status package

**Files:**
- Create: `internal/pipeline/status.go`
- Test: `internal/pipeline/status_test.go`

**What to Do:**
Create a new package that reads pipeline state from existing gromit data sources and returns a structured status. The `PipelineStatus` struct should contain: `UnrefinedCount int`, `UnrefinedIdeas []string` (idea texts), `UnplannedSpecs []string` (spec names), `UndecomposedPlans []string` (plan names), `ReadyBeadCount int`, `Recommendation string`. Implement `ReadStatus(gromitDir, specsDir, plansDir string) (*PipelineStatus, error)` that:
- Parses `backlog.jsonl` to count items where status != "refined"
- Scans specs dir for `.md` files without corresponding plan in plans dir
- Scans plans dir, parses frontmatter, finds plans where `decomposed` != true
- Runs `bd ready --json` to count ready beads (best-effort — returns 0 if bd unavailable)
- Generates a recommendation string based on priority: unrefined > unplanned > undecomposed > ready beads

**Acceptance Criteria:**
- `ReadStatus` returns correct counts from fixture backlog/specs/plans
- Recommendation prioritizes pipeline stages correctly
- Handles missing directories and empty files gracefully

**Dependencies:** None

**Notes:** Reuse `internal/backlog.NewFile` for backlog reading and `internal/frontmatter.ReadFile` for plan parsing. For bd, use `internal/bead.NewClient` but handle errors gracefully (bd may not be installed).

### Task 2: Create orchestrator skill template and hook script

**Files:**
- Create: `skills/gromit-orchestrator/SKILL.md`
- Create: `skills/gromit-orchestrator/pipeline-resume.sh`

**What to Do:**
Write the orchestrator skill markdown that will be installed as `.claude/skills/gromit.md`. The skill should contain:
1. Frontmatter with `name: gromit`, description, version
2. Dashboard instructions: read backlog.jsonl (parse JSONL, count by status), scan specs/ and plans/ dirs, run `bd ready --json`, display formatted status, show recommendation
3. Stage dispatch instructions for each mode:
   - **Interactive (refine, plan)**: Write pipeline-state.json with stage/inputs/created_at, tell user to `/clear`
   - **Non-interactive (decompose)**: Launch Task subagent with decompose skill content + plan content
   - **Simple (add, run, status, queue, board)**: Run `gromit <cmd>` via Bash
4. Placeholder markers `<!-- REFINE_SKILL_CONTENT -->`, `<!-- PLAN_SKILL_CONTENT -->`, `<!-- DECOMPOSE_SKILL_CONTENT -->` where stage skills get inlined

Write the `pipeline-resume.sh` bash script that:
1. Checks if `.gromit/pipeline-state.json` exists — exits 0 if not
2. Reads the JSON, extracts `stage` field
3. Based on stage, finds the corresponding skill section in `.claude/skills/gromit.md` (between delimiter comments) and outputs it plus the inputs context to stdout
4. Deletes `.gromit/pipeline-state.json`

**Acceptance Criteria:**
- SKILL.md contains complete orchestrator instructions with dashboard, all dispatch modes, and placeholder markers
- pipeline-resume.sh exits silently when no state file exists
- pipeline-resume.sh outputs skill content and deletes state file when state file exists

**Dependencies:** None

**Notes:** The hook script uses `jq` for JSON parsing (common on dev machines). Include a fallback to `python3 -c` if jq unavailable. Skill content delimiters: `<!-- BEGIN:REFINE -->` / `<!-- END:REFINE -->` etc.

### Task 3: Add embeds and build skill content function

**Files:**
- Modify: `skills/embed.go`
- Create: `cmd/gromit/install_skill.go` (partial — just the build functions)

**What to Do:**
Add two new embed directives to `skills/embed.go`:
```go
//go:embed gromit-orchestrator/SKILL.md
var OrchestratorSkill string

//go:embed gromit-orchestrator/pipeline-resume.sh
var PipelineResumeHook string
```

In `cmd/gromit/install_skill.go`, implement `buildSkillContent() string` that:
- Takes the `OrchestratorSkill` template
- Replaces `<!-- REFINE_SKILL_CONTENT -->` with the content of `skills.RefineSkill` wrapped in delimiter comments
- Same for plan and decompose
- Returns the fully assembled skill content

Also implement `mergeHookSettings(existingJSON []byte) ([]byte, error)` that:
- Parses existing `.claude/settings.json` (or starts with empty object if file doesn't exist)
- Adds the SessionStart hook entry without duplicating if already present
- Preserves all existing settings (permissions, other hooks, etc.)
- Returns formatted JSON

**Acceptance Criteria:**
- `buildSkillContent()` output contains refine, plan, and decompose skill content between delimiters
- `mergeHookSettings()` correctly adds hook to empty settings, existing settings, and is idempotent

**Dependencies:** Task 2 (skill template must exist for embed to compile)

### Task 4: Implement install-skill command

**Files:**
- Create: `cmd/gromit/install_skill.go` (complete the command)
- Test: `cmd/gromit/install_skill_test.go`

**What to Do:**
Implement the `gromit install-skill` Cobra command with `--force` flag. The `RunE` handler should:
1. Create `.gromit/hooks/` directory
2. Write `.gromit/hooks/pipeline-resume.sh` from embedded content, set executable (0755)
3. Build skill content via `buildSkillContent()`
4. Create `.claude/skills/` directory
5. Write `.claude/skills/gromit.md` with built content
6. Read existing `.claude/settings.json` (or empty), merge hook via `mergeHookSettings()`, write back
7. Print confirmation and usage instructions
8. Use `writeFileIfNotExists` pattern from init.go (skip if exists unless `--force`)

Write unit tests for `buildSkillContent()` and `mergeHookSettings()`. Write integration tests that run the full command in a temp directory and verify all artifacts.

**Acceptance Criteria:**
- Running `gromit install-skill` creates all three artifacts with correct content
- Running twice is idempotent (no duplicate hooks, files skipped without `--force`)
- `--force` overwrites existing files

**Dependencies:** Task 3 (embeds and build functions)

### Task 5: Update CLI contract tests and Makefile

**Files:**
- Modify: `cmd/gromit/cli_contract_test.go`
- Modify: `Makefile`
- Create: `cmd/gromit/testdata/golden/install-skill.help.txt`

**What to Do:**
Add `install-skill` to all three contract test tables:
1. Help text golden file test: add `{"install-skill", []string{"install-skill", "--help"}}`
2. Flag contract: add `{name: "install-skill", flags: map[string]string{"force": "bool"}}`
3. Exit code tests: add help exit 0 test

Generate the golden file by running with `-update` flag.

Add Makefile target:
```makefile
install-skill: build
	./gromit install-skill
```

Update `.PHONY` to include `install-skill`.

**Acceptance Criteria:**
- `go test ./cmd/gromit/ -run TestCLIContract` passes with install-skill entries
- `make install-skill` builds and runs the command

**Dependencies:** Task 4 (command must exist for golden file generation)

---

## Notes

- The orchestrator skill file will be ~800-1000 lines once all three stage skills are inlined. This is large but necessary — the hook script must be self-contained.
- The `pipeline-resume.sh` script depends on `jq` or `python3` for JSON parsing. The install command should print a note about this dependency.
- After updating gromit (e.g., new skill content), users need to re-run `gromit install-skill --force` to update the installed files. The install command should print the current gromit version in the skill file header for traceability.
- The pipeline-state.json format is intentionally simple and forward-compatible — new fields can be added without breaking the hook script.
