---
created: 2026-02-16T00:00:00Z
decomposed: true
decomposed_at: "2026-02-16T02:27:05Z"
id: provider-cli-test-pyramid
source_spec: provider-cli-test-pyramid
---

# Provider CLI Test Pyramid Implementation Plan

**Goal:** Split provider acceptance coverage into a fast fake-backed default lane and a thin real-CLI smoke lane so PR feedback stays fast without losing integration confidence.

**Architecture:** Keep routine tests hermetic and deterministic using fake Claude/Codex CLIs and real-output fixtures, then add a minimal `smokecli` build-tagged suite for true CLI integration checks behind explicit env gates.

**Tech Stack:** Go, shell fake binaries, Go test build tags (`acceptance`, `contract`, `e2e`, `smokecli`), existing timing script infrastructure.

**Spec:** `.gromit/specs/provider-cli-test-pyramid.md`

---

## Architecture

**Overview:**
Use a two-lane test pyramid by keeping default coverage fully fake-backed (fast, hermetic), and adding a minimal `smokecli` build-tag lane for real Claude/Codex CLI integration checks.

**Key Components:**
1. **Fake Codex CLI (`test/fakes/codex`)**: Add a first-class fake matching fake Claude ergonomics (call logging, fixture-driven output, fail toggles, optional JSONL stream fixture mode).
2. **Shared test harness wiring**: Extend contract/e2e env setup so fake Codex is automatically available via PATH and env controls like existing fakes.
3. **Default lane test migration**: Refactor Codex provider acceptance/contract tests to use `test/fakes/codex` + fixtures instead of inline temp scripts.
4. **Real-CLI smoke suite (`//go:build smokecli`)**: Add very small integration tests gated by `CLAUDE_SMOKE=1` and/or `CODEX_SMOKE=1`, skipping otherwise.
5. **Fixture contract set (`test/fixtures/`)**: Add representative Claude/Codex success/failure/stream event fixtures derived from real runs and consumed by fast tests.
6. **Lane commands + runtime budgets**: Add per-lane commands/docs and timing hooks for fast lane vs smoke lane.

**Integration Points:**
- Reuse existing fake executable pattern from `test/fakes/claude`.
- Reuse existing `TestMain` env setup pattern in `test/contracts/helpers_test.go` and `test/e2e/e2e_test.go`.
- Keep smoke tests out of default `go test ./...` via `smokecli` tag and explicit env checks.
- Keep existing acceptance-tag hygiene intact (`cmd/gromit/final_verification_test.go`), since smoke uses a separate tag.

**Data Flow:**
- Fast lane: test sets fixture env -> fake codex/claude reads stdin -> emits deterministic output/events -> provider parses -> tests assert behavior and call logs.
- Smoke lane: test invokes real provider entrypoint -> real CLI runs with real auth -> assert minimal success/stream/failure invariants -> skip unless env gate present.

**Files to Modify:**
- `test/contracts/helpers_test.go` - add Codex fixture env helpers and utility glue.
- `test/contracts/fakes_integration_test.go` - add fake Codex behavior parity checks.
- `internal/provider/codex_streaming_acceptance_test.go` - replace inline mock scripts with fake Codex + fixtures.
- `scripts/test_timing.sh` - support per-lane timing invocation/reporting.
- `cmd/gromit/final_verification_test.go` - only if explicit guardrails are needed for smoke naming/tag hygiene.

**Files to Create:**
- `test/fakes/codex` - fake Codex executable.
- `test/fixtures/codex_*.jsonl` and `test/fixtures/codex_*.txt` - success/failure/stream snapshots.
- `test/fixtures/claude_stream_*.jsonl` as needed for representative stream contract samples.
- `internal/provider/cli_smoke_test.go` (or split by provider) with `//go:build smokecli`.
- Testing documentation updates for lane commands and fixture refresh workflow.

**Tradeoffs:**
- **Canonical fake codex binary** over per-test inline scripts for consistency and reduced duplication.
- **Build tag + env gate for smoke** over env-only gating to avoid accidental default execution.
- **Real-output fixture snapshots** over synthetic-only fixtures to reduce drift between mocks and real CLI formats.

## Test Strategy

