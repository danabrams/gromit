---
id: orchestration-policy-interfaces
epic: run-loop-reliability
---

# Orchestration Policy Interfaces

**Status:** Draft
**Created:** 2026-02-18
**Backlog:** idea-1771180135243

## Problem

The runner mixes orchestration policy (what to do) with execution mechanism (how to do it). This coupling causes real problems:

- **Two recent reverts** (b379fac, 74b0e07) where timeout classification logic tangled with invocation callbacks broke the retry loop.
- **callbacks.go** is a 486-line bottleneck with 10 nested closures across 3 callback factories that mix policy into mechanism.
- **No isolated policy tests.** Every policy decision (stuck detection, escalation tier, validation gate selection, recovery attempt bounds) requires full runner setup to test.
- **Policy duplication.** Phase timeout attribution is wired separately in the validation gate and acceptance verification gate.

## Specification

Extract four policy interfaces from the runner. Each interface owns one decision domain. Default implementations delegate to existing config fields. The runner calls policy interfaces instead of reaching into config or embedding decisions inline.

### StuckPolicy

Decides whether a bead is stuck.

```go
type StuckPolicy interface {
    IsStuck(beadID string, stats logger.BeadStats) bool
}
```

**Replaces:** `isStuckBeadWithStats` in `lifecycle.go:97-115`, which reads `r.cfg.Loop.StuckBeadThreshold` directly.

**Default implementation:** `ThresholdStuckPolicy` returns `stats.Failures >= threshold`.

### EscalationPolicy

Decides tier selection, retry limits, and timeout classification.

```go
type EscalationPolicy interface {
    SelectInitialTier(priority int, labels []string) string
    SelectModel(tier string, labels []string) string
    NextTier(currentTier string) string
    MaxRetriesPerModel() int
    MaxRetriesPerBead() int
    ClassifyTimeout(parentCancelled bool, stallFired bool) string
}
```

**Replaces:**
- `escalation.SelectTier` / `config.SelectTier` in `process.go:38`.
- `escalation.SelectModel` / `config.SelectModel` in `process.go:39`.
- `config.NextEscalationTier` in `escalation/handler.go:155`.
- Hardcoded retry fields in `config.EscalationConfig`.
- Timeout classification branches in `callbacks.go:105-123` (`handleInvokeError`).

**Default implementation:** `ConfigEscalationPolicy` delegates to existing config methods. `ClassifyTimeout` encapsulates the three-way branch (stall / bead / invocation) that caused the two reverts.

### ValidationPolicy

Decides gate type, periodic full validation frequency, recovery attempt bounds, and mandatory command coverage.

```go
type ValidationPolicy interface {
    SelectGate(consecutiveSuccesses int) GateType
    MaxRecoveryAttempts() int
    ShouldEscalateRecovery() bool
    MandatoryCommandPrefixes() []string
}
```

**Replaces:**
- Fast vs Full gate selection logic spread across `process.go` and `lifecycle.go`.
- Hardcoded `bc.MaxRetries = 0` and single-escalation-attempt pattern in `makeValidationExecuteFn` (`callbacks.go:162-206`).
- `mandatoryQualityGateCommandPrefixes` variable in `lifecycle.go:22`.
- `cfg.Validation.FullValidationEveryN` reads scattered across the runner.

**Default implementation:** `ConfigValidationPolicy`. `SelectGate` returns Full when `consecutiveSuccesses % fullEveryN == 0`, otherwise Fast. `MaxRecoveryAttempts` returns 2 (matching current behavior: 1 quick + 1 escalated). `MandatoryCommandPrefixes` returns `["go test", "go vet", "go build"]`.

### MethodologyPolicy

Decides methodology activation, phase timeouts, deadline guards, and post-success deferral.

```go
type MethodologyPolicy interface {
    IsActive(labels []string, methodology string) bool
    PhaseTimeout(phase string, beadTimeoutSec int) int
    MinRefactorBudget() time.Duration
    MinRevalidationBudget() time.Duration
    ShouldDeferPostSuccess(atddActive, tddActive bool) bool
}
```

