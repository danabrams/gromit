---
created: 2026-02-27T00:00:00Z
decomposed: true
decomposed_at: "2026-02-27T03:44:51Z"
id: phase-4-token-efficiency-rag-evaluation
source_spec: phase-4-token-efficiency-rag-evaluation
---

# Token Efficiency RAG Evaluation (Phase 4) Implementation Plan

**Goal:** Run a bounded, reproducible experiment to determine whether retrieval-assisted discovery reduces token spend and latency without degrading solution quality.

**Architecture:** Add an experimental local retrieval subsystem for discovery-only usage with strict attribution, staleness signaling, and confidence-gated fallback to baseline grep/read, then evaluate paired baseline vs retrieval scenarios through explicit adoption gates.

**Tech Stack:** Go, existing gromit benchmark/report pipeline (`cmd/gromit/benchmark.go`, `internal/benchmark`), runner token-efficiency routing hooks (`internal/runner`), prompt/policy seams (`internal/prompt`), YAML config (`internal/config`).

**Spec:** `.gromit/specs/phase-4-token-efficiency-rag-evaluation.md`

---

## Architecture

**Overview:**
Add a bounded retrieval experiment layer that can guide discovery queries but never replace deterministic source verification. Retrieval outputs must include file/line attribution and confidence metadata, with explicit staleness signaling. Evaluation runs paired baseline and retrieval-assisted scenarios and decides adoption strictly through measurable gates.

**Key Components:**
1. **Experimental Retrieval Package (`internal/retrieval`)**: Local index and query APIs returning top-K attributed snippets (`file`, `line_start`, `line_end`, `score/confidence`).
2. **Index Lifecycle Manager (`internal/retrieval/indexer.go`)**: Initial build from tracked files, incremental refresh support, and explicit stale/outdated state markers.
3. **Discovery Policy Layer (`internal/retrieval/policy.go` and runner/prompt integration seam)**: Retrieval is advisory only; source verification is mandatory; low-confidence or stale retrieval results force fallback to baseline tools.
4. **Phase-4 Evaluation Engine (`internal/benchmark/phase4.go`)**: Paired baseline/retrieval scenario metric aggregation and gate decision logic.
5. **Phase-4 Reporting CLI (`cmd/gromit/benchmark.go`)**: Command wiring for deterministic report artifact generation under `.gromit/reports/`.

**Integration Points:**
- Reuse benchmark/report artifact patterns from `internal/benchmark/phase3.go`.
- Reuse token-efficiency utility-routing category semantics (`discovery_indexing`) where needed for consistent telemetry.
- Keep retrieval behind explicit experiment scope/config; baseline workflow remains default and authoritative.

**Data Flow:**
1. Build retrieval index from tracked repository files for an experiment scenario set.
2. Query index during discovery stage to return attributed snippets and confidence/staleness metadata.
3. Apply policy gate:
- if confidence acceptable and index fresh: use retrieval as entry guidance and immediately verify by opening cited files/lines;
- otherwise: fall back to baseline grep/read discovery.
4. Capture paired baseline vs retrieval metrics (discovery tokens, discovery latency, success outcomes, wrong-file/mislocated-edit signals, operational overhead).
5. Compute medians and evaluate adoption gates; emit final adopt/no-adopt decision with rationale.

**Files to Modify:**
- `internal/config/config_types.go` - add retrieval experiment config surface.
- `internal/config/config_defaults.go` - default-off retrieval experiment settings.
- `internal/config/config_accessors.go` - retrieval feature toggles/threshold accessors.
- `cmd/gromit/benchmark.go` - add phase-4 report command and argument validation.
- `cmd/gromit/benchmark_test.go` - command wiring, validation, and dispatch tests.
- `internal/benchmark/report.go` - optional shared helper extensions if needed for gate report formatting.

**Files to Create:**
- `internal/retrieval/types.go` - request/response/index metadata structures.
- `internal/retrieval/indexer.go` - build/refresh and staleness detection.
- `internal/retrieval/query.go` - ranked snippet retrieval with line-range attribution.
- `internal/retrieval/policy.go` - confidence/staleness fallback policy.
- `internal/retrieval/indexer_test.go`
- `internal/retrieval/query_test.go`
- `internal/retrieval/policy_test.go`
- `internal/retrieval/staleness_test.go`
- `internal/benchmark/phase4.go` - paired comparison aggregation and adoption gate evaluation.
- `internal/benchmark/phase4_test.go` - gate math and report-structure tests.

