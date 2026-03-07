---
id: validation-auto-fix-integration
source_ideas: ["idea-1772534537434"]
created: 2026-03-03
accepted: true
---

# Integrated Validation Auto-Fixes

## Specification

Integrate "trivial auto-fixes" (specifically `gofmt` and `goimports`) into the standard Gromit build loop. Currently, these fixes are implemented in the `internal/runner/validation` package but are primarily used in the integration queue and tests, rather than the core `Orchestrator` pipeline.

### Core Mechanism

When the **Validate** stage (Stage 3) detects a failure in the standard build loop, it should attempt to resolve the failure using local tools before reporting the failure back to the Orchestrator for an LLM-based repair.

1.  **Detection:** The `Validate` stage identifies a non-zero exit code from a validation command (e.g., `go test` or `golangci-lint`).
2.  **Auto-Fix Attempt:** If auto-fixes are enabled, the stage runs `gofmt -w` and `goimports -w` on all Go files modified since the `StartCommit`.
3.  **Re-Validation:** After the auto-fix, the stage immediately re-runs the failed validation commands.
4.  **Outcome Mapping:**
    - If re-validation passes: The stage returns `Proceed` and sets `bc.Result.TrivialAutoFixed = true`.
    - If re-validation fails: The stage returns `Block` with the original (or updated) failure output, allowing the standard Orchestrator retry loop to trigger an LLM-based repair.

### Architectural Changes

- **AutoFix Implementation:** Implement a production-ready `AutoFixFn` that identifies changed files via `git diff --name-only <StartCommit>` and applies formatting tools.
- **Pipeline Update:** Enhance `internal/pipeline/validate.Validate` to accept and execute an `AutoFixFn`.
- **Wiring:** Update `internal/runner/constructor.go` to provide a concrete `AutoFixFn` to the `Validate` stage.
- **Observability:** Ensure the `TrivialAutoFixed` flag is correctly propagated to the `IterationLog` to track the efficiency gains of avoiding LLM invocations for simple formatting issues.

## Acceptance Criteria

- [ ] A production `AutoFixFn` is implemented and verified to run `gofmt` and `goimports` on changed files.
- [ ] The `Validate` stage attempts auto-fixes upon command failure before returning a `Block` decision.
- [ ] If an auto-fix resolves the validation failure, the iteration continues without an LLM-based repair (no second `Build` stage invocation).
- [ ] Successful auto-fixes are recorded in `iteration-*.jsonl` via the `trivial_auto_fixed` field.
- [ ] The mechanism respects the `MaxValidationRetries` configuration (auto-fix counts as a "local" attempt or is integrated into the first pass).
- [ ] Unit tests in `internal/pipeline/validate` verify the auto-fix/re-validate sequence.

## Decisions

1.  **Target changed files only:** To avoid unnecessary I/O and potential side effects in unrelated packages, auto-fixes should only target files touched in the current iteration.
2.  **Run before LLM repair:** Local, deterministic tools are significantly cheaper and faster than LLM invocations. They must always be the first line of defense.
3.  **Fail-safe:** If the auto-fix tool itself fails (e.g., `goimports` cannot find a package), the system should ignore the error and proceed to LLM-based repair.

## Research & Context

### Current State
- `internal/runner/validation/runner.go` contains `RunWithRecovery` which already implements this logic, but this `Runner` is not currently used as the implementation for the `Validate` pipeline stage.
- `internal/pipeline/validate/validate.go` is the current implementation used by the `Orchestrator`, but it is a "dumb" runner that only executes commands and returns results.

### Relevant Files
- `internal/runner/orchestrator.go`: The main run loop coordination.
- `internal/pipeline/validate/validate.go`: The stage where auto-fixes should be triggered.
- `internal/runner/constructor.go`: Where dependencies are wired.
- `internal/runner/validation/runner.go`: Reference implementation for recovery logic.
