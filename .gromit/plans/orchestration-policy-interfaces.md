---
created: 2026-02-18T00:00:00Z
decomposed: true
decomposed_at: "2026-02-18T18:04:54Z"
id: orchestration-policy-interfaces
source_spec: orchestration-policy-interfaces
---

# Orchestration Policy Interfaces Implementation Plan

**Goal:** Extract four policy interfaces from the runner to separate orchestration policy (what to do) from execution mechanism (how to do it), enabling isolated policy testing and eliminating the policy-in-mechanism coupling that caused two recent reverts.

**Architecture:** Create a new `internal/runner/policy/` sub-package with four interfaces (`StuckPolicy`, `EscalationPolicy`, `ValidationPolicy`, `MethodologyPolicy`), each with a config-backed default implementation. The Runner struct gains four policy fields wired at construction, and call sites migrate from direct config reads to policy calls.

**Tech Stack:** Go, existing config/runner/escalation/validation/methodology packages

**Spec:** `.gromit/specs/orchestration-policy-interfaces.md`

---

## Architecture

### Package Structure

New package `internal/runner/policy/` with four files (one per interface):
- `stuck.go` — StuckPolicy + ThresholdStuckPolicy
- `escalation.go` — EscalationPolicy + ConfigEscalationPolicy
- `validation.go` — ValidationPolicy + ConfigValidationPolicy + GateType
- `methodology.go` — MethodologyPolicy + ConfigMethodologyPolicy

### Runner Integration

The Runner struct gains four fields following the existing `escalationHandler`/`validationRunner` pattern:

```go
stuckPolicy       policy.StuckPolicy
escalationPolicy  policy.EscalationPolicy
validationPolicy  policy.ValidationPolicy
methodologyPolicy policy.MethodologyPolicy
```

Default implementations are constructed from `*config.Config` in both `constructor.go` and `constructor_with_deps.go`. The `Deps` struct gains optional policy fields (nil means use config defaults).

### Call Site Migration

| Before | After | File |
|--------|-------|------|
| `r.isStuckBeadWithStats(b, stats)` | `r.stuckPolicy.IsStuck(b.ID, stats)` | lifecycle.go |
| `escalation.SelectTier(r.cfg, b)` | `r.escalationPolicy.SelectInitialTier(b.Priority, b.Labels)` | process.go |
| `escalation.SelectModel(r.cfg, b)` | `r.escalationPolicy.SelectModel(tier, b.Labels)` | process.go |
| `r.cfg.NextEscalationTier(savedTier)` | `r.escalationPolicy.NextTier(savedTier)` | callbacks.go |
| Timeout branches in `handleInvokeError` | `r.escalationPolicy.ClassifyTimeout(parentCancelled, stallFired)` | callbacks.go |
| `bc.MaxRetries = 0` pattern | `r.validationPolicy.MaxRecoveryAttempts()` | callbacks.go |
| `mandatoryQualityGateCommandPrefixes` var | `r.validationPolicy.MandatoryCommandPrefixes()` | lifecycle.go |
| `r.successesSinceFull >= everyN` logic | `r.validationPolicy.SelectGate(consecutiveSuccesses)` | process.go |
| `bead.IsMethodologyActive(...)` | `r.methodologyPolicy.IsActive(labels, "atdd")` | process_methodology.go |
| `r.cfg.Methodology.ResolvePhaseTimeoutSeconds(...)` | `r.methodologyPolicy.PhaseTimeout(phase, beadTimeoutSec)` | process_methodology.go |
| `minRefactorTime` / `minRevalidationTime` constants | `r.methodologyPolicy.MinRefactorBudget()` / `.MinRevalidationBudget()` | process_methodology.go |
| `!atddActive && !tddActive` deferral check | `r.methodologyPolicy.ShouldDeferPostSuccess(atddActive, tddActive)` | process_methodology.go |

## Test Strategy

### Unit Tests (per-policy, isolated)
Each policy interface gets table-driven tests in `internal/runner/policy/*_test.go`. Pure logic, no runner setup.