**Tradeoffs:**
- **Local deterministic retrieval over full production semantic stack:** minimizes operational burden and improves reproducibility for experiment-first scope.
- **Advisory retrieval with mandatory verification over direct edit-driving retrieval:** protects correctness and avoids wrong-file edits.
- **Hard gate adoption decision over qualitative judgment:** keeps rollout evidence-driven and reversible.

## Test Strategy

**Test Levels:**
1. **Unit Tests**: index build/refresh, snippet attribution, staleness detection, policy fallback, and gate threshold evaluation.
2. **Integration Tests**: phase-4 benchmark/report path, CLI command wiring, and paired baseline-vs-retrieval artifact consistency.
3. **Manual Testing**: fixed scenario set execution to validate report outputs and adopt/no-adopt decision plausibility.

**Key Test Cases:**
- Retrieval returns top-K snippets with file path and exact line-range metadata.
- Retrieval response includes confidence score and these values affect policy decisions.
- Index staleness is detected when source files change without refresh and surfaced in results.
- Policy enforces fallback to baseline grep/read on low-confidence or stale retrieval state.
- Retrieval-assisted workflow still requires explicit source verification before edits.
- Phase-4 report contains paired baseline/retrieval medians for discovery input tokens and discovery latency.
- Adoption gates enforce:
  - median discovery input token reduction >= 20%
  - median discovery latency reduction >= 15%
  - no material task success-rate drop
  - wrong-file/mislocated-edit rate does not exceed threshold
  - operational overhead remains acceptable
- Any failing gate yields a deterministic no-adopt decision.

**Mocking Strategy:**
- Use fixture repositories/files for deterministic line-range attribution assertions.
- Use synthetic scenario metrics fixtures for gate-threshold boundary tests.
- Use fake clock and deterministic timestamps for stable report snapshots.
- Keep aggregation logic mostly unmocked to validate real reduction/gate computations.

**Coverage Goals:**
- Critical paths: attribution correctness, stale detection, fallback behavior, paired median/gate computations.
- Edge cases: empty index, empty scenario set, malformed records, confidence ties, and stale index with otherwise-high scores.
- Regression goal: baseline-only workflow remains unchanged when experiment toggle is disabled.

**Test Organization:**
- `internal/retrieval/indexer_test.go`
- `internal/retrieval/query_test.go`
- `internal/retrieval/staleness_test.go`
- `internal/retrieval/policy_test.go`
- `internal/benchmark/phase4_test.go`
- `cmd/gromit/benchmark_test.go`

---

## Implementation Tasks

