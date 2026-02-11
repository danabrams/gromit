---
id: decompose-overlap-guard
source_spec: decompose-overlap-guard
created: 2026-02-10
decomposed: false
---

# Decompose Overlap Guard Implementation Plan

**Goal:** Prevent sibling bead overlap and test-only bead creation under ATDD during auto-decomposition.

**Architecture:** Add `ATDDActive` field to `DecomposeContext`, wire it through `DecomposeTask()`, and add anti-overlap + conditional ATDD guidance to `PROMPT_decompose.md`.

**Tech Stack:** Go, Go text/template

**Spec:** `.gromit/specs/decompose-overlap-guard.md`

---

## Architecture

**Overview:**
Three surgical changes: extend the decompose context struct with an ATDD flag, populate it in the runner, and add template guidance that uses it.

**Key Components:**
1. **`DecomposeContext`** (`internal/prompt/prompt.go`): Extended with `ATDDActive bool` field
2. **`PROMPT_decompose.md`** (`.gromit/templates/`): Anti-overlap guidance (unconditional) + ATDD test-only suppression (conditional on `{{.ATDDActive}}`)
3. **`DecomposeTask()`** (`internal/runner/runner.go`): Computes ATDD-active from bead labels + config, passes to context

**Integration Points:**
- `DecomposeContext` is consumed by `RenderDecompose()` which renders `PROMPT_decompose.md`
- `DecomposeTask()` builds the context and already has access to `r.cfg` (for global ATDD default) and the bead (for label overrides)
- `bead.IsMethodologyActive()` already exists and handles the label+config logic

**Data Flow:**
1. `attemptDecomposition()` calls `DecomposeTask(bead)`
2. `DecomposeTask()` checks `bead.IsMethodologyActive(b.Labels, "atdd", r.cfg.Methodology.ATDD)`
3. Sets `ATDDActive` on `DecomposeContext`
4. `RenderDecompose()` renders template; `{{if .ATDDActive}}` controls ATDD section visibility

**Files to Modify:**
- `internal/prompt/prompt.go` — Add `ATDDActive bool` to `DecomposeContext` struct
- `internal/runner/runner.go` — Set `ATDDActive` in `DecomposeTask()` context construction
- `.gromit/templates/PROMPT_decompose.md` — Add anti-overlap and conditional ATDD guidance

**Tradeoffs:**
- Template guidance over code enforcement: Overlap detection is a judgment call better handled by prompt instructions than algorithmic detection
- Spec scope only: CLI decompose skill (`SKILL.md`) not modified — can be addressed separately

## Test Strategy

**Test Levels:**
1. **Unit Tests**: Template rendering with ATDDActive=true and ATDDActive=false
2. **Regression Tests**: Existing `go test ./...` passes unchanged

**Key Test Cases:**
- RenderDecompose with ATDDActive=false → contains anti-overlap text, no ATDD section
- RenderDecompose with ATDDActive=true → contains both anti-overlap and ATDD text
- Default DecomposeContext (ATDDActive zero-value) → backward compatible, same as false

**Test Organization:**
- New test in `internal/prompt/prompt_test.go` using real template rendering
- Existing runner/decompose tests remain unchanged (mock interface signature unchanged)

## Implementation Tasks

### Task 1: Add ATDDActive to DecomposeContext and wire through DecomposeTask

**Files:**
- Modify: `internal/prompt/prompt.go`
- Modify: `internal/runner/runner.go`

**What to Do:**
Add `ATDDActive bool` field to the `DecomposeContext` struct in prompt.go. In runner.go's `DecomposeTask()`, compute ATDD-active status using `bead.IsMethodologyActive(b.Labels, "atdd", r.cfg.Methodology.ATDD)` and set it on the context before rendering. This is a two-line code change in each file.

**Acceptance Criteria:**
- `DecomposeContext` has an `ATDDActive bool` field
- `DecomposeTask()` sets `ATDDActive` based on bead labels and config methodology setting
- Existing decompose tests still pass (`go test ./internal/runner/...`)

**Dependencies:**
- None

**Notes:**
The `PromptRenderer` interface signature doesn't change — `RenderDecompose` still takes `*DecomposeContext`. Mocks default `ATDDActive` to false (zero value), so no mock updates needed.

### Task 2: Add anti-overlap and ATDD guidance to decompose template with tests

**Files:**
- Modify: `.gromit/templates/PROMPT_decompose.md`
- Create: `internal/prompt/decompose_test.go` (or add to existing prompt_test.go)

**What to Do:**
Add two blocks to PROMPT_decompose.md's Guidelines section: (1) Anti-overlap rules — unconditional, instruct the decomposer to verify each sub-task's acceptance criteria wouldn't be satisfied by completing a sibling task. (2) ATDD test-only bead suppression — conditional on `{{if .ATDDActive}}`, instruct not to create beads whose sole purpose is writing tests. Write tests that render the template with ATDDActive=true and ATDDActive=false and verify the expected text is present/absent.

**Acceptance Criteria:**
- PROMPT_decompose.md contains anti-overlap guidance visible in all decompose outputs
- PROMPT_decompose.md contains ATDD test-only suppression guidance visible only when ATDDActive=true
- Tests verify both conditions pass

**Dependencies:**
- Task 1 (ATDDActive field must exist on DecomposeContext)

---

## Notes

- The CLI `gromit decompose` command uses a separate path (`skills/gromit-decompose/SKILL.md`) that is NOT affected by these changes. Adding similar guidance to the CLI skill would be a separate spec/bead.
- The anti-overlap guidance uses the "mental cross-check" approach from the spec: instruct Claude to ask "If I completed task N, would task M's criteria still fail?" for each pair of sibling tasks.
- The ATDD conditional uses Go's `{{if .ATDDActive}}...{{end}}` template syntax, consistent with existing template patterns (e.g., `{{if .ParentBead}}` in the current decompose template).
