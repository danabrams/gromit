---
created: 2026-02-12T00:00:00Z
decomposed: true
decomposed_at: "2026-02-12T10:02:23-05:00"
id: bead-scope-validation
source_spec: bead-scope-validation
---

# Bead Scope Validation Implementation Plan

**Goal:** Validate proposed bead definitions against sizing rules after decomposition, re-prompting Claude when violations are found, before creating beads.

**Architecture:** A new `internal/validate` package provides pure-function validation (criteria count, sibling overlap, scope-signal keywords) and re-prompt building. Both decompose paths (CLI and runner) call into it, handling LLM re-invocation through their own mechanisms. A new `DecomposeConfig` section controls the behavior.

**Tech Stack:** Go, existing config/prompt/runner patterns

**Spec:** `.gromit/specs/bead-scope-validation.md`

---

## Architecture

### Package: `internal/validate/`

Stateless validation functions with zero LLM cost. Exports:

- `BeadCandidate` — struct with `Title`, `Description`, `AcceptanceCriteria []string` (common denominator of `beadDef` and `SubTask`)
- `Violation` — struct with `BeadIndex int`, `Rule string`, `Message string`
- `CheckBeads([]BeadCandidate) []Violation` — runs all rules, returns violations
- `BuildReprompt(originalPrompt string, candidates []BeadCandidate, violations []Violation) string` — generates re-decompose prompt

Rules:
1. **Criteria count**: Flag beads with >3 acceptance criteria
2. **Sibling overlap**: Flag beads with criteria that are substrings of a sibling's criteria (case-insensitive)
3. **Scope signals**: Flag beads whose title or description contains phrases like "refactor entire", "update all", "across all packages", "and also" (case-insensitive)

### Config: `DecomposeConfig`

```yaml
decompose:
  validate: true              # Enable/disable validation (default: true)
  max_validation_retries: 2   # Max re-prompt attempts (default: 2)
```

Uses `*bool` pattern for `validate` (defaults to true when nil), matching `ScopeCheckConfig.BlockOversized`.

### Integration

**CLI path** (`cmd/gromit/decompose.go`):
After `jsonutil.ExtractJSON` parses `[]beadDef`, convert to `[]BeadCandidate`, call `validate.CheckBeads`. On violations, call `validate.BuildReprompt` and re-invoke `claudeClient.Run()`. Loop up to `MaxValidationRetries`. `--skip-validation` flag bypasses entirely.

**Runner path** (`internal/runner/process.go`):
In `attemptDecomposition()`, after `DecomposeTask()` returns `[]SubTask`, convert to `[]BeadCandidate`, call `validate.CheckBeads`. On violations, rebuild prompt and re-invoke via router. Loop up to config limit. Respects `cfg.Decompose.IsValidateEnabled()`.

## Test Strategy

**Unit tests** in `internal/validate/`: Pure function tests for each rule — positive/negative cases, edge cases (empty criteria, single bead, partial keyword matches). Re-prompt builder tests verify output includes violations and original definitions.

**Config tests**: Extend `internal/config/config_test.go` for `DecomposeConfig` defaults and YAML parsing.

**Integration**: Wiring in decompose.go and process.go tested through existing mock patterns only if non-trivial logic beyond calling `validate.CheckBeads`.

## Implementation Tasks

### Task 1: Create validation rules and types

**Files:**
- Create: `internal/validate/validate.go`
- Create: `internal/validate/validate_test.go`