### Key Test Cases
- **StuckPolicy:** threshold boundary (2 < 3, 3 >= 3), disabled (threshold=0), missing stats
- **EscalationPolicy:** priority→tier mapping, label overrides, chain traversal, ClassifyTimeout all four branches (stall/bead/parent/invocation), retry limits from config
- **ValidationPolicy:** gate selection modulo arithmetic (boundaries, disabled), recovery attempts, mandatory commands passthrough
- **MethodologyPolicy:** label true/false/absent, phase timeout fallback, budget durations, deferral logic

### Integration
Existing runner tests updated to inject mock policies where they currently set config fields. No new runner-level test files.

### Compile-time Checks
`var _ StuckPolicy = (*ThresholdStuckPolicy)(nil)` etc. in the policy package.

---

## Implementation Tasks

### Task 1: Implement StuckPolicy interface and default

**Files:**
- Create: `internal/runner/policy/stuck.go`
- Create: `internal/runner/policy/stuck_test.go`

**What to Do:**
Define `StuckPolicy` interface with `IsStuck(beadID string, stats logger.BeadStats) bool`. Implement `ThresholdStuckPolicy` struct with a `threshold int` field. Constructor: `NewThresholdStuckPolicy(threshold int) *ThresholdStuckPolicy`. Logic: return false if threshold <= 0 (disabled), otherwise `stats.Failures >= threshold`. Add compile-time interface check.

**Acceptance Criteria:**
- `ThresholdStuckPolicy` returns true when failures >= threshold, false otherwise
- Threshold <= 0 disables stuck detection (always returns false)
- Missing/zero-value stats returns false

**Dependencies:** None

### Task 2: Implement EscalationPolicy interface and default

**Files:**
- Create: `internal/runner/policy/escalation.go`
- Create: `internal/runner/policy/escalation_test.go`

**What to Do:**
Define `EscalationPolicy` interface with six methods: `SelectInitialTier(priority int, labels []string) string`, `SelectModel(tier string, labels []string) string`, `NextTier(currentTier string) string`, `MaxRetriesPerModel() int`, `MaxRetriesPerBead() int`, `ClassifyTimeout(parentCancelled bool, stallFired bool) string`. Implement `ConfigEscalationPolicy` wrapping `*config.Config`. `ClassifyTimeout` encapsulates the three-way branch: stall fired → "stall", bead ctx done + parent alive → "bead", parent done → "parent", else → "invocation". Delegates `SelectInitialTier` to `cfg.SelectTier()`, `NextTier` to `cfg.NextEscalationTier()`, retry limits to config fields.

**Acceptance Criteria:**
- `SelectInitialTier` delegates to config priority/label logic correctly
- `ClassifyTimeout` returns correct classification for all four branches
- `NextTier` returns next in chain or empty string at end

**Dependencies:** None

**Notes:** ClassifyTimeout is the most critical method — this was the source of two reverts. The test suite must exhaustively cover every branch combination.

### Task 3: Implement ValidationPolicy interface and default

**Files:**
- Create: `internal/runner/policy/validation.go`
- Create: `internal/runner/policy/validation_test.go`

**What to Do:**
Define `GateType` type (string) with constants `GateFast` and `GateFull`. Define `ValidationPolicy` interface with four methods: `SelectGate(consecutiveSuccesses int) GateType`, `MaxRecoveryAttempts() int`, `ShouldEscalateRecovery() bool`, `MandatoryCommandPrefixes() []string`. Implement `ConfigValidationPolicy` wrapping relevant config fields. `SelectGate` returns `GateFull` when `fullEveryN > 0 && consecutiveSuccesses % fullEveryN == 0`, else `GateFast`. `MaxRecoveryAttempts` returns 2. `MandatoryCommandPrefixes` returns configured list or default `["go test", "go vet", "go build"]`.

**Acceptance Criteria:**
- `SelectGate` returns Full at correct intervals and Fast otherwise
- `SelectGate` with fullEveryN=0 always returns Fast
- `MandatoryCommandPrefixes` returns configured or default list

**Dependencies:** None

### Task 4: Implement MethodologyPolicy interface and default

**Files:**
- Create: `internal/runner/policy/methodology.go`
- Create: `internal/runner/policy/methodology_test.go`

