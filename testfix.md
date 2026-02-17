# Test Suite Cost Reduction Plan

## Goal
Make the default `unit` lane fast and deterministic by moving integration-heavy tests into `acceptance`/`e2e_live`.

## Why It Is Slow
- Default `go test ./...` currently includes many integration-like tests.
- Major cost centers observed:
  - `internal/runner` (~79s)
  - `cmd/gromit` (~27s)
  - `internal/provider` (~23s)
  - `internal/pipeline` (~13s)
- Several tests shell out to subprocesses or do broad orchestration checks.

## Plan
1. Inventory expensive tests in default lane
- Use `go test -json ./...` and rank by package/test duration.
- Label each slow test as one of:
  - pure unit (keep in default)
  - deterministic acceptance (move to `//go:build acceptance`)
  - live external integration (move to `//go:build e2e_live`)

2. Tighten default lane policy
- Default lane must avoid:
  - real external CLIs/services
  - network/auth dependence
  - heavy filesystem/process orchestration unless strictly necessary
- Keep only deterministic tests with local in-memory or fixture-based dependencies.

3. Move integration-heavy tests out of default lane
- Retag candidates in `internal/runner`, `cmd/gromit`, `internal/provider`, and `internal/pipeline`.
- Preserve behavior coverage by keeping moved tests in `acceptance` or `e2e_live`.

4. Narrow `e2e_live` execution scope
- Update `make test-e2e-live` and CI to run only packages containing `e2e_live` tests (not full `./...`).

5. Enforce with guardrails
- Add/extend verification tests to fail if:
  - live-external tests are in default lane
  - `_acceptance_test.go` files are missing accepted tags (`acceptance` or `e2e_live`)

6. Rollout strategy
- Phase A: Retag highest-cost offenders first.
- Phase B: Re-run timings and compare before/after.
- Phase C: Continue until default lane meets target runtime budget.

## Success Criteria
- `make test-unit` runtime significantly reduced (target: under 60s on CI baseline runner).
- `make test-acceptance` remains deterministic and green.
- `make test-e2e-live` is focused and only run when required (nightly, release, or integration-impacting changes).
