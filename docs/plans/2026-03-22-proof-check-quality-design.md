# Design: Proof Check Quality Improvements

## Problem

Two related failure modes in the proof check system:

1. **Planner generates source-pattern checks for behavioral properties.** When verifying that a CLI flag, endpoint, or command exists, the LLM generates `grep -q '--flag'` against source files. Source patterns vary by language and framework (`"title"` in Go/cobra, `@click.option('--title')` in Python, `.option('--title')` in JS). The running binary is canonical; the source pattern is fragile.

2. **No diagnostic when all build checks pass but grep checks fail.** When a task exhausts retries and fails, the system marks it failed without distinguishing "compilation is broken" from "implementation is correct but proof check pattern doesn't match." The fix planner gets no signal that re-implementing the code is the wrong response — it should instead rewrite the proof checks.

Both were observed in run-646a0249fef42569: `t-021` had `grep -q '--title'` and `grep -q '--change'` against `review_proposals.go`, but cobra flag registration uses `"title"` and `"change"` (no dashes). The implementation was correct; the proof checks were wrong.

## Design

### Fix 1: Planner prompt — rule 7 (both plan and fix-plan prompts)

Add to the Proof Check Quality Guidelines in `buildPlanPrompt` and `buildFixPlanPrompt`:

> **7. Runtime over source-grep for behavioral properties**: When verifying that a CLI flag, endpoint, command, or other user-visible behavior exists, check the *running artifact*, not the source code. Source patterns vary by language and framework. The runtime is canonical. Example: `./binary subcommand --help | grep -q -- '--flag-name'`. Use source grep only for implementation structure (e.g. call sites, ordering) where no runtime check is possible.

This is language-agnostic and project-agnostic: it works whether the project uses Go/cobra, Python/click, JS/commander, or anything else. It also aligns with the existing rule 2 ("behavioral over presence") by extending that principle to CLI surfaces specifically.

### Fix 2: Suspect-proof-check diagnostic in taskloop

After all retries are exhausted and `ir.Pass` is still false (`taskloop.go`, around line 310), classify the failing checks before marking the task failed:

- **If any failing check is a build/compile check** (`go build`, `npm run build`, `cargo build`, etc.) → genuine failure, no change to behavior.
- **If ALL failing checks are pattern-matching** (grep, awk, string search) AND all compile checks pass → append a diagnostic prefix to each failure message: `[suspect-proof-check] All build checks pass but pattern-matching checks failed. The implementation may be correct; proof checks may be testing source structure rather than behavior.`

The task still gets marked failed — we don't reduce pressure. But the fix planner sees the diagnostic and can generate a proof-check repair task rather than a code re-implementation task.

**Classification heuristic**: a check is a "compile check" if it matches any of: `go build`, `go vet`, `npm run build`, `cargo build`, `mvn compile`, `make build`. A check is "pattern-matching" otherwise (grep, awk, sed, etc.). This is a best-effort heuristic — false classification is safe since we only change the diagnostic message, not the pass/fail outcome.

## What this does NOT do

- Does not auto-pass tasks with suspect proof checks (would reduce pressure).
- Does not add a new LLM invocation to validate proof checks (too expensive per task).
- Does not rewrite proof checks automatically (deferred — requires more design).

## Alignment with vision

From `docs/quality-backpressure-vision.md`:
> "A manually discovered failure mode should be treated as incomplete systemization, not as an unavoidable cost of shipping."
> "Every issue that a human catches after a run should have a path to becoming durable system pressure."

Fix 1 converts this manual catch into a durable planner heuristic. Fix 2 surfaces the diagnostic so future fix planners can see the pattern and self-correct.

## Files changed

- `internal/next/planner/planner.go` — `buildPlanPrompt` and `buildFixPlanPrompt`
- `internal/next/specloop/taskloop.go` — failure classification after retry exhaustion
- Tests for both
