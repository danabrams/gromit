---
created: 2026-02-16T00:00:00Z
decomposed: true
decomposed_at: "2026-02-16T01:08:23Z"
id: acceptance-runtime-optimization
source_spec: acceptance-test-cleanup
---

# Acceptance Runtime Optimization Plan

**Goal:** Reduce acceptance-test runtime and iteration drag by shrinking heavy acceptance suites, moving non-E2E checks to unit tests, and tightening slow patterns.

**Context:** Recent runs show acceptance verification repeatedly spending significant time in `internal/runner/...` and retry loops. The largest acceptance files are concentrated in provider/runner/cmd packages and include many internal-behavior checks.

## Tasks

### Task 1: Reclassify non-E2E acceptance tests in heavy runner/provider files
**Files:**
- `internal/runner/validation_extraction_acceptance_test.go`
- `internal/runner/build_router_from_config_acceptance_test.go`
- `internal/runner/build_router_from_config_additional_acceptance_test.go`
- `internal/provider/codex_streaming_acceptance_test.go`

**Work:**
- Move tests that validate internal behavior, wiring, or TODO expectations into `*_test.go` unit suites.
- Keep only true command/API-surface acceptance coverage in `*_acceptance_test.go`.
- Preserve assertions while reducing acceptance-tag execution footprint.

**Acceptance Criteria:**
- Acceptance files retain only true acceptance behavior.
- Reclassified tests continue to run under normal `go test ./...`.
- `go test -tags acceptance ./internal/runner/... ./internal/provider/...` passes.

### Task 2: Table-drive repetitive acceptance scenarios
**Files:**
- `internal/runner/build_router_from_config_acceptance_test.go`
- `cmd/gromit/review_spec_validation_acceptance_test.go`
- `internal/runner/scope_check_double_invocation_acceptance_test.go`

**Work:**
- Consolidate repetitive setup/assert patterns into shared setup helpers and table-driven tests.
- Remove duplicated fixtures where test structure only differs by input/expected outcome.

**Acceptance Criteria:**
- Repetitive scenario groups are table-driven.
- Total line count in target files decreases while preserving scenario coverage.
- Tests remain readable and deterministic.

### Task 3: Remove avoidable runtime delays and brittle timing checks
**Files:**
- `internal/retro/filtered_hash_eviction_acceptance_test.go`
- `internal/runner/invocation_timeout_acceptance_test.go`

**Work:**
- Remove explicit sleeps where not behaviorally required.
- Replace wide time-window assertions with deterministic signal-based checks where possible.

**Acceptance Criteria:**
- No avoidable `time.Sleep` remains in acceptance tests.
- Timeout tests remain meaningful but less wall-clock expensive/flaky.

### Task 4: Add/keep a slim acceptance smoke path for key E2E behavior
**Files:**
- `cmd/gromit/*_acceptance_test.go`
- `internal/runner/*_acceptance_test.go` (targeted smoke subset)

**Work:**
- Ensure acceptance suite focuses on high-value end-to-end checks.
- Move lower-level behavior validation out of acceptance-tagged paths.

**Acceptance Criteria:**
- Acceptance suite covers critical end-to-end behavior with smaller runtime surface.
- Supporting non-E2E checks run in unit suites.

### Task 5: Validate runtime and correctness after optimization
**Commands:**
- `go test -vet=off -p 4 -parallel 4 ./...`
- `go test -tags acceptance -vet=off -p 4 -parallel 4 ./internal/config/... ./internal/runner/... ./internal/provider/... ./cmd/gromit/...`
- `go vet ./...`
- `go build ./...`

**Acceptance Criteria:**
- All commands pass.
- Acceptance-focused command runtime is measurably reduced versus current baseline for touched packages.

## Dependencies

1. Task 1 before Task 2 (reclassification first, then consolidation).
2. Task 1 before Task 4 (smoke suite definition depends on reclassification outcome).
3. Task 2/3/4 before Task 5 (final validation and timing check).

## Testing Strategy

- Preserve behavior equivalence while shifting test type (acceptance -> unit).
- Use package-scoped acceptance runs during iteration to keep feedback tight.
- Run full project validation once optimization beads complete.
