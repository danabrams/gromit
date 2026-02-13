---
created: 2026-02-13T00:00:00Z
decomposed: true
decomposed_at: "2026-02-13T15:23:25Z"
id: reduce-iteration-cost
source_spec: reduce-iteration-cost
---

# Reduce Iteration Cost Implementation Plan

**Goal:** Reduce gromit iteration time and token spend via three independent levers: reducing validation retries, trimming static context, and surfacing learnings selectively.

**Architecture:** Most work is already complete. Levers A and C are fully implemented. Lever B (trim static context) is ~70% done — RULES.md phase filtering and review wiring are complete. The remaining work is removing dead CLAUDE.md injection from non-build phases.

**Tech Stack:** Go, text/template

**Spec:** `.gromit/specs/reduce-iteration-cost.md`

---

## Architecture

### Already Completed (no further work needed)

**Lever A — Reduce Validation Retry Rate (COMPLETE):**
- `RecentValidationFailures []string` on `prompt.Context` and `validationFailures []string` on `Runner`
- `extractValidationSummary()` in `validation_summary.go` extracts FAIL lines from go test/vet, caps at 500 chars
- Accumulation in `runValidation()` (`process.go:1155-1157`), injection of last 3 in `buildPromptForBead()` (`process.go:126-132`), reset per `Run()` call (`runner.go:337`)
- Self-check guidance in all 3 build templates (standard, ATDD, TDD)

**Lever B — RULES.md Phase Filtering (COMPLETE):**
- Phase annotations in `.gromit/RULES.md` (`<!-- phases: build, review -->` on Code Style/Safety/Test Quality; `<!-- phases: build -->` on Process)
- `LoadRulesForPhase()` and `filterRulesByPhase()` in `prompt.go:445-525`
- `runLightReview()` and `runThoroughReview()` call `LoadRulesForPhase("review")`

**Lever C — Learnings Filtering (COMPLETE):**
- `FilterOptions{MaxChars, Keywords}` and `GetConfirmedFiltered()` in `learnings.go`
- `LearningsConfig{MaxLearningChars}` with default 8000 in `config.go`
- `SetMaxLearningChars()` on `Renderer`, wired in `NewRunner()`, used in `BuildContext()`

### Remaining Work

**Lever B — Trim CLAUDE.md for non-build phases:**

Every Claude CLI invocation (`claude -p`) automatically loads CLAUDE.md from the project root. Gromit prompts ALSO include CLAUDE.md content explicitly via templates. For non-build phases, this explicit inclusion is unnecessary:

1. **Review templates** (`PROMPT_review.md`, `PROMPT_thorough_review.md`): Don't reference `{{.ClaudeMD}}` at all. The `LoadClaudeMD()` calls in `runLightReview()` (line 1868) and `runThoroughReview()` (line 2126) are dead code — they load CLAUDE.md into context but the templates never render it. Remove these calls.

2. **Refactor template** (`PROMPT_refactor.md`): Has `{{if .ClaudeMD}}{{.ClaudeMD}}{{end}}` block. Since Claude CLI loads CLAUDE.md automatically, this explicit inclusion is redundant. Remove the block from the template.

3. **Acceptance test template** (`PROMPT_acceptance_tests.md`): Same pattern as refactor. Remove the `{{.ClaudeMD}}` block.

4. **Build templates**: KEEP `{{.ClaudeMD}}` — build is the highest-value phase where inline positioning of project context improves Claude's awareness of conventions.

**Context savings:** ~5.3KB removed per non-build invocation (review, refactor, acceptance tests). With 2.8 invocations per bead and ~1.8 non-build invocations, this saves ~9.5KB per bead.

---

## Test Strategy

**Unit Tests:**
- Verify review context construction no longer calls `LoadClaudeMD()`
- Verify build context still includes CLAUDE.md (no regression)

