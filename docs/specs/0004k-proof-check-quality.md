# Spec 0004k — Proof Check Quality and Suspect Annotation

## spec_id
proof-check-quality

## Vision

Pattern-matching proof checks (grep, awk, sed over source files) are brittle. A CLI flag registered as `"--title"` in Go/cobra, `@click.option('--title')` in Python, or `.option('--title')` in JS/commander looks entirely different to a source grep, yet all three register the same flag. When the executor writes correct code but in a different style than the planner expected, pattern-matching checks fail while the build passes — and the fix planner, seeing only a generic failure message, generates a new code-implementation task that re-does correct work instead of fixing the proof check.

Two things need to happen. First, the planning and fix-planning prompts must instruct the planner to prefer behavioral proof checks (running the built binary) over source-structure proof checks. Second, when pattern-matching checks fail after all retries while build checks pass, the system must signal to the fix planner that the proof checks themselves are suspect — so it creates a proof-check rewrite task, not another implementation task.

## Summary

Four changes, deployed as a unit:

1. **Rule 7 in both planner prompts**: "Runtime over source-grep for behavioral properties" — prefer `./binary subcommand --help | grep -q -- '--flag-name'` over source grep when verifying CLI flags or other user-visible behaviors.

2. **`isBuildCheck` classifier**: A helper in the task loop that identifies build/compile commands (go build, go vet, cargo build, etc.) vs. pattern-matching commands (grep, awk, sed).

3. **`annotateSuspectProofChecks`**: When all build checks in a task's `ProofChecks` list pass but only pattern-matching checks are failing, each failure message is prefixed with `[suspect-proof-check]` and a human-readable explanation.

4. **Failure propagation and fix-planner instruction**: Annotated failure strings are stored in `TaskResult.Failures`, collected by the execute stage into `FailureContext.Failures` (replacing the generic `"all tasks failed"` string), and the fix-planning prompt instructs the fix planner to respond to `[suspect-proof-check]` failures by rewriting the proof check rather than re-implementing code.

## Goals

### Primary
- Planners prefer behavioral proof checks over source-structure proof checks
- When builds pass but grep/awk checks fail after all retries, failures are annotated `[suspect-proof-check]`
- Annotated failures reach the fix planner via `FailureContext.Failures`
- Fix planner creates proof-check rewrite tasks (not code tasks) in response to `[suspect-proof-check]`

### Secondary
- `isBuildCheck` is language-aware (Go, Rust, Maven, npm, make)
- Annotation is suppressed when any build check is also failing (real implementation problem)
- Annotation is suppressed when no build check exists in the task (no build evidence to infer from)
- Per-task failure strings replace the generic `"all tasks failed"` fallback when failures are available

## Non-goals

- Automatically rewriting proof checks without human/LLM involvement
- Detecting suspect proof checks at plan-generation time
- Annotating partial failures (only annotates when all retries are exhausted)
- Modifying the always-run check pipeline

## Architecture

### Rule 7 in planner prompts (`planner.go`)

`buildPlanPrompt` gains rule 7 in its `## Proof Check Quality Guidelines` section. `buildFixPlanPrompt` gains the same rule in its `## Output Format` proof check quality rules block. Both reference the canonical example: `./binary subcommand --help | grep -q -- '--flag-name'`.

### `isBuildCheck(cmd string) bool` (`taskloop.go`)

Classifies a proof check command as a build/compile check by matching the first two tokens against known build tool invocations:

- `go build`, `go vet`
- `npm run build`
- `cargo build`
- `mvn compile`
- `make build`

Returns false for test runners, grep, awk, sed, and runtime binary invocations.

### `annotateSuspectProofChecks(proofChecks, failures []string) []string` (`taskloop.go`)

Called after all inspection retries are exhausted. Algorithm:

1. If `proofChecks` is empty or `failures` is empty → return failures unchanged
2. If any build check (from `proofChecks`) appears as a substring of any failure → return failures unchanged (build is failing; not a suspect-proof-check situation)
3. If no build check exists in `proofChecks` → return failures unchanged (no build evidence)
4. Otherwise: prefix every failure with `[suspect-proof-check] All build checks pass but pattern-matching checks failed. The implementation may be correct; proof checks may be testing source structure rather than behavior. `

### `TaskResult.Failures []string` (`taskloop.go`)

New field on `TaskResult`, populated immediately after `annotateSuspectProofChecks`. `NormalizeNilFields` maps nil to `[]string{}` for JSON consistency.

### Failure collection in execute stage (`execute.go`)