**What to Do:**
Define `MethodologyPolicy` interface with five methods: `IsActive(labels []string, methodology string) bool`, `PhaseTimeout(phase string, beadTimeoutSec int) int`, `MinRefactorBudget() time.Duration`, `MinRevalidationBudget() time.Duration`, `ShouldDeferPostSuccess(atddActive, tddActive bool) bool`. Implement `ConfigMethodologyPolicy` wrapping `*config.Config`. `IsActive` delegates to the same label-scanning + config-fallback logic currently in `bead.IsMethodologyActive`. `PhaseTimeout` delegates to `cfg.Methodology.ResolvePhaseTimeoutSeconds`. Budget methods return the current constant values (60s, 30s). `ShouldDeferPostSuccess` returns `!atddActive && !tddActive`.

**Acceptance Criteria:**
- `IsActive` honors label overrides ("atdd:true"/"atdd:false") over config default
- `PhaseTimeout` falls back to bead timeout when phase timeout is unset
- `ShouldDeferPostSuccess` returns true only when both methodologies are inactive

**Dependencies:** None

### Task 5: Wire policy fields into Runner struct and constructors

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/constructor.go`
- Modify: `internal/runner/constructor_with_deps.go`

**What to Do:**
Add four policy fields to the Runner struct: `stuckPolicy`, `escalationPolicy`, `validationPolicy`, `methodologyPolicy`. In `newRunnerImpl` (constructor.go), construct default implementations from `r.cfg` and assign to the policy fields after the Runner struct literal. In `Deps` struct (constructor_with_deps.go), add optional policy fields. In `newRunnerWithDepsImpl`, use provided policies or fall back to config defaults when nil.

**Acceptance Criteria:**
- Runner struct has four policy fields of the correct interface types
- `NewRunner` produces a Runner with config-backed default policies
- `NewRunnerWithDeps` uses provided policies or defaults when nil
- Existing tests compile and pass without modification (additive change only)

**Dependencies:** Tasks 1-4

### Task 6: Migrate stuck detection and mandatory commands in lifecycle.go

**Files:**
- Modify: `internal/runner/lifecycle.go`
- Modify: existing tests that test stuck detection / mandatory commands

**What to Do:**
Replace `isStuckBeadWithStats` method body with delegation to `r.stuckPolicy.IsStuck(b.ID, stats)`. Keep the method as a thin wrapper for nil-guard compatibility, or inline the policy call at the call site. Delete the `mandatoryQualityGateCommandPrefixes` package-level var. Update `missingMandatoryQualityCommands` to accept a `mandatoryPrefixes []string` parameter. Update `enforceMandatoryQualityGateCoverage` to call `r.validationPolicy.MandatoryCommandPrefixes()` and pass result to `missingMandatoryQualityCommands`. Add early return when the list is empty.

**Acceptance Criteria:**
- `isStuckBeadWithStats` delegates to `r.stuckPolicy.IsStuck()`
- `mandatoryQualityGateCommandPrefixes` package-level var is deleted
- `enforceMandatoryQualityGateCoverage` sources prefixes from `r.validationPolicy`
- Existing tests pass

**Dependencies:** Task 5

### Task 7: Migrate escalation and timeout classification in process.go and callbacks.go

**Files:**
- Modify: `internal/runner/process.go`
- Modify: `internal/runner/callbacks.go`

**What to Do:**
In `process.go`, replace `escalation.SelectTier(r.cfg, b)` with `r.escalationPolicy.SelectInitialTier(b.Priority, b.Labels)` and `escalation.SelectModel(r.cfg, b)` with `r.escalationPolicy.SelectModel(tier, b.Labels)`. In `callbacks.go` `handleInvokeError`, replace the inline timeout classification with `r.escalationPolicy.ClassifyTimeout(bc.ParentCtx.Err() != nil, invResult.StallFired)` and branch on the returned string. In `makeValidationExecuteFn`, replace `r.cfg.NextEscalationTier(savedTier)` with `r.escalationPolicy.NextTier(savedTier)`.

**Acceptance Criteria:**
- No remaining `escalation.SelectTier` / `escalation.SelectModel` calls in process.go
- `handleInvokeError` timeout logic delegates to `ClassifyTimeout`
- `makeValidationExecuteFn` uses `escalationPolicy.NextTier`
- Existing tests pass

**Dependencies:** Task 5

**Notes:** The `handleInvokeError` migration is the highest-risk change. The ClassifyTimeout return values must map exactly to the existing branch behavior.

### Task 8: Migrate validation gate selection and recovery bounds

**Files:**
- Modify: `internal/runner/process.go`
- Modify: `internal/runner/callbacks.go`

**What to Do:**
In `process.go` `maybeRunPeriodicFullValidation`, replace the `successesSinceFull >= everyN` check with `r.validationPolicy.SelectGate(r.successesSinceFull)` returning `GateFull`. In `callbacks.go` `makeValidationExecuteFn`, replace the hardcoded `bc.MaxRetries = 0` + single-escalation pattern with a loop bounded by `r.validationPolicy.MaxRecoveryAttempts()`, using `r.validationPolicy.ShouldEscalateRecovery()` to decide whether to attempt tier escalation.

**Acceptance Criteria:**
- Gate selection uses `validationPolicy.SelectGate()` instead of inline arithmetic
- Recovery attempts bounded by `validationPolicy.MaxRecoveryAttempts()`
- Behavior identical to current: 1 quick + 1 escalated attempt

**Dependencies:** Task 5

### Task 9: Migrate methodology policy call sites

**Files:**
- Modify: `internal/runner/process_methodology.go`

**What to Do:**
Replace `bead.IsMethodologyActive(bc.Bead.Labels, "atdd", r.cfg.Methodology.ATDD)` with `r.methodologyPolicy.IsActive(bc.Bead.Labels, "atdd")` (and same for "tdd"). Replace all `r.cfg.Methodology.ResolvePhaseTimeoutSeconds(phase, beadTimeout)` calls with `r.methodologyPolicy.PhaseTimeout(phase, beadTimeout)`. Replace `minRefactorTime` constant usage with `r.methodologyPolicy.MinRefactorBudget()` and `minRevalidationTime` with `r.methodologyPolicy.MinRevalidationBudget()`. Replace inline `!atddActive && !tddActive` deferral check with `r.methodologyPolicy.ShouldDeferPostSuccess(atddActive, tddActive)`. Delete the `minRefactorTime` and `minRevalidationTime` constants.

**Acceptance Criteria:**
- No remaining `bead.IsMethodologyActive` calls in process_methodology.go
- No remaining `cfg.Methodology.ResolvePhaseTimeoutSeconds` calls in process_methodology.go
- `minRefactorTime` and `minRevalidationTime` constants deleted
- Existing tests pass

**Dependencies:** Task 5

### Task 10: Update runner tests and final verification

**Files:**
- Modify: `internal/runner/process_test.go`
- Modify: other affected test files as needed

**What to Do:**
Update existing runner tests that set config fields for policy decisions (e.g., `StuckBeadThreshold`, `FullValidationEveryN`) to also work with the new policy-field-based wiring. Where tests construct Runner structs directly, ensure policy fields are set (either via config defaults through constructor or explicit mock policies). Run full test suite (`go test ./...`), `go vet ./...`, and `go build ./...` to confirm everything passes. Grep for any remaining direct config reads that should have been migrated.

**Acceptance Criteria:**
- `go test ./...` passes
- `go vet ./...` passes
- `go build ./...` passes
- No remaining direct config reads for policy decisions outside the policy package

**Dependencies:** Tasks 6-9

---

## Notes

- **Risk hotspot:** Task 7 (timeout classification migration) is highest risk. The `ClassifyTimeout` encapsulation must exactly replicate the existing three-way branch behavior. The two previous reverts were caused by changes in this area.
- **Backward compatibility:** No config YAML changes. No constructor signature changes for existing callers. Policies default to config-backed implementations.
- **The `escalation.SelectTier` / `escalation.SelectModel` functions in `internal/runner/escalation/tierselect.go` can be deprecated but don't need to be deleted immediately** — they may still be used by other callers outside the runner. Check during implementation.
- **Tasks 1-4 are fully independent** and can be parallelized during decompose.
- **Tasks 6-9 are independent of each other** (each touches different files) and can also be parallelized after Task 5 completes.
