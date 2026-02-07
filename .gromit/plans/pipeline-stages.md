---
decomposed: true
decomposed_at: "2026-02-07T10:00:00-05:00"
---

# Pipeline Stages Implementation Plan

## Context

Gromit currently has `refine` and `plan` commands that both jump straight from conversation to creating beads. This loses the brainstorming/research artifacts and provides no quality gates between stages. We're building a four-stage pipeline (Capture → Refine → Plan → Decompose) where each stage produces a durable artifact and transitions are explicit command invocations.

Full spec: `.gromit/specs/pipeline-stages.md`

---

## Task 1: Add Status/SpecName fields to backlog + Update method

**Files:**
- Modify: `internal/backlog/backlog.go`
- Modify: `internal/backlog/backlog_test.go`

**Changes:**
- Add `Status string` and `SpecName string` to `Idea` struct with `json:",omitempty"` tags
- Add `Update(id string, fn func(*Idea))` method — loads all ideas, applies fn to matching idea, rewrites file (same pattern as `Delete`)

**Tests:**
- Roundtrip: add idea with status, read back, verify fields
- Update: add idea, update status, verify persisted
- Backwards compat: existing ideas without status/spec_name load fine (omitempty handles this)

---

## Task 2: Add Plans path to config + create in init

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `cmd/gromit/init.go`

**Changes in config.go:**
- Add `Plans string \`yaml:"plans"\`` to `PathsConfig` struct (after Specs field, line ~78)
- Add default in `setDefaults()`: `if c.Paths.Plans == "" { c.Paths.Plans = ".gromit/plans" }`

**Changes in init.go:**
- Add `".gromit/plans"` to `dirs` slice (line ~50)
- Update `Long` description to mention `plans/` directory
- Update "Next steps" output to mention the pipeline commands

**Tests:**
- Verify Plans default is set when loading config with no plans path
- Verify custom plans path from YAML is respected

---

## Task 3: Create frontmatter utility package

**Files:**
- Create: `internal/frontmatter/frontmatter.go`
- Create: `internal/frontmatter/frontmatter_test.go`

**Package provides:**
```go
// Parse splits markdown content into YAML frontmatter and body.
// Returns nil map and full content as body if no frontmatter present.
func Parse(content string) (map[string]interface{}, string, error)

// Serialize combines frontmatter map and body into markdown with YAML frontmatter.
func Serialize(fm map[string]interface{}, body string) (string, error)

// ReadFile reads a markdown file and returns parsed frontmatter + body.
func ReadFile(path string) (map[string]interface{}, string, error)

// UpdateFile reads a file, applies updates to frontmatter, writes back.
func UpdateFile(path string, updates map[string]interface{}) error
```

**Implementation:**
- Split on `---\n` delimiter (standard YAML frontmatter format)
- Use `gopkg.in/yaml.v3` for YAML marshal/unmarshal (already a dependency)
- `UpdateFile` reads existing content, merges updates into frontmatter map, writes back with body preserved

**Tests:**
- Parse with valid frontmatter
- Parse with no frontmatter (returns empty map, full body)
- Parse with empty frontmatter (`---\n---\n`)
- Roundtrip: Parse → Serialize → Parse yields same result
- ReadFile from disk
- UpdateFile merges new keys, preserves existing keys and body

---

## Task 4: Rewrite gromit-refine skill

**Files:**
- Modify: `skills/gromit-refine/SKILL.md`

**Key changes from current skill:**
- Output target: spec file in `.gromit/specs/<name>.md` instead of beads
- Keep: one-question-at-a-time approach, clarifying questions, approach exploration
- Add: codebase exploration phase (read relevant files before designing)
- Add: collaborative spec naming (propose slug, human approves)
- Add: spec format with frontmatter (`id`, `source_ideas`, `created`) and four sections (Specification, Acceptance Criteria, Decisions, Research & Context)
- Remove: all bead creation guidance, `bd create` examples, complexity-to-priority mapping

**Skill instructs Claude to:**
1. Read the codebase to understand existing patterns
2. Ask clarifying questions about the idea
3. Explore 2-3 approaches with tradeoffs
4. Collaboratively choose a spec name
5. Write the spec file using the Write tool

---

## Task 5: Rewrite gromit-plan skill

**Files:**
- Modify: `skills/gromit-plan/SKILL.md`

**Key changes from current skill:**
- Input: spec file content instead of feature description string
- Output: plan file in `.gromit/plans/<name>.md` instead of beads
- Keep: approach exploration, complexity assessment
- Add: architecture checkpoint (present, wait for human approval)
- Add: test strategy checkpoint (present, wait for human approval)
- Add: plan format with frontmatter (`id`, `source_spec`, `created`, `decomposed: false`) and flexible task structure
- Remove: all bead creation guidance, `bd create` examples

**Skill instructs Claude to:**
1. Read the spec thoroughly
2. Explore the codebase for relevant patterns
3. Propose architecture — pause for human review
4. Propose test strategy — pause for human review
5. Break work into tasks with files, acceptance criteria, dependencies
6. Write the plan file using the Write tool

---

## Task 6: Create gromit-decompose skill

**Files:**
- Create: `skills/gromit-decompose/SKILL.md`

**This skill guides non-interactive decomposition.** Claude reads it as context for the non-interactive `-p` invocation.

**Skill instructs Claude to:**
1. Read the plan content provided in the prompt
2. Extract tasks from the plan
3. Map each task to a bead following bead sizing rules (1-2 files, 1-3 acceptance criteria, self-contained)
4. Output a JSON array of bead definitions:
```json
[
  {
    "title": "...",
    "description": "...",
    "priority": 1,
    "acceptance_criteria": ["...", "..."],
    "depends_on_index": null
  }
]
```
5. `depends_on_index` is the array index of a prerequisite bead (null if none)
6. Output ONLY the JSON array, no markdown wrapper

