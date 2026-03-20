# Spec 0005b — Adversarial Second-Pass Review

## spec_id
adversarial-second-pass-review

## Depends on
spec-0005a (pipeline stage ordering established)

## Vision
The normal review asks whether the implementation aligns with the spec. But many serious misses are not alignment failures — they are failures of pressure. A feature can satisfy the written happy path, pass deterministic checks, and still be wrong in ways a skeptical reviewer notices immediately. An adversarial second pass with fresh context and an explicitly contrary posture turns that skepticism into a pipeline stage, catching the issues that currently only surface during manual post-run review.

## Summary
A new configurable pipeline stage, AdversarialReview, runs after the normal Review stage only when no blocking findings remain. It operates with full context isolation — seeing only the spec, diff, test results, and acceptance criteria — and runs three adversarial facets (red_team, skeptical_reviewer, user_advocate) to probe for issues the normal review missed. Blocking findings trigger replan; non-blocking findings are recorded in evidence.

## Goals
### Primary
- Catch issues that pass normal review but fail manual human review
- Provide independent second-opinion coverage through context isolation
- Apply three distinct adversarial lenses for broad fault-finding coverage

### Secondary
- Make adversarial pressure configurable per-project and per-facet
- Keep cost bounded by only running when normal review has already passed

## Non-goals
- Replacing or modifying the existing 5-facet review stage (this supplements it)
- Risk-based gating of when the adversarial pass runs (deferred to 0005c)
- Feeding adversarial findings back into the normal review's facet weights (out of scope)

## Architecture

### Pipeline Position

Simplified view showing only the stages relevant to 0005b's placement. The full pipeline includes earlier stages (Plan, Build, Validate) established in prior specs.

```
Review → [blockers?] → replan (skip adversarial)
Review → [no blockers] → AdversarialReview → [blockers?] → replan
Review → [no blockers] → AdversarialReview → [no blockers] → Accept
```

### New Stage: AdversarialReview

Lives in `internal/next/specloop/stages/adversarial_review.go`. Implements the existing `Stage` interface. `Name()` returns `"adversarial_review"`.

### Three Adversarial Facets

```go
var AdversarialFacets = []FacetDef{
    {
        Name:           "red_team",
        Description:    "Assume bugs exist and find reasons the implementation should not ship",
        DefaultTier:    "high",
        PromptTemplate: "Your job is to find reasons this implementation should NOT ship. Assume bugs exist and find them." + basePromptBody,
    },
    {
        Name:           "skeptical_reviewer",
        Description:    "Find where the implementation is brittle, under-specified, or accidentally passing",
        DefaultTier:    "high",
        PromptTemplate: "Assume the normal review missed something. Where is this implementation brittle, under-specified, or accidentally passing?" + basePromptBody,
    },
    {
        Name:           "user_advocate",
        Description:    "Find where the implementation will break, confuse, or silently misbehave in production",
        DefaultTier:    "high",
        PromptTemplate: "You are the end user. Where will this break, confuse, or silently do the wrong thing in production?" + basePromptBody,
    },
}
```

### Context Isolation
- The stage receives only: spec text, git diff, validation results (test output, vet output), acceptance criteria
- It does NOT receive: normal review findings, review dispositions, prior cycle context
- Each facet runs as an independent LLM invocation via the existing `ReviewAgent` interface
- To enforce isolation, the adversarial stage calls `ReviewAgent.ReviewFacet` directly with adversarial-specific prompt rendering that excludes prior findings and dispositions. The prompt template's `{{if .PriorFindings}}` section will render empty because the adversarial stage supplies an empty `PriorFindings` slice. This ensures the LLM receives no signal about what the normal review found or accepted.

### Findings
- Adversarial findings use the same `Finding` struct as normal review, with facet set to the adversarial facet name
- Same severity levels: error, warning, suggestion, info
- Shares the same blocking threshold config as normal review (i.e., `error` severity blocks by default). No separate `blocking_threshold` config key — adversarial findings are evaluated with the same severity-to-blocking logic as normal review findings
- Findings are recorded in evidence as `adversarial-review.json`, separate from `review.json`

### Configuration

```yaml
adversarial_review:
  enabled: true                              # default: true
  facets: [red_team, skeptical_reviewer, user_advocate]  # default: all three
  model_tier: high                           # default: high
```

