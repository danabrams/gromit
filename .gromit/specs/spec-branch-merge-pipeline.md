---
id: spec-branch-merge-pipeline
source_ideas: []
created: 2026-02-21
epic: developer-experience
---

# Spec Branch Merge Pipeline

## Specification

When `gromit run` processes beads that belong to a spec, all work for that spec happens on an isolated feature branch named `gromit/spec-<spec-name>`. Beads without a spec label continue to execute directly on the current branch and merge to main immediately.

### Branch Lifecycle

When the run loop encounters the first bead of a spec, it creates (or checks out) the feature branch `gromit/spec-<spec-name>` from the current HEAD of main. All subsequent beads for that spec execute on this branch. Beads without a spec continue on main.

When the last bead of a spec closes, the merge pipeline triggers — replacing the current spec gate as the completion mechanism for spec work.

### Merge Pipeline Stages

The merge pipeline runs sequentially with hard gates. If any stage fails, later stages do not run.

**Stage 1: Full Validation**

Run the complete test suite with all build labels (no `//go:build` tag filtering), so acceptance tests and integration tests are included. Run all configured `full_commands` (tests, vet, build) plus linting. If ATDD is enabled, acceptance tests run as part of this stage.

**Stage 2: Spec Conformance Review**

LLM-driven review that verifies the implementation satisfies the spec's acceptance criteria. The full diff of the spec branch against main is provided along with the spec document. The review produces a pass/fail result with detailed findings.

**Stage 3: Code Quality Review**

LLM-driven review covering style, error handling, test quality, nil checks, and other code-level concerns. Operates on the same full diff. Produces a pass/fail result with findings and fix suggestions.

**Stage 4: Architectural Review**

LLM-driven review evaluating whether the implementation fits codebase patterns, uses appropriately sized modules, has clean boundary seams at the right places, and uses suitable abstractions. Produces a pass/fail result with findings and fix suggestions.

### Failure Handling

When any review phase fails, it creates fix beads labeled with the same spec. These beads enter the normal run loop. When the last fix bead closes, the merge pipeline triggers again from Stage 1 (full validation), regardless of which stage originally failed.

This cycle repeats up to a configurable cap (default: 3 attempts). If the cap is reached, the pipeline stops and alerts the user that the spec could not pass the merge gate.

### Merge to Main

After all four stages pass:

1. Rebase the spec branch onto the current HEAD of main.
2. If the rebase is clean, fast-forward merge to main.
3. If there are conflicts during rebase, invoke an LLM agent to resolve them. If the agent cannot resolve the conflicts, block and alert the user.
4. After successful merge, delete the spec branch.

### Bead Routing

The run loop must route beads to the correct branch:
- Beads with a `spec:<name>` label execute on `gromit/spec-<name>`.
- Beads without a spec label execute on main.
- When switching between specs or between spec/non-spec beads, the runner checks out the appropriate branch before executing.

## Acceptance Criteria

- Beads belonging to a spec execute on branch `gromit/spec-<spec-name>`, not on main.
- Beads without a spec label execute on main directly.
- After the last bead of a spec closes, the merge pipeline triggers automatically.
- Stage 1 runs the full test suite with all build labels (no tag filtering), all `full_commands`, and acceptance tests when ATDD is enabled.
- Stage 2 (spec conformance) verifies implementation against the spec's acceptance criteria and produces a pass/fail result.
- Stage 3 (code quality) reviews the diff for style, error handling, test quality, and produces a pass/fail result.
- Stage 4 (architectural review) evaluates module sizing, boundary seams, pattern conformance, and produces a pass/fail result.
- Each stage is a hard gate: failure at any stage prevents later stages from running.
- Review failures create fix beads labeled with the spec, which re-enter the run loop.
- The merge pipeline re-triggers from Stage 1 after fix beads complete.
- The retry cycle is capped at a configurable limit (default: 3).
- After all stages pass, the branch is rebased onto main and fast-forward merged.
- Merge conflicts trigger an LLM agent for resolution; unresolvable conflicts block and alert the user.
- The spec branch is deleted after successful merge.

## Decisions

1. **Per-spec branching, not per-bead.** Specs represent coherent units of work that should land atomically. Individual beads within a spec are too granular for branch isolation. Beads without specs are small enough to go directly to main.

2. **Spec conformance review runs first.** No point reviewing code quality or architecture for an implementation that doesn't meet its requirements. This ordering prevents wasted review effort.

3. **Hard gates between review phases.** Each phase must pass before the next runs. This keeps review feedback focused — fix conformance issues before worrying about code style, fix code style before evaluating architecture.

4. **Fix beads re-enter the normal run loop.** Rather than a separate fix mechanism, review failures create spec-labeled beads that the existing run loop processes. This reuses all existing build/validate infrastructure and keeps the fix work visible in bead tracking.

5. **Full pipeline re-run after fixes.** After fix beads complete, the merge pipeline restarts from Stage 1 (validation), not from the failing stage. This ensures fixes don't introduce regressions.

6. **Rebase-then-merge strategy.** Rebasing onto main before merging keeps the commit history linear. The LLM agent is only invoked when conflicts exist, avoiding unnecessary overhead for clean merges.

7. **Architectural review includes module sizing and boundary seams.** Beyond pattern conformance, the architectural review evaluates whether modules are appropriately sized and whether boundaries between components are clean and well-placed.

8. **Configurable retry cap defaults to 3.** Prevents infinite fix-review cycles. Three attempts balances giving the system enough chances to converge against wasting compute on specs that need human intervention.

## Research & Context

### Current State

The spec gate (`internal/runner/spec_gate.go`, `internal/specgate/`) currently fires when the last bead of a spec closes. It runs acceptance tests and an LLM review of acceptance criteria, creating fix beads on failure with a `max_cycles` cap. This merge pipeline replaces and extends that mechanism.

The worktree system (`internal/worktree/worktree.go`) provides branch creation, merge-back, and conflict resolution infrastructure — currently used only for interactive commands (plan, refine, review, retro). The branch naming pattern `gromit/<command>-<timestamp>` exists; this spec introduces `gromit/spec-<spec-name>`.

The review infrastructure (`internal/runner/reviewpkg/reviewer.go`) supports light per-bead reviews and thorough periodic reviews. The merge pipeline's review phases are new — they are blocking gates rather than advisory reviews.

Validation (`internal/runner/validation/`, `internal/validate/`) supports fast per-bead commands and full command suites. The merge pipeline's validation stage uses the full commands but removes build tag filtering to include acceptance and integration tests.

The runner loop (`internal/runner/run_iteration.go`) currently processes beads sequentially on a single branch. Bead routing to different branches based on spec labels is new infrastructure.

### Existing Specs

- `pipeline-stages.md` — defines the abstract Gate/Build/Validate/Review/Epilogue stage model
- `spec-acceptance-verification-loop.md` — the current spec gate mechanism this replaces
- `atdd-methodology.md` — acceptance test methodology that feeds into Stage 1
- `session-epilogue.md` — end-of-session cleanup that runs after all specs complete
