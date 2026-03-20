# Spec 0005c — Risk-Weighted Review Rigor

## spec_id
risk-weighted-review-rigor

## Depends on
spec-0005a, spec-0005b (controls knobs introduced by both)

## Vision
Not every run deserves the same scrutiny. Applying the heaviest gates to every change raises cost without improving outcomes, but applying uniform moderate scrutiny leaves the system blind to risk spikes. Risk-weighted rigor lets the pipeline spend quality budget where it matters most — cheap runs stay cheap, while larger diffs, sensitive areas, degraded evidence, and repeated failures trigger proportionally stronger proof demands.

## Summary
A RiskAssessor computes a discrete risk level (low, medium, high) from five signal categories and uses it to modulate pipeline behavior. Risk is assessed twice per run — preliminarily from the plan and updated from the actual diff. The risk level controls adversarial review scope, counterexample caps, and blocking thresholds. The computed level can be overridden in project or spec config.

## Goals
### Primary
- Make pipeline cost proportional to change risk
- Automatically escalate scrutiny for large, sensitive, or degraded-evidence runs
- Provide a single, legible risk level that controls multiple pipeline knobs

### Secondary
- Allow projects to override risk level when they know their risk profile better than the signals do
- Keep the risk computation cheap and deterministic (no LLM calls)

## Non-goals
- Per-file or per-function risk scoring (too granular for this spec)
- Dynamic risk adjustment mid-execution based on builder behavior (out of scope)
- Custom risk signals beyond the five defined categories (extensibility deferred)

## Architecture

### Risk Levels and Knob Mapping

| Knob | Low | Medium | High |
|------|-----|--------|------|
| Adversarial review | skip | 1 facet (skeptical_reviewer) | all 3 facets |
| Counterexample cap | 1 per scenario | 3 per scenario | 5 per scenario |
| Blocking threshold | error only | error only | warning + error (threshold lowered to `SeverityWarning`, per `IsBlocking(threshold, severity)` semantics) |

**Note:** Risk modulates the default adversarial scope, but explicit `adversarial_review.enabled` and `adversarial_review.facets` config from 0005b takes precedence. If a project explicitly configures adversarial review, that config overrides the risk-driven default.

### RiskAssessor

Lives in `internal/next/risk/assessor.go`. Pure function, no LLM calls.

```go
type RiskLevel string

const (
    RiskLow    RiskLevel = "low"
    RiskMedium RiskLevel = "medium"
    RiskHigh   RiskLevel = "high"
)

type RiskSignals struct {
    DiffLines        int
    FilesTouched     int
    PackagesTouched  int
    PublicAPIChanges bool
    SensitiveAreas   []string
    DegradedFlags    []string
    TrustLevel       string
    ReplanCount      int
    RepeatedFailure  bool
}

func Assess(signals RiskSignals) RiskLevel
```

### Scoring Rules (deterministic, no weights)

#### Signal Category to Struct Field Mapping

| Signal Category | RiskSignals Fields |
|---|---|
| Diff size | `DiffLines` |
| Surface area | `FilesTouched`, `PackagesTouched`, `PublicAPIChanges` |
| Sensitive areas | `SensitiveAreas` |
| Evidence quality | `DegradedFlags`, `TrustLevel` |
| Cycle history | `ReplanCount`, `RepeatedFailure` |

High if ANY of:
- `DiffLines > 500` or `FilesTouched > 20`
- `PackagesTouched > 8` or `PublicAPIChanges == true`
- Any sensitive area touched
- `TrustLevel == "low"`
- `len(DegradedFlags) > 0`
- `RepeatedFailure == true`

Low if ALL of:
- `DiffLines < 50` and `FilesTouched < 5`
- No sensitive areas
- `TrustLevel == "high"`
- `ReplanCount == 0`

Medium otherwise (including `TrustLevel == "medium"`, which is neither low-triggering nor high-triggering and falls to this catch-all).

### Two Assessment Points

1. **After Plan, before WriteContracts** — preliminary assessment from plan metadata. Plan metadata maps to RiskSignals as follows: task count is used as an estimate for DiffLines (e.g., tasks * 50), targeted packages map to FilesTouched and PackagesTouched estimates, and package names are matched against the sensitive areas list. Sets counterexample cap for 0005a.
2. **After Execute, before Review** — updated assessment from actual diff and validation results. Sets adversarial scope and blocking threshold for 0005b.

### Configuration

```yaml
risk:
  level: auto          # "auto" (default), "low", "medium", or "high"
  floor: null          # optional minimum: "medium" or "high"
  thresholds:
    large_diff: 500
    many_files: 20
    small_diff: 50
    few_files: 5
```