When all tasks fail, the execute stage collects `r.Failures` from each `TaskResult` into a single slice. If no per-task failures exist (empty or nil across all results), falls back to `["all tasks failed"]`. The collected slice populates `FailureContext.Failures`, making annotated strings visible to the fix planner.

### Fix-planner instruction (`planner.go`)

In `buildFixPlanPrompt`'s `## Instructions` block, a new bullet:

> If a failure message starts with `[suspect-proof-check]`, do NOT create a code implementation task. Instead, create a proof-check rewrite task that replaces the failing pattern-matching check with a behavioral check (e.g. `./binary subcommand --help | grep -q -- '--flag-name'`).

## Acceptance Criteria

1. `buildPlanPrompt` output contains the text `"Runtime over source-grep"` and references `--help`.

2. `buildFixPlanPrompt` output contains the text `"Runtime over source-grep"` and references `--help`.

3. `isBuildCheck("go build ./...")` returns true; `isBuildCheck("grep -q 'func Foo' foo.go")` returns false.

4. When a task has `ProofChecks: ["go build ./...", "grep -q '--title' cmd/foo.go"]` and only the grep check fails (build passes), all failure messages are prefixed with `[suspect-proof-check]`.

5. When a task has the same setup but `go build ./...` also appears in the failures, no annotation is added.

6. When a task has no build check in `ProofChecks`, no annotation is added.

7. After inspection fails on all retries, `TaskResult.Failures` contains the (possibly annotated) failure strings.

8. When all tasks fail in the execute stage and per-task failures exist, `FailureContext.Failures` contains those per-task failure strings (not `"all tasks failed"`).

9. When all tasks fail and no per-task failures exist, `FailureContext.Failures` contains `["all tasks failed"]`.

10. `buildFixPlanPrompt` output contains the text `"[suspect-proof-check]"` and `"proof-check rewrite"` in the Instructions section.

11. All existing tests pass.

## Scenarios

### Scenario: Pattern-matching check fails, build passes — annotation applied

**Given:** a task has `ProofChecks: ["go build ./...", "grep -q '--title' cmd/foo.go"]`
**And:** after all retries, `go build ./...` passes but `grep -q '--title' cmd/foo.go` fails
**When:** the task loop exhausts retries
**Then:** `TaskResult.Failures` contains `"[suspect-proof-check] All build checks pass but pattern-matching checks failed..."`
**And:** the execute stage places this string in `FailureContext.Failures`
**And:** the fix planner receives it and generates a proof-check rewrite task

### Scenario: Build also fails — no annotation

**Given:** a task has `ProofChecks: ["go build ./...", "grep -q '--title' cmd/foo.go"]`
**And:** both `go build ./...` and the grep check appear in failures
**When:** the task loop exhausts retries
**Then:** `TaskResult.Failures` contains the failure messages without `[suspect-proof-check]` prefix
**And:** the fix planner generates a code implementation task (normal fix path)

### Scenario: No build check in task — no annotation

**Given:** a task has `ProofChecks: ["grep -q '--title' cmd/foo.go", "awk '/stepA/' foo.go"]`
**And:** both checks fail
**When:** the task loop exhausts retries
**Then:** `TaskResult.Failures` contains the failure messages without annotation
**And:** the fix planner cannot infer the implementation is correct

### Scenario: Fix planner rewrites proof check

**Given:** `FailureContext.Failures` contains `"[suspect-proof-check]... grep -q '--title' cmd/foo.go: exit status 1"`
**When:** the fix planner generates the fix plan
**Then:** the fix plan contains a task with objective describing a proof-check rewrite
**And:** the fix task's `proof_checks` uses a behavioral check: `./binary --help | grep -q -- '--title'`
**And:** no task is created to re-implement the CLI flag registration

### Scenario: All tasks fail with per-task failures — specific strings reach planner

**Given:** two tasks both fail, each with annotated `[suspect-proof-check]` failures
**When:** the execute stage builds the replan context
**Then:** `FailureContext.Failures` contains both failure strings
**And:** does not contain the generic `"all tasks failed"` string

### Scenario: All tasks fail without per-task failures — generic fallback

**Given:** tasks fail at the runner level (execution error, not inspection failure)
**And:** `TaskResult.Failures` is empty for all failed tasks
**When:** the execute stage builds the replan context
**Then:** `FailureContext.Failures` is `["all tasks failed"]`

## Validation

```
go test ./internal/next/planner/...
go test ./internal/next/specloop/...
go test ./internal/next/specloop/stages/...
go vet ./...
go build ./...
```