If `facets: []` is configured (empty list), it is treated as equivalent to `enabled: false` — the stage is skipped entirely. This avoids a config validation error while making the intent unambiguous.

### Key Decisions
- Separate evidence file (`adversarial-review.json`) keeps adversarial findings distinct from normal review findings for traceability
- Reuses `ReviewAgent` interface and `Finding` struct — no new abstractions for running facets
- The stage is skipped entirely (not just returning empty) when disabled, same pattern as 0005a

## Acceptance Criteria

1. When `adversarial_review.enabled` is true (default), the AdversarialReview stage runs after Review only when the normal review produced no blocking findings
2. When `adversarial_review.enabled` is false, the stage is skipped entirely
3. When the normal review produces blocking findings, the AdversarialReview stage does not run and the pipeline replans as usual
4. The adversarial stage receives only spec text, git diff, validation results, and acceptance criteria — not normal review findings or dispositions
5. Each configured adversarial facet runs as an independent LLM invocation
6. The facet list is configurable, defaulting to all three: `red_team`, `skeptical_reviewer`, `user_advocate`
7. The model tier is configurable, defaulting to the project's high tier
8. Adversarial findings use the same `Finding` struct and severity levels as normal review findings
9. Blocking adversarial findings trigger replan with failure context
10. Adversarial findings are recorded in `adversarial-review.json` in the evidence directory, separate from `review.json`
11. All existing tests continue to pass

## Scenarios

### Scenario: Adversarial review catches issue normal review missed
**Given:** A spec with 3 acceptance criteria, normal review completed with no blocking findings
**When:** The AdversarialReview stage runs with all three facets enabled
**Then:** The `skeptical_reviewer` facet produces 1 error-severity finding about an unhandled nil case. The stage returns `ReplanFrom` with the finding as context. The finding is recorded in `adversarial-review.json`.

### Scenario: Adversarial review passes — pipeline continues to Accept
**Given:** Normal review completed with no blocking findings
**When:** The AdversarialReview stage runs and all three facets produce only suggestion-severity findings
**Then:** The stage returns `Continue`, findings are recorded in `adversarial-review.json`, and the pipeline proceeds to Accept

### Scenario: Skipped when normal review has blockers
**Given:** Normal review produced 2 error-severity findings
**When:** The pipeline evaluates whether to run AdversarialReview
**Then:** The AdversarialReview stage is skipped, no LLM invocations occur, and the pipeline replans from the normal review's findings

### Scenario: Stage disabled via config
**Given:** A project with `adversarial_review.enabled: false`
**When:** Normal review completes with no blocking findings
**Then:** The AdversarialReview stage is skipped, no LLM invocations occur, and the pipeline proceeds directly to Accept

### Scenario: Adversarial facet fails due to LLM error
**Given:** Normal review completed with no blocking findings, all three adversarial facets are enabled
**When:** The `red_team` facet invocation returns an LLM error (timeout, parse failure, etc.)
**Then:** The stage logs the error, records partial results from the successful facets in `adversarial-review.json`, and returns `Continue` (graceful degradation). A failed facet does not block the pipeline — it is treated as if it produced no findings. The error is recorded in evidence for observability.

### Scenario: Subset of facets configured
**Given:** A project with `adversarial_review.facets: [red_team, user_advocate]`
**When:** The AdversarialReview stage runs
**Then:** Only `red_team` and `user_advocate` facets are invoked. No `skeptical_reviewer` invocation occurs. Findings from both facets are recorded in `adversarial-review.json`.

### Scenario: Empty facet list treated as disabled
**Given:** A project with `adversarial_review.facets: []`
**When:** Normal review completes with no blocking findings
**Then:** The AdversarialReview stage is skipped (equivalent to `enabled: false`), no LLM invocations occur, and the pipeline proceeds directly to Accept

### Scenario: Model tier configuration respected
**Given:** A project with `adversarial_review.model_tier: medium`
**When:** The AdversarialReview stage runs
**Then:** All adversarial facet LLM invocations use the project's medium-tier model instead of the default high tier

## Validation

```
go test ./internal/next/specloop/stages/...
go test ./internal/next/review/...
go test ./internal/next/...
go vet ./...
```
