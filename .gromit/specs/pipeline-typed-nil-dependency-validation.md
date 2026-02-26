---
id: pipeline-typed-nil-dependency-validation
source_ideas: []
created: 2026-02-19
epic: codebase-health
---

# Pipeline Typed Nil Dependency Validation

## Specification

The pipeline dependency validators for review and explore workflows must detect missing dependencies using explicit typed checks instead of interface-wrapped map iteration.

This change applies to:
- `validateReviewDeps` in `internal/pipeline/pipeline.go`
- `validateExploreDeps` in `internal/pipeline/explore.go`

The validators must preserve current behavior and return the same dependency-specific errors when a required dependency is absent. The validation logic should avoid any pattern that can miss typed nil values hidden inside interface values.

The validation order and coverage should remain straightforward and deterministic: each required dependency is checked directly against nil in code, and the first missing dependency returns the existing error string.

## Acceptance Criteria

- `validateReviewDeps` uses direct typed nil checks for each required dependency and no longer uses `map[string]interface{}` plus loop-based `dep == nil` checks.
- `validateExploreDeps` uses direct typed nil checks for each required dependency and no longer uses `map[string]interface{}` plus loop-based `dep == nil` checks.
- Neither validator uses reflection for nil detection.
- Existing error message text remains unchanged (for example, `pipeline: nil BacklogClient`) for all missing dependency cases.
- A typed nil dependency value (for a nil-able concrete implementation assigned to an interface field) is treated as missing and returns the expected dependency-specific error instead of passing validation.

## Decisions

1. **Use explicit typed checks instead of generic nil helpers**
Direct nil checks on each dependency field were chosen over map+interface or generic reflection helpers to avoid interface nil pitfalls and keep behavior obvious in code review.

2. **Apply the same strategy to both review and explore validators**
Both validators currently share the same interface-map nil-check pattern, so both are included in scope to prevent partial fixes and repeated bug classes.

3. **Preserve existing error strings exactly**
Error text remains unchanged to avoid breaking tests, logs, or caller expectations tied to current dependency-specific messages.

4. **Disallow reflection in validator logic**
Reflection-based nil detection is intentionally excluded to reduce complexity and keep dependency validation static, explicit, and easier to maintain.

## Research & Context

### Current State

- `internal/pipeline/pipeline.go` `validateReviewDeps` builds `requiredDeps := map[string]interface{}{...}` and checks each `dep == nil`.
- `internal/pipeline/explore.go` `validateExploreDeps` uses the same map+interface nil-check pattern.
- This pattern can miss typed nil values stored in interface slots, allowing invalid dependencies through validation and causing later runtime failures when methods are called.

### Why This Fits Existing Architecture

- `Deps` in `internal/pipeline/pipeline.go` is a fixed struct with strongly typed interface fields (`ClaudeClient`, `PromptRenderer`, `BacklogClient`, etc.).
- Because required dependencies are known at compile time and small in number, explicit checks align with the codebase’s typed pipeline direction and avoid introducing dynamic validation machinery.