**What to Do:**
Define `BeadCandidate` struct (Title, Description, AcceptanceCriteria), `Violation` struct (BeadIndex, Rule, Message), and `CheckBeads([]BeadCandidate) []Violation`. Implement three rules: criteria count (>3 flags), sibling overlap (case-insensitive substring match across siblings' criteria), and scope signals (configurable keyword list with conservative defaults). Export a `DefaultScopeSignals` variable with the initial keyword list.

**Acceptance Criteria:**
- `CheckBeads` returns empty slice for well-formed beads and correct violations for each rule
- All three rules are independently testable and combined correctly
- Scope-signal matching is case-insensitive

**Dependencies:** None

### Task 2: Create re-prompt builder

**Files:**
- Create: `internal/validate/reprompt.go`
- Create: `internal/validate/reprompt_test.go`

**What to Do:**
Implement `BuildReprompt(originalPrompt string, candidates []BeadCandidate, violations []Violation) string`. The output should include the original prompt context, list violations grouped by bead index, instruct Claude to re-decompose only flagged beads while keeping valid ones unchanged, and request the same JSON output format.

**Acceptance Criteria:**
- Output includes violation details per flagged bead
- Output instructs to keep unflagged beads unchanged
- Output requests same JSON array format as original decompose prompt

**Dependencies:** Task 1 (uses Violation and BeadCandidate types)

### Task 3: Add DecomposeConfig to config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `gromit.yaml`

**What to Do:**
Add `DecomposeConfig` struct with `Validate *bool` and `MaxValidationRetries int`. Add `IsValidateEnabled()` method (returns true when nil). Add to `Config` struct as `Decompose DecomposeConfig yaml:"decompose"`. Wire into `SetDefaults()` (default MaxValidationRetries to 2). Add `decompose:` section to gromit.yaml with comments.

**Acceptance Criteria:**
- `IsValidateEnabled()` returns true when Validate is nil, respects explicit false
- `SetDefaults()` sets MaxValidationRetries to 2 when zero
- YAML round-trips correctly

**Dependencies:** None

### Task 4: Wire validation into CLI decompose path

**Files:**
- Modify: `cmd/gromit/decompose.go`

**What to Do:**
Add `--skip-validation` flag to decomposeCmd. In `decomposeSinglePlan`, after JSON parsing (line 196) and before bead creation (line 222), add a validation loop: convert `[]beadDef` to `[]validate.BeadCandidate`, call `validate.CheckBeads`, if violations exist and not skip-validation, call `validate.BuildReprompt`, re-invoke `claudeClient.Run`, re-parse, re-validate, up to `cfg.Decompose.MaxValidationRetries` times. If violations persist after retries, print warning and proceed. Log violation counts and re-prompt attempts.

**Acceptance Criteria:**
- `--skip-validation` bypasses all validation checks
- Violations trigger re-prompt with Claude, up to configured retry limit
- After exhausting retries, beads are created with a warning (pipeline not blocked)

**Dependencies:** Tasks 1, 2, 3

### Task 5: Wire validation into runner decompose path

**Files:**
- Modify: `internal/runner/process.go`

**What to Do:**
In `attemptDecomposition()`, between `DecomposeTask()` (line 563) and `CreateSubBeads()` (line 573), add a validation loop: convert `[]SubTask` to `[]validate.BeadCandidate`, call `validate.CheckBeads`, if violations exist and `cfg.Decompose.IsValidateEnabled()`, rebuild the decompose prompt with violations, re-invoke via `r.DecomposeTask` pattern (render prompt, call provider), re-parse, re-validate, up to config limit. If violations persist, log warning and proceed to `CreateSubBeads`. Convert validated candidates back to `[]SubTask` if re-decomposition produced new output.

**Acceptance Criteria:**
- Validation runs when `decompose.validate` is true (default) and skips when false
- Re-prompt uses the same provider/tier as the original decomposition
- After exhausting retries, proceeds to CreateSubBeads with warning

**Dependencies:** Tasks 1, 2, 3

---

## Notes

- The `BeadCandidate` type is intentionally minimal — only fields needed for validation. Both `beadDef` and `SubTask` have additional fields (priority, dependencies) that pass through unchanged.
- Scope-signal keywords should start conservative: "refactor entire", "update all", "across all packages", "and also", "complete rewrite". Better to miss edge cases than trigger false-positive re-prompts that cost LLM calls.
- The sibling overlap check uses substring matching, not semantic similarity. This catches copy-paste criteria but won't catch semantically equivalent but differently-worded criteria. This is intentional for V1 — keeps it zero-LLM-cost.
- When re-prompting, the original prompt context is included so Claude has full context. The violations serve as additional constraints, not a replacement for the original instructions.