When `level` is set to a specific value, `Assess()` is skipped and that level is used directly. When `floor` is set, the computed level is raised to the floor if lower.

### Integration Points
- SynthesizeCounterexamples reads counterexample cap from the preliminary risk level
- AdversarialReview reads adversarial scope from the updated risk level
- Review stage reads blocking threshold from the updated risk level
- Evidence bundler records the risk level and signals in `risk-assessment.json`

## Acceptance Criteria

1. The RiskAssessor computes a risk level from five signal categories: diff size, surface area, sensitive areas, evidence quality, and cycle history
2. Risk computation is deterministic with no LLM calls
3. Risk is assessed preliminarily after Plan (from plan metadata) and updated after Execute (from actual diff and validation results)
4. The preliminary risk level sets the counterexample cap for the SynthesizeCounterexamples stage
5. The updated risk level sets adversarial review scope and blocking threshold for Review and AdversarialReview stages
6. When `risk.level` is set to a specific value in config, the computed assessment is bypassed and that level is used directly
7. When `risk.floor` is set, the computed level is raised to the floor if lower but never lowered
8. Risk level and input signals are recorded in `risk-assessment.json` in the evidence directory
9. Scoring thresholds (large_diff, many_files, small_diff, few_files) are configurable with sensible defaults
10. All existing tests continue to pass

## Scenarios

### Scenario: High-risk run triggers full adversarial review
**Given:** A run that touches 25 files across 9 packages with 600 lines changed
**When:** The RiskAssessor runs after Execute
**Then:** Risk level is computed as `high`. AdversarialReview runs all 3 facets. Counterexample cap is 5 per scenario. Blocking threshold is warning + error. `risk-assessment.json` records the level and all input signals.

### Scenario: Low-risk run skips adversarial review
**Given:** A run that touches 3 files in 1 package with 30 lines changed, no sensitive areas, trust level high, no replans
**When:** The RiskAssessor runs after Execute
**Then:** Risk level is computed as `low`. AdversarialReview is skipped. Counterexample cap is 1 per scenario. Blocking threshold is error only.

### Scenario: Low trust level forces high risk
**Given:** A run with 3 files, 40 lines changed, but TrustLevel is "low"
**When:** The RiskAssessor runs after Execute
**Then:** Risk level is `high` because TrustLevel is "low", despite the small diff

### Scenario: Sensitive area forces high risk regardless of diff size
**Given:** A run that touches 2 files with 20 lines changed, but one file is in an auth package
**When:** The RiskAssessor runs after Execute
**Then:** Risk level is `high` because a sensitive area was touched, despite the small diff.

### Scenario: Config override bypasses computation
**Given:** A project with `risk.level: low` in config
**When:** The run touches 30 files with 800 lines changed
**Then:** Risk level is `low` regardless of signals. Adversarial review is skipped. `Assess()` is not called. `risk-assessment.json` records the override level and notes that computation was bypassed, but does not record a hypothetical computed level.

### Scenario: Floor raises computed level
**Given:** A project with `risk.floor: medium` in config
**When:** The run touches 2 files with 15 lines changed (would compute as `low`)
**Then:** Risk level is raised to `medium`. AdversarialReview runs with 1 facet. Counterexample cap is 3.

### Scenario: Degraded evidence flags force high risk
**Given:** A run that touches 4 files with 40 lines changed and no sensitive areas, but DegradedFlags contains `["test_coverage_incomplete"]`
**When:** The RiskAssessor runs after Execute
**Then:** Risk level is `high` because degraded evidence flags are present, despite the small diff. AdversarialReview runs all 3 facets. Blocking threshold is warning + error.

### Scenario: Replan prevents low risk classification
**Given:** A run that touches 2 files with 20 lines changed, no sensitive areas, trust level high, but ReplanCount is 1
**When:** The RiskAssessor runs after Execute
**Then:** Risk level is NOT `low` because `ReplanCount > 0` violates the low-risk requirement. Risk falls to `medium` (the catch-all).

### Scenario: Preliminary vs updated risk diverge
**Given:** A plan targeting 2 tasks in 1 package (preliminary risk: `low`, counterexample cap set to 1), but execution produces a 600-line diff touching auth code
**When:** The RiskAssessor runs after Execute
**Then:** Updated risk is `high`. AdversarialReview runs all 3 facets with warning + error blocking. The counterexample cap from the preliminary assessment (1) is not retroactively changed.

## Validation

```
go test ./internal/next/risk/...
go test ./internal/next/specloop/stages/...
go test ./internal/next/...
go vet ./...
```