**Integration Tests:**
- Render refactor and acceptance test templates, confirm no CLAUDE.md section in output
- Render build templates, confirm CLAUDE.md section is still present

**Mocking:** Use existing `mockPromptRenderer` with `LoadClaudeMDFn`

**Coverage Goals:**
- Critical: build prompts unchanged, non-build prompts trimmed
- Edge: empty ClaudeMD field handled by template conditionals (already works via `{{if .ClaudeMD}}`)

---

## Implementation Tasks

### Task 1: Remove dead ClaudeMD loading from review code paths

**Files:**
- Modify: `internal/runner/runner.go`

**What to Do:**
Remove the two dead `LoadClaudeMD()` calls:
- Line 1868 in `runLightReview()`: `reviewCtx.ClaudeMD, _ = r.renderer.LoadClaudeMD()`
- Line 2126 in `runThoroughReview()`: `reviewCtx.ClaudeMD, _ = r.renderer.LoadClaudeMD()`

The `ClaudeMD` fields on `ReviewContext` and `ThoroughReviewContext` structs stay (zero-value empty strings are harmless and avoid cascading struct changes). The review templates don't reference `{{.ClaudeMD}}` so this is pure dead code removal.

**Acceptance Criteria:**
- `runLightReview()` does not call `LoadClaudeMD()`
- `runThoroughReview()` does not call `LoadClaudeMD()`
- Existing review tests pass without modification

**Dependencies:** None

**Notes:** This is dead code removal — the review templates (`PROMPT_review.md`, `PROMPT_thorough_review.md`) have never referenced `{{.ClaudeMD}}`. No behavioral change.

### Task 2: Remove explicit ClaudeMD from refactor and acceptance test templates

**Files:**
- Modify: `.gromit/templates/PROMPT_refactor.md`
- Modify: `.gromit/templates/PROMPT_acceptance_tests.md`
- Test: `internal/prompt/prompt_test.go` (if existing tests check for ClaudeMD in these templates)

**What to Do:**
Remove the `{{if .ClaudeMD}}...{{end}}` blocks from both templates. Claude CLI's automatic CLAUDE.md loading provides the same project context. Keep the `ClaudeMD` field in the `Context` struct (still needed by build templates).

In `PROMPT_refactor.md`, remove lines 29-31:
```
{{if .ClaudeMD}}
{{.ClaudeMD}}
{{end}}
```

In `PROMPT_acceptance_tests.md`, remove lines 29-31:
```
{{if .ClaudeMD}}
{{.ClaudeMD}}
{{end}}
```

**Acceptance Criteria:**
- Refactor template renders without CLAUDE.md section
- Acceptance test template renders without CLAUDE.md section
- Build templates still include CLAUDE.md (no regression)

**Dependencies:** None (independent of Task 1)

### Task 3: Add test for review phase ClaudeMD omission

**Files:**
- Modify: `internal/runner/review_phase_rules_test.go` (or new test in runner package)

**What to Do:**
Add a test that verifies `runLightReview()` does not call `LoadClaudeMD()`. Use the existing `mockPromptRenderer` pattern — configure `LoadClaudeMDFn` to set a flag if called, then run a light review and assert the flag was not set. This prevents regression (someone re-adding the call).

**Acceptance Criteria:**
- Test verifies `LoadClaudeMD()` is not called during review
- Test passes with current implementation

**Dependencies:** Task 1

---

## Notes

- The `ClaudeMD` field remains on `ReviewContext`, `ThoroughReviewContext`, and `Context` structs. Removing struct fields would cascade into mock updates and test changes with no runtime benefit. Empty strings are harmless.
- Claude CLI's automatic CLAUDE.md loading means non-build phases still have project context — they just don't have it duplicated inline in the prompt.
- The `LoadClaudeMD()` method stays on the `PromptRenderer` interface since `BuildContext()` still uses it for build prompts.
- Existing bead `gromit-4dvf` ("Trim prompt boilerplate for non-build phases") covers this remaining work.