**Test Levels:**
1. **Unit/provider tests (default):** Validate parsing, orchestration, arg construction, retries, and error classification with fake binaries/fixtures.
2. **Contract/acceptance tests (default):** Validate CLI invocation contract and end-to-end provider paths against fake Claude/Codex.
3. **Smoke integration (`smokecli`):** Minimal real CLI checks for one non-stream success, one stream path, and one failure handling path.
4. **Manual/ops validation:** Fixture refresh commands produce reviewable snapshot diffs.

**Key Test Cases:**
- Fake Codex logs invocation command lines and respects fixture/failure env controls.
- Fake Codex supports plain output and JSONL stream event fixture output.
- Codex provider acceptance paths use fake Codex from PATH rather than inline mock scripts.
- Contract-level CLI tests can assert Codex invocation patterns similarly to Claude.
- Smoke Claude test: real invocation succeeds through provider entrypoint.
- Smoke Codex test: real non-stream and stream invocations succeed through provider entrypoint.
- Smoke failure test: known non-zero/failure-mode path is surfaced as expected.

**Mocking Strategy:**
- Default CI/local lane uses fake CLIs and fixture snapshots only (no auth, no network dependency).
- Real CLI execution is isolated to smoke lane and explicitly enabled via env flags.

**Coverage Goals:**
- Preserve broad deterministic coverage for both providers in default workflows.
- Verify drift-prone real integration paths with a tiny high-signal smoke suite.
- Ensure parse-path edge handling remains covered (malformed lines, partial events, non-zero exits).

**Test Organization:**
- Fake executable under `test/fakes/codex`.
- Fixtures under `test/fixtures/` with clear provider/scenario naming.
- Smoke tests in `*_smoke_test.go` files behind `//go:build smokecli`.
- Lane timing visibility via `scripts/test_timing.sh` lane-aware commands/reporting.

## Implementation Tasks

Priority convention for this plan: all tasks below are **P0 (high priority)**.

### Task 1 (P0): Add Fake Codex CLI With Claude-Parity Ergonomics

**Files:**
- Create: `test/fakes/codex`
- Test: `test/contracts/fakes_integration_test.go`

**What to Do:**
Implement a fake Codex executable with behavior parallel to fake Claude: invocation logging via `TEST_CALL_LOG`, stdin consumption, fixture-driven output for plain and JSONL modes, configurable non-zero failures, and optional delay toggles for timing tests.

**Acceptance Criteria:**
- `test/fakes/codex` supports call logging, fixture output, and failure toggles through env vars.
- JSONL stream fixture mode is supported for provider stream parsing tests.
- Contract fake-integration tests validate fake Codex success and failure behavior.

**Dependencies:**
- None

**Notes:**
Align env var naming with existing fake conventions to reduce cognitive load.

### Task 2 (P0): Add Codex and Claude Contract Fixtures From Real CLI Shapes

**Files:**
- Create: `test/fixtures/codex_success.txt`
- Create: `test/fixtures/codex_failure.txt`
- Create: `test/fixtures/codex_stream_success.jsonl`
- Create: `test/fixtures/codex_stream_failure.jsonl`
- Create/Modify: `test/fixtures/claude_stream_success.jsonl` (if missing representative shape)

**What to Do:**
Add representative fixture snapshots for both providers (success, failure, stream event shapes), structured for direct reuse by fake-backed tests. Include brief provenance comments where appropriate so refreshes are explicit.

**Acceptance Criteria:**
- Fixtures exist for both providers across success/failure/stream scenarios.
- Fixtures are consumed by tests rather than inline hardcoded JSON/script payloads.
- Fixture naming is consistent and scenario-driven.

**Dependencies:**
- Task 1

**Notes:**
Keep fixture payloads minimal but realistic to avoid brittle overfitting.

### Task 3 (P0): Wire Fake Codex Into Shared Contract/E2E Harness

**Files:**
- Modify: `test/contracts/helpers_test.go`
- Modify: `test/e2e/e2e_test.go`
- Test: `test/contracts/fakes_integration_test.go`

**What to Do:**
Ensure harness setup and helper utilities expose fake Codex in the same way fake Claude/Git/BD are exposed, and make it straightforward for contract/e2e tests to configure Codex fixtures and inspect call logs.

**Acceptance Criteria:**
- Contract and e2e harnesses can run tests that invoke fake Codex without global Codex installation/auth.
- Shared helpers provide consistent call-log filtering for Codex invocations.
- Existing non-Codex tests remain green after harness changes.

**Dependencies:**
- Task 1
- Task 2

**Notes:**
Keep harness APIs symmetric across providers to minimize test branching.

