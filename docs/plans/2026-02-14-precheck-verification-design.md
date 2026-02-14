# Precheck Two-Model Verification

## Problem

Haiku precheck frequently auto-closes beads whose acceptance criteria are not actually met. It sees partial implementations (infrastructure in place, wiring missing) and concludes the work is done. False positives cascade when downstream beads build on incomplete work, costing more time than precheck saves.

## Solution

Add a second verification phase to precheck. Haiku screens cheaply; sonnet verifies independently. Both must agree PRECHECK_PASSED for auto-close. Misses (the common case at ~79%) cost nothing extra.

## Core Mechanism

`runPrecheck()` becomes two-phase:

1. **Phase 1 (screen):** Haiku runs the existing precheck prompt. If PRECHECK_NOT_MET or error, stop. No additional cost.
2. **Phase 2 (verify):** Only if phase 1 passed. Sonnet runs the same prompt independently — no mention of haiku's verdict. If sonnet also returns PRECHECK_PASSED, auto-close. Otherwise, proceed to normal build.

Both invocations use the same PROMPT_precheck.md template. Independence prevents anchoring bias.

## Configuration

```yaml
precheck:
  enabled: true
  timeout_seconds: 120
  verification:
    enabled: true
    model: sonnet
    timeout_seconds: 120
```

- `precheck.enabled: false` kills both phases
- `precheck.verification.enabled: false` restores single-model behavior
- `precheck.verification.model` is configurable (sonnet default, opus for high-stakes)

Phase 1 model stays haiku via existing `router.Select("precheck", TierLow)`.

## Runner Integration

Change is contained to `runPrecheck()`. The main loop is untouched — it still sees `passed, duration := r.runPrecheck(ctx, b)`.

Inside runPrecheck:
- Phase 1: `router.Select("precheck", TierLow)` — haiku
- Phase 2: `router.Select("precheck", TierMedium)` — sonnet
- Duration returned is sum of both phases
- Phase 2 errors treated as NOT_MET (conservative)
- Log line on phase 2 rejection: "Pre-check passed by haiku but rejected by sonnet verification"

Consecutive skip counter, auto-close logic, and safety mechanisms unchanged.

## Testing

Three new mock-based tests in interfaces_test.go:

1. **PrecheckVerificationRejects** — haiku PASSED, sonnet NOT_MET → normal build
2. **PrecheckVerificationConfirms** — both PASSED → auto-close
3. **PrecheckVerificationError** — haiku PASSED, sonnet errors → normal build

Existing PrecheckPassed test updated so mock returns PASSED from both tiers.

## Files Touched

- `internal/config/config.go` — VerificationConfig struct, defaults, nil normalization
- `internal/runner/runner.go` — runPrecheck() phase 2 logic
- `gromit.yaml` — verification section under precheck
- `internal/runner/interfaces_test.go` — 3 new tests + mock adjustment
