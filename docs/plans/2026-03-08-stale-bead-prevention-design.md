# Stale Bead Prevention Design

## Problem

When the implementation takes a different architectural path than the decomposition planned, beads become orphaned: their intent is satisfied but they remain open because no code matches their structural criteria. The run loop rebuilds them endlessly, wasting API calls.

Three failure modes combine:

1. **Accept stage diffs only uncommitted changes.** After beads commit and squash, `git diff HEAD` returns empty. Gap analysis sees "nothing done" and fails all criteria.
2. **Gate checks status, not satisfaction.** Gate skips closed beads but never asks whether an open bead's work is already done by other means.
3. **Decompose ties beads to file structure.** Acceptance criteria reference specific files/functions. When the implementation consolidates, criteria become unfulfillable.

## P0: Cumulative Diff in Accept Stage

### Change

Add `DiffFromBase(ctx, worktree) (string, error)` to `ExecGitAdapter`. Accept stage calls this instead of `Diff`.

### Mechanism

- On worktree creation (`Checkout`), store the base commit SHA to `.gromit/v2/branch-base` in the worktree.
- `DiffFromBase` reads the stored base SHA, runs `git diff <base>..HEAD` to capture all committed + uncommitted changes since the branch point.
- Fallback: if no base file exists (legacy worktrees), fall back to `git diff HEAD`.

### Why not git merge-base?

The worktree branch may have been rebased or force-updated. Storing the base at creation time is more reliable.

### Files

- `internal/v2/adapter/git/exec_git_adapter.go` -- add `DiffFromBase`, update `Checkout` to write base file
- `internal/v2/stage/accept/accept.go` -- call `DiffFromBase` instead of `Diff`

## P1: Pre-Build Satisfaction Check in Gate

### Change

After dependency check passes but before `DecisionProceed`, gate optionally evaluates the bead's acceptance criteria against the current worktree state via LLM.

### Tier Escalation by Generation

| Generation | Tier   | Model  | Rationale                                           |
|------------|--------|--------|-----------------------------------------------------|
| 0          | skip   | none   | First pass, beads are fresh, no redundancy possible |
| 1          | low    | haiku  | Cheap sanity check on remediation beads             |
| 2          | medium | sonnet | More careful evaluation after two passes            |
| 3+         | high   | opus   | Expensive but we're burning API calls on stale work |

### False Positive Guards

Refactor and test beads change code structure or add tests without changing observable behavior. A satisfaction check asking "does this behavior exist?" would false-positive.

Guard: if bead title/description matches refactor or test patterns (contains "refactor", "reorganize", "extract", "move", "rename", "add test", "test coverage"), skip the satisfaction check and always proceed. Keyword heuristic, not LLM -- cheap and deterministic.

### Evaluation

Evaluate the bead's `acceptance_criteria` against a cumulative diff from base (reuses P0 `DiffFromBase`). All criteria pass -> close bead + `DecisionSkip`. Any fail -> `DecisionProceed`.

### Backwards Compatibility

Gate's constructor takes optional `LLMProvider` and `GitDiffer` params. When nil, satisfaction check is skipped.

### Files

- `internal/v2/stage/gate/gate.go` -- add satisfaction check logic, generation-based tier selection, refactor/test guard
- `internal/v2/stage/gate/gate_test.go` -- unit tests

## P2: Behavioral Bead Definitions in Decompose Prompt

### Change

Update decompose prompt templates to require behavioral acceptance criteria. Add validation rule to catch structural criteria.

### Prompt Addition

```
acceptance_criteria: each criterion MUST describe an observable behavior or
capability, NOT a file path, function name, or code structure.

Good: "debug command identifies root cause category from event log"
Bad: "create internal/v2/debug/diagnose.go with Diagnose() function"

The implementation may consolidate or restructure deliverables -- criteria must
remain valid regardless of how the code is organized.
```

Applied to both `defaultDecomposePromptTemplate` and `remediationDecomposePromptTemplate`.

### Validation

Add a `validate.Violation` rule that flags acceptance criteria containing file paths (regex for `/` or `.go`, `.ts`, etc.) as a soft warning.

### expected_outputs vs acceptance_criteria

`expected_outputs` drive TDD cycles and are inherently structural -- that's fine. `acceptance_criteria` are what gate's satisfaction check evaluates. Expected outputs say what to build; acceptance criteria say how to verify it worked.

### Files

- `internal/v2/stage/decompose/decompose.go` -- update prompt templates
- `internal/validate/decompose.go` -- add file-path-in-criteria violation rule

## Verification Plan

### P0 Tests

- Unit: `DiffFromBase` returns cumulative diff from stored base SHA
- Unit: `DiffFromBase` falls back to `git diff HEAD` when no base file exists
- Unit: Accept stage with `DiffFromBase` passes criteria satisfied by earlier committed beads
- Integration: Create worktree, commit across 3 beads, squash, run accept, all criteria pass

### P1 Tests

- Unit: Gen 0 skips satisfaction check
- Unit: Gen 1 uses low tier, gen 2 medium, gen 3+ high
- Unit: Bead with "refactor" in title skips satisfaction check
- Unit: Bead with "add test" in title skips satisfaction check
- Unit: All criteria pass -> bead closed + DecisionSkip
- Unit: Any criterion fails -> DecisionProceed
- Unit: Nil LLMProvider skips satisfaction check (backwards compat)
- Integration: Bead for already-implemented work -> gate closes without building

### P2 Tests

- Unit: Validation flags acceptance criteria containing file paths
- Unit: Updated prompt produces behavioral criteria (mock LLM, verify no file-path criteria)

### End-to-End

- Full scenario: spec with completed tasks, orphaned beads from different architecture. Gate closes stale beads at gen 1+ instead of rebuilding.

### Manual

- Run `gromit run2` on immutable-pipeline spec after fixes. Confirm 4 orphaned beads get closed by gate satisfaction check.

## Appendix Tracking

These changes should be added to the v2 run loop appendix for future reference.
