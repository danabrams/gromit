---
id: provider-cli-test-pyramid
source_ideas: []
created: 2026-02-16
epic: provider-ecosystem
---

# Split Provider Acceptance Tests Into Mocked Fast Path and Real-CLI Smoke Path

## Specification

Gromit's acceptance coverage currently depends heavily on subprocess-style provider tests and contract tests around CLI invocation behavior. This gives strong confidence, but runtime cost grows quickly when tests depend on real external CLIs.

This spec defines a two-lane strategy:

1. Fast deterministic acceptance tests that mock Claude/Codex CLI behavior and run by default in PR workflows.
2. A thin real-CLI smoke suite that validates true integration and catches CLI drift, but runs on slower cadence (nightly and release/pre-merge gates).

This keeps local/PR feedback fast while preserving confidence that real CLI integrations still work.

### 1. Default lane: mocked acceptance and contracts

Default acceptance and provider tests should use deterministic fakes or mock binaries:

- Keep using `test/fakes/claude` and fixture-driven outputs for Claude integration behavior.
- Add a `test/fakes/codex` counterpart with matching capabilities:
  - invocation logging
  - fixture output
  - failure modes
  - optional stream-json style event fixtures
- Keep provider parsing and orchestration tests in-process with mock binaries in temp dirs (current codex provider pattern).
- Keep tests hermetic: no network, no user auth state, no dependency on globally installed CLIs.

The default lane should be the standard CI path for routine PRs and local development.

### 2. Thin real-CLI smoke lane

Add a minimal suite that exercises real Claude and Codex CLIs end-to-end for a small number of representative scenarios:

- one successful non-stream provider invocation
- one streaming invocation with event parsing path
- one failure-mode assertion (for example, known non-zero exit handling)

Characteristics:

- Build-tagged separately from fast tests (for example `//go:build smokecli`).
- Requires explicit env gates (for example `CLAUDE_SMOKE=1`, `CODEX_SMOKE=1`) and skips otherwise.
- Runs only when credentials and binaries are intentionally configured.
- Stays intentionally small (target: 3-6 tests total).

### 3. Contract fixtures to prevent mock drift

For each provider, maintain fixture snapshots sourced from real CLI runs:

- store representative outputs under `test/fixtures/` (success, failure, streamed event shapes)
- use those fixtures in mocked acceptance/provider tests
- refresh fixtures on a regular cadence or when CLI output formats change

Fixture refresh should be explicit and documented so diffs are reviewable.

### 4. Runtime budgets and execution policy

Define runtime expectations by lane:

- fast default lane (unit + acceptance/contract with fakes): optimized for developer loop and PR gating
- smoke lane (real CLIs): slower, allowed to be separate from default blocking checks

Track lane runtime with existing timing tooling (`scripts/test_timing.sh`) and add per-lane commands so regressions are visible.

## Acceptance Criteria

- A fake Codex CLI exists at `test/fakes/codex` with parity to the fake Claude ergonomics needed by tests (logging, fixture output, and failure toggles)
- Provider and acceptance tests that do not require real CLIs use fakes/mock binaries and run without Claude/Codex auth setup
- A new real-CLI smoke lane exists behind an explicit build tag and env gates; it is not part of default `go test ./...`
- Smoke lane includes at least one real Claude invocation test and one real Codex invocation test through provider entry points
- Representative real CLI outputs are captured as fixtures for both Claude and Codex and are consumed by mocked tests
- Test commands are documented in-repo for:
  - fast default lane
  - smoke real-CLI lane
  - fixture refresh workflow
- Runtime budgets are defined for default lane and smoke lane, and timing regressions are visible through existing tooling or companion scripts

## Decisions

1. **Mock by default, verify reality with a thin smoke lane.** Confidence comes from broad deterministic coverage plus a small number of real integration checks, not from running all tests against real CLIs.

2. **Keep real-CLI checks opt-in and environment-gated.** Real CLI tests are sensitive to auth, quotas, and external service variability; they should run intentionally, not accidentally.

3. **Codex fake should match Claude fake capabilities.** Symmetry reduces test-only branching and makes provider tests consistent across backends.

4. **Fixture snapshots are part of the contract.** Real output samples anchor mocks to reality and reduce silent drift.

## Research & Context

### Current State

- Contract suite already exists behind `//go:build contract` (`test/contracts/*`) and uses fake CLIs plus call-log assertions.
- E2E suite already exists behind `//go:build e2e` (`test/e2e/*`) and uses fake CLIs for deterministic behavior.
- Provider acceptance tests for Codex currently use temp mock binaries in `internal/provider/codex_streaming_acceptance_test.go`.
- Repo currently has `test/fakes/claude`, `test/fakes/bd`, and `test/fakes/git`, but no `test/fakes/codex`.
- `cmd/gromit/final_verification_test.go` enforces acceptance tag hygiene for `*_acceptance_test.go`.

### Relevant Files

- `test/fakes/claude`
- `test/contracts/claude_contract_test.go`
- `test/e2e/e2e_test.go`
- `internal/provider/codex_streaming_acceptance_test.go`
- `scripts/test_timing.sh`
- `cmd/gromit/final_verification_test.go`