### Task 4 (P0): Migrate Codex Acceptance/Contract Tests to Canonical Fake

**Files:**
- Modify: `internal/provider/codex_streaming_acceptance_test.go`
- Modify: `test/contracts/*codex*` (create or update targeted files as needed)
- Test: `internal/provider/codex_test.go` (only if fixture plumbing needs minor updates)

**What to Do:**
Replace ad-hoc temp mock scripts in Codex acceptance/contract tests with fake Codex binary + fixture files. Preserve behavioral assertions (args, parsing, failure handling) while removing duplicated inline shell scaffolding.

**Acceptance Criteria:**
- Codex acceptance tests no longer create inline shell mock binaries for core scenarios.
- Tests assert behavior via fixtures and call logs through the shared fake harness.
- Default lane tests run hermetically without Codex auth or network.

**Dependencies:**
- Task 2
- Task 3

**Notes:**
Focus on high-signal behavior assertions; avoid retesting fake implementation internals.

### Task 5 (P0): Add Real-CLI Smoke Lane Behind Build Tag and Env Gates

**Files:**
- Create: `internal/provider/claude_codex_smoke_test.go`
- Modify/Create: supporting smoke helper file if needed under `internal/provider/`

**What to Do:**
Introduce a tiny `//go:build smokecli` test suite that exercises real provider entrypoints for Claude and Codex with explicit env gates (`CLAUDE_SMOKE=1`, `CODEX_SMOKE=1`). Add skip messaging that clearly indicates missing prerequisites.

**Acceptance Criteria:**
- Smoke tests are excluded from default `go test ./...`.
- Smoke suite contains at least one real Claude invocation and one real Codex invocation.
- Suite includes at least one failure-path assertion for real CLI behavior.

**Dependencies:**
- Task 4

**Notes:**
Target 3-6 tests total to keep smoke runtime bounded.

### Task 6 (P0): Document Lane Commands and Fixture Refresh Workflow

**Files:**
- Modify: `README.md` (or existing testing doc location)
- Modify/Create: `docs/testing.md` (if project convention prefers docs directory)

**What to Do:**
Document exact commands for fast default lane, smoke lane, and fixture refresh/update workflow, including required env gates and expected runtime intent.

**Acceptance Criteria:**
- Docs include runnable commands for default lane and smoke lane.
- Fixture refresh process is explicit and review-oriented.
- Docs clarify that smoke lane requires intentionally configured credentials/binaries.

**Dependencies:**
- Task 5

**Notes:**
If docs already have testing sections, extend instead of duplicating.

### Task 7 (P0): Add Lane-Aware Timing and Runtime Budget Visibility

**Files:**
- Modify: `scripts/test_timing.sh`
- Modify: `scripts/test_package_budgets.txt` (if per-lane budget entries are added)

**What to Do:**
Add lane-aware timing commands/reporting (default lane vs smoke lane), and make budget violations visible with explicit lane context so regressions are detectable.

**Acceptance Criteria:**
- Timing script supports measuring/reporting default and smoke lanes distinctly.
- Budget output identifies lane context for exceeded thresholds.
- Script remains backward compatible for existing timing workflow.

**Dependencies:**
- Task 5

**Notes:**
Smoke budgets should be intentionally looser than default lane budgets.

### Task 8 (P0): Final Verification and Stability Sweep

**Files:**
- Modify: any touched test/docs files as needed

**What to Do:**
Run the fast lane test commands and targeted smoke command shape checks (skip-expected without env), verify fixture-backed tests are deterministic, and ensure no accidental real-CLI dependency leaked into default paths.

**Acceptance Criteria:**
- Default test lane passes without Claude/Codex auth setup.
- Smoke lane is opt-in and skip-gated unless explicit env vars are set.
- Plan deliverables satisfy spec acceptance criteria and are ready for decompose.

**Dependencies:**
- Task 1
- Task 2
- Task 3
- Task 4
- Task 5
- Task 6
- Task 7

**Notes:**
Capture any leftover edge work as separate follow-up beads during decomposition.

---

## Notes

- Keep smoke tests intentionally minimal; they are integration sentinels, not a second acceptance suite.
- Prefer fixture-backed assertions over brittle full-output exact matching when only schema/shape matters.
- Preserve existing build-tag boundaries (`acceptance`, `contract`, `e2e`) and introduce `smokecli` as additive, not replacement.