**Replaces:**
- `bead.IsMethodologyActive(...)` calls in `process_methodology.go:13,25`.
- `cfg.Methodology.ResolvePhaseTimeoutSeconds(...)` calls scattered across `process_methodology.go:46,97,213,248,271`.
- `minRefactorTime` and `minRevalidationTime` constants in `process_methodology.go:178,183`.
- Inline `!atddActive && !tddActive` deferral check in `process_methodology.go:93`.

**Default implementation:** `ConfigMethodologyPolicy` wraps existing config methods and constants.

## Runner Integration

The Runner struct gains four policy fields:

```go
type Runner struct {
    // ... existing fields ...
    stuckPolicy       StuckPolicy
    escalationPolicy  EscalationPolicy
    validationPolicy  ValidationPolicy
    methodologyPolicy MethodologyPolicy
}
```

Default implementations are constructed from `Config` at runner creation, following the same pattern as `escalationHandler` and `validationRunner`. The constructor signature does not change for existing callers; policies default to config-backed implementations when not explicitly provided.

### Call Site Migration

| Before | After |
|--------|-------|
| `r.cfg.Loop.StuckBeadThreshold` check in `isStuckBeadWithStats` | `r.stuckPolicy.IsStuck(b.ID, stats)` |
| `escalation.SelectTier(bc, r.cfg)` | `r.escalationPolicy.SelectInitialTier(priority, labels)` |
| `r.cfg.NextEscalationTier(savedTier)` | `r.escalationPolicy.NextTier(savedTier)` |
| Timeout branches in `handleInvokeError` | `r.escalationPolicy.ClassifyTimeout(parentCancelled, stallFired)` |
| `bc.MaxRetries = 0` in `makeValidationExecuteFn` | `r.validationPolicy.MaxRecoveryAttempts()` |
| `mandatoryQualityGateCommandPrefixes` var | `r.validationPolicy.MandatoryCommandPrefixes()` |
| `bead.IsMethodologyActive(...)` | `r.methodologyPolicy.IsActive(labels, "atdd")` |
| `minRefactorTime` constant | `r.methodologyPolicy.MinRefactorBudget()` |

## What Does Not Change

- **Config format.** No YAML changes. Policies are constructed from existing config fields.
- **Runner loop structure.** The loop in `lifecycle.go` keeps its current shape. It calls policy interfaces instead of config.
- **Existing sub-packages.** `escalation/`, `validation/`, `methodology/` retain their structure. Policies sit above them.
- **Provider routing.** Already abstracted behind `provider.Router`. No new policy interface needed.
- **makeMethodologyExec().** The 230-line function in `callbacks.go` is mostly mechanism (stream invocation, heartbeat, provider fallback). It does not shrink much. The methodology policy extraction happens in `process_methodology.go`.

## Testing

Each policy gets isolated unit tests without runner setup:

```go
func TestThresholdStuckPolicy(t *testing.T) {
    p := NewThresholdStuckPolicy(3)
    assert.False(t, p.IsStuck("bead-1", logger.BeadStats{Failures: 2}))
    assert.True(t, p.IsStuck("bead-1", logger.BeadStats{Failures: 3}))
}

func TestClassifyTimeout_StallFired(t *testing.T) {
    p := NewConfigEscalationPolicy(cfg)
    assert.Equal(t, "stall", p.ClassifyTimeout(false, true))
}

func TestValidationGateSelection_PeriodicFull(t *testing.T) {
    p := NewConfigValidationPolicy(cfg)
    assert.Equal(t, GateFast, p.SelectGate(1))
    assert.Equal(t, GateFull, p.SelectGate(3)) // fullEveryN=3
}
```

Runner integration tests verify policies are called at the right times. Policy logic is tested independently.

## Scope Boundary

This spec covers extracting policy interfaces and their default config-backed implementations. It does not cover:

- Alternate policy implementations (e.g., adaptive stuck detection, ML-based escalation).
- Config schema changes to support pluggable policies.
- Refactoring `callbacks.go` beyond the policy extraction points described above.