### Task 1: Add Retrieval Experiment Config Surface

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config_defaults.go`
- Modify: `internal/config/config_accessors.go`
- Test: `internal/config/token_efficiency_config_test.go` (and related config tests)

**What to Do:**
Add additive config for phase-4 retrieval experiment controls (enable flag, top-K, confidence threshold, staleness policy/thresholds, and optional index path/storage bounds), defaulting to disabled.

**Acceptance Criteria:**
- New config keys parse and validate correctly.
- Defaults preserve existing behavior with retrieval disabled.
- Accessors expose nil-safe values for retrieval policy decisions.

**Dependencies:**
- None

**Notes:**
Follow existing token-efficiency naming conventions and keep backward compatibility with legacy configs.

### Task 2: Implement Retrieval Types and Index Lifecycle

**Files:**
- Create: `internal/retrieval/types.go`
- Create: `internal/retrieval/indexer.go`
- Create: `internal/retrieval/staleness.go`
- Test: `internal/retrieval/indexer_test.go`
- Test: `internal/retrieval/staleness_test.go`

**What to Do:**
Implement deterministic local index build from tracked files, incremental refresh hooks, and staleness metadata/state signaling.

**Acceptance Criteria:**
- Initial index build covers tracked project files deterministically.
- Incremental refresh updates modified/deleted file entries correctly.
- Stale/outdated state is detectable and queryable via explicit metadata.

**Dependencies:**
- Task 1

**Notes:**
Keep index schema simple and versioned to support future changes without breaking readers.

### Task 3: Implement Attributed Retrieval Querying

**Files:**
- Create: `internal/retrieval/query.go`
- Test: `internal/retrieval/query_test.go`

**What to Do:**
Implement top-K retrieval query API returning ranked snippets with file and line-range attribution plus score/confidence metadata.

**Acceptance Criteria:**
- Query results include file path and line boundaries for every snippet.
- Ranking is deterministic for stable index/query inputs.
- Confidence metadata is populated and bounded for downstream policy use.

**Dependencies:**
- Task 2

**Notes:**
Keep snippet extraction bounded and avoid returning large file regions that negate token savings.

### Task 4: Enforce Discovery Policy and Fallback Guardrails

**Files:**
- Create: `internal/retrieval/policy.go`
- Test: `internal/retrieval/policy_test.go`
- Modify: retrieval integration seam in runner/prompt discovery path (exact file(s) finalized during implementation)

**What to Do:**
Add policy evaluator that permits retrieval as discovery guidance only when confidence and staleness checks pass; otherwise force baseline grep/read fallback and require source verification before edits.

**Acceptance Criteria:**
- Low-confidence retrieval is rejected in favor of baseline flow.
- Stale index retrieval is rejected in favor of baseline flow.
- Verification-required signal is emitted for accepted retrieval results.

**Dependencies:**
- Task 1
- Task 3

**Notes:**
Policy outcomes should be explicit and telemetry-friendly to support auditability in phase-4 reports.

### Task 5: Build Phase-4 Paired Evaluation Engine

**Files:**
- Create: `internal/benchmark/phase4.go`
- Test: `internal/benchmark/phase4_test.go`

**What to Do:**
Implement paired baseline vs retrieval metric aggregation and adoption-gate evaluation logic for fixed scenario runs.

**Acceptance Criteria:**
- Report computes median discovery tokens and latency deltas between baseline and retrieval.
- Report tracks success-rate parity and wrong-file/mislocated-edit thresholds.
- Gate evaluator returns adopt/no-adopt decision with deterministic reasons.

**Dependencies:**
- Task 4

**Notes:**
Mirror the deterministic reporting style used by phase-3 measurement artifacts.

### Task 6: Add Phase-4 Benchmark Report Command and Artifacts

**Files:**
- Modify: `cmd/gromit/benchmark.go`
- Modify: `cmd/gromit/benchmark_test.go`
- Modify/Create: shared report serialization helper(s) in `internal/benchmark` as needed

**What to Do:**
Add CLI command for phase-4 report generation from paired run logs/artifacts and write JSON/Markdown outputs under `.gromit/reports/`.

**Acceptance Criteria:**
- Command validates required inputs and timestamp format.
- Command dispatches to phase-4 report writer and returns actionable errors.
- Deterministic report artifacts are written at expected locations.

**Dependencies:**
- Task 5

**Notes:**
Keep CLI shape consistent with existing `benchmark phase3-report` behavior.

### Task 7: Add End-to-End Regression and Non-Adoption Safety Coverage

**Files:**
- Modify: `internal/benchmark/phase4_test.go`
- Modify: `cmd/gromit/benchmark_test.go`
- Modify: any touched runner/prompt tests for baseline non-regression

**What to Do:**
Add high-signal integration/regression tests ensuring baseline behavior is unchanged when disabled and no-adopt outcomes are enforced when any gate fails.

**Acceptance Criteria:**
- Disabled retrieval experiment path matches baseline discovery behavior.
- Failing any single gate produces no-adopt decision.
- Passing all gates produces adopt decision with full gate evidence in output.

**Dependencies:**
- Task 6

**Notes:**
This task is the final confidence pass before decomposition/implementation.

---

## Notes

- Phase 4 remains evaluation-first: no production rollout commitment is implied by implementation.
- Retrieval output is never authoritative; cited files/lines must be opened and verified before edits.
- If gate results are mixed or operational overhead is high, default outcome is no-adopt with documented findings.
- Tasks are intentionally scoped so `gromit decompose phase-4-token-efficiency-rag-evaluation` can split each into 1-3 beads with clear dependencies.