---

## Task 7: Rewrite refine command

**Files:**
- Modify: `cmd/gromit/refine.go`

**Changes:**
- Args: `cobra.MaximumNArgs(1)` instead of `cobra.NoArgs`
- Three input modes:
  - No args: load backlog, filter `Status != "refined"`, display numbered list, prompt for selection with `bufio.Reader` (same pattern as `triage.go`)
  - Arg matches `idea-*` pattern: treat as backlog ID, load that specific idea
  - Other arg: treat as ad-hoc idea text
- System prompt includes:
  - The idea text (and context if from backlog)
  - Specs directory path (so Claude knows where to write)
  - Reference to gromit-refine skill
- After Claude exits: scan `.gromit/specs/` for files modified after session start time
  - If new spec found and input was a backlog item: call `backlog.File.Update()` to set `Status: "refined"` and `SpecName` to the spec's id
- Launch pattern: same `exec.Command("claude", "--append-system-prompt", ...)` pattern

---

## Task 8: Rewrite plan command

**Files:**
- Modify: `cmd/gromit/plan.go`

**Changes:**
- Args: `cobra.MaximumNArgs(1)` instead of `cobra.ExactArgs(1)`
- Flags: add `--force` bool flag
- Two input modes:
  - With arg: treat as spec name, look up `.gromit/specs/<name>.md`
  - No arg: glob `.gromit/specs/*.md`, display numbered list, prompt for selection
- Check if `.gromit/plans/<name>.md` exists:
  - If exists and no `--force`: print error message "Plan already exists for '<name>'. Use --force to regenerate." and return error
  - If exists and `--force`: continue
- Load spec content using `frontmatter.ReadFile()`
- System prompt includes:
  - Full spec content
  - Plans directory path (so Claude knows where to write)
  - Spec name (so plan frontmatter links back)
  - Open beads as context (keep existing pattern from current plan.go)
  - Reference to gromit-plan skill
- Launch pattern: same interactive `exec.Command` pattern

---

## Task 9: Create decompose command

**Files:**
- Create: `cmd/gromit/decompose.go`
- Modify: `cmd/gromit/main.go` (add `rootCmd.AddCommand(decomposeCmd)` in init)

**Command:** `gromit decompose <plan-name> [--review] [--force]`

**Implementation:**
1. Load config to get paths and Claude settings
2. Read plan file from `.gromit/plans/<name>.md` using `frontmatter.ReadFile()`
3. Check frontmatter `decomposed` field — if true and no `--force`, refuse
4. Load decompose skill content from `skills/gromit-decompose/SKILL.md`
5. Build prompt: plan content + skill instructions + spec name for labels
6. Call `claude.Client.Run(ctx, prompt, "sonnet")` — non-interactive
7. Parse JSON array from result output (trim markdown fences if present)
8. If `--review`: print each proposed bead (title, priority, AC, dependencies), prompt y/n
9. Create beads sequentially via `bead.Client.CreateWithParentAndDescription()`:
   - Track created bead IDs in a slice
   - Resolve `depends_on_index` to parent ID from the slice
   - Add `spec:<name>` label to each bead's labels
10. Update plan frontmatter via `frontmatter.UpdateFile()`: set `decomposed: true`, `decomposed_at: <now>`
11. Print summary: "Created N beads from plan '<name>'"

**Error handling:**
- JSON parse failure: print raw output and error, suggest `--review` next time
- Bead creation failure: print which beads were created before failure, don't mark as decomposed

**Bead definition struct (local to decompose.go):**
```go
type beadDef struct {
    Title              string   `json:"title"`
    Description        string   `json:"description"`
    Priority           int      `json:"priority"`
    AcceptanceCriteria []string `json:"acceptance_criteria"`
    DependsOnIndex     *int     `json:"depends_on_index"`
}
```

---

## Dependency Graph

```
Task 1 (backlog fields) ──────────────────┐
Task 2 (config + init) ───────────────────┤
Task 3 (frontmatter util) ────────────────┤
                                          │
Task 4 (refine skill)  ───── depends on nothing, but should be done before Task 7
Task 5 (plan skill)    ───── depends on nothing, but should be done before Task 8
Task 6 (decompose skill) ── depends on nothing, but should be done before Task 9
                                          │
Task 7 (refine cmd)    ───── depends on 1, 4
Task 8 (plan cmd)      ───── depends on 2, 3, 5
Task 9 (decompose cmd) ───── depends on 2, 3, 6
```

Tasks 1-6 can all be done in parallel. Tasks 7, 8, 9 can be done in parallel once their dependencies are met.

---

## Verification

**Build:**
```bash
go fmt ./... && go build ./cmd/gromit
```

**Unit tests:**
```bash
go test ./internal/backlog/... ./internal/config/... ./internal/frontmatter/...
```

**Integration test (manual):**
1. `gromit init --force` — verify `.gromit/plans/` created
2. `gromit add "Add OAuth support"` — capture idea
3. `gromit refine` — pick idea, brainstorm, verify spec written to `.gromit/specs/`
4. `gromit refine` — verify the refined idea no longer shows in picker
5. `gromit plan <spec-name>` — verify plan written to `.gromit/plans/`
6. `gromit plan <spec-name>` — verify refusal (plan exists)
7. `gromit plan <spec-name> --force` — verify overwrite works
8. `gromit decompose <plan-name>` — verify beads created with correct labels/dependencies
9. `gromit decompose <plan-name>` — verify refusal (already decomposed)
10. `gromit decompose <plan-name> --force` — verify re-decompose works

**Lint:**
```bash
golangci-lint run ./...
```
