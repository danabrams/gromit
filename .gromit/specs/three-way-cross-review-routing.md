---
id: three-way-cross-review-routing
source_ideas: []
created: 2026-02-26
---

# Three-Way Cross-Review Routing

## Specification

When review phase routing is configured as cross-review (`routing.phase_preferences.review: cross`), Gromit must run review using any available provider that is different from the provider that built the bead.

This applies to two-provider and three-provider setups, including Gemini-enabled routing.

Behavior requirements:

- If one or more non-build providers are available, select one of those providers for review.
- If multiple non-build providers are available, any non-build choice is acceptable.
- If no non-build providers are available, fall back to the build provider so review still runs rather than failing or skipping solely due to routing.
- If no providers are available at all, return no provider (existing terminal failure behavior).

This behavior is only for cross-review routing. Non-cross review routing (`review: claude`, `review: openai`, `review: any`, etc.) is unchanged.

## Acceptance Criteria

- With `review: cross` and providers `claude`, `openai`, `gemini`, a bead built by `gemini` is reviewed by either `claude` or `openai`, never `gemini`, when either alternative is available.
- With `review: cross` and providers `claude`, `openai`, `gemini`, a bead built by `claude` is reviewed by either `openai` or `gemini`, never `claude`, when either alternative is available.
- With `review: cross`, when all non-build providers are unavailable and the build provider is available, review falls back to the build provider.
- If only one provider exists in configuration and `review: cross` is set, review uses that provider (fallback behavior).
- Existing non-cross routing behavior and provider selection paths remain unchanged.
- Automated tests cover three-provider cross-review selection and fallback behavior with unavailable alternatives.

## Decisions

1. **Cross-review remains “different provider”, not fixed pair mapping** Cross-review should not encode hardcoded pairs like “Gemini reviews Claude only.” The requirement is provider diversity from the build provider, which scales naturally from two to three providers.

2. **Fallback prioritizes continuity over strict separation** If no alternate provider is available, review should still execute with the build provider to avoid blocking the loop on temporary provider availability constraints.

3. **Selection among multiple alternatives may remain non-deterministic** When two or more non-build providers are available, any non-build provider is valid. Deterministic balancing among alternatives is out of scope for this refinement.

## Research & Context

### Current State

- [`internal/runner/reviewpkg/reviewer.go`](/home/dabrams/gromit/.-gromit-refine-1772068559052345361/internal/runner/reviewpkg/reviewer.go) already routes through `SelectCross(buildProvider, tier)` when `routing.phase_preferences.review == "cross"` and a build provider is known.
- [`internal/provider/router.go`](/home/dabrams/gromit/.-gromit-refine-1772068559052345361/internal/provider/router.go) implements `SelectCross` by selecting the first available provider whose name differs from `buildProvider`, then falling back to `buildProvider`.
- [`internal/provider/cross_review_test.go`](/home/dabrams/gromit/.-gromit-refine-1772068559052345361/internal/provider/cross_review_test.go) already verifies:
- Two-provider opposite selection behavior.
- Fallback to build provider when alternatives are unavailable.
- Three-provider behavior that only enforces “not the build provider.”
- [`gromit.yaml`](/home/dabrams/gromit/.-gromit-refine-1772068559052345361/gromit.yaml) currently has `review: claude` in sample config; cross-review behavior is activated when this is set to `cross`.

### Fit with Existing Patterns

- This refinement aligns with existing router abstractions and review orchestration interfaces without introducing new provider-specific coupling.
- Gemini support is already part of the provider ecosystem in specs/tests; this feature clarifies expected cross-review semantics as provider count grows.

