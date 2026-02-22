# Learnings

Accumulated operational knowledge from Gromit iterations.
This file is automatically updated. Review periodically with `gromit retro`.

---

## Confirmed

*Patterns seen multiple times - high confidence.*

### 2026-02-07 | Status File Management | patterns
*Related to: nalr, k8c2, kydj, ead1, xpfn, lm34, 2y2d, yj2h, vpyl, kim2*

Status struct fields require backward-compatible changes (omitempty for new optional fields). Use ReadStatus()/IsProcessAlive() for state + liveness checks. Return nil,nil for missing optional files (not an error). StatusWriter handles both lifecycle states and preserves completed iteration count on shutdown. Round-trip tests verify serialization fidelity. Stale resource cleanup integrates into status reporting via process liveness checks. Process utilities (IsProcessAlive) are co-located with Status in status.go. Test file I/O uses t.TempDir() for isolation.

### 2026-02-07 | Methodology Label Activation | patterns
*Related to: ralph-runner-4a3f, ralph-runner-nzue*

Methodologies use label-based activation ("methodology:true"/"false") with global config fallback via bead.IsMethodologyActive(). When active, replace the build prompt with a specialized RenderXXXBuild method. Check parent labels before adding globally-active methodology labels to sub-beads to avoid duplicates. Order methodology checks carefully for precedence when multiple methodologies are active.

### 2026-02-22 | Prompt Assembly Integrity | conventions
*Related to: gromit-rpne, review-1771763626626526682*

Prompt assembly integrity requires both strict template structure fidelity and explicit error handling in rule/template loaders. Preserve section/whitespace contracts in template files and never discard loader errors; return or warn with phase context so degraded prompts are observable.

### 2026-02-16 | Provider Contract Fixtures | patterns
*Related to: gromit-d7j9*

Contract tests consume canonical provider fixtures under test/fixtures/ using scenario-driven naming: `{provider}[_stream]_{outcome}.{format}`. Fixtures (codex_success.txt, codex_failure.txt, codex_stream_success.jsonl, codex_stream_failure.jsonl, claude_stream_success.jsonl) must include brief provenance comments describing the source and refresh workflow. Payloads should be minimal but realistic—Codex plain-text fixtures show output structure (touched/tests lines), JSONL fixtures emit `{"type":"assistant",...}` and `{"type":"result",...}` events. Fixture environment variables (CODEX_FIXTURE, CLAUDE_FIXTURE) point fake CLIs to fixture paths. Test assertions verify output matches canonical payloads, enabling both roundtrip validation and contract evolution tracking. Provenance comments facilitate fixture refresh workflow without manual intervention.

### 2026-02-22 | Stage Contract & Soft-Failure Orchestration | patterns
*Related to: 9980dae8, gromit-22nrv, 8d85f5d7*

Stage-based orchestration should keep business logic in stages with typed I/O, dependency injection via builder methods, and explicit inter-stage state flow. Optional dependencies may soft-fail with warnings only when explicitly optional; critical contracts (validation output, touched package carryover, iteration numbering) require deterministic tests.

---

## Provisional

*Seen once - may be specific to one task.*

### 2026-02-20 | Cost/Token Accounting Needs Consistent Delta Semantics | gotchas
*Related to: code-review*

Cost/token tracking uses inconsistent accumulation patterns: (1) PhaseMetric recording — the green phase uses before/after usage snapshots via snapshotIterationUsage() but red and refactor phases use recordPhaseMetric() without snapshots, mixing per-phase deltas with raw values. (2) Codex stream events — turn.completed overwrites usage while response.completed and result events merge via mergeCodexUsage(). Both patterns should use explicit before/after snapshots for phases and consistent merge semantics for stream events to make cost attribution reliable for retrospective analysis.

### 2026-02-22 | Orchestrator Migration Safety & Parity | conventions
*Related to: code-review, review-1771733992016921570*

Orchestrator migration must preserve behavior parity with legacy Runner while dual paths coexist. Enforce migration safety checks: complete call-site migration before deletion, verify exported symbol reachability, compile acceptance-tagged tests (go test -tags acceptance -run '^$' ./...), remove keep-alive forced imports, and add parity tests for cost tracking/state saving across both paths until legacy removal.

## Emerging

*Newly observed — needs validation across more tasks.*


### 2026-02-22 | SPC Display Formatting Two-Tier Pattern | patterns
*Related to: review-1771784092725425988*

SPC (Statistical Process Control) formatting follows a two-tier pattern: formatSPCSummary orchestrates sections (window, control limits, anomalies), while formatSPCLine/formatSPCValue handle individual metric values. simplifySPCMetric provides human-friendly labels for anomaly display (e.g., "rolling_success_rate" → "success"). Keep metric name constants in sync between the logger package (which produces them) and the runner/format package (which displays them) — string-based coupling requires test coverage since there's no compile-time check.

### 2026-02-22 | Interface Evolution Through Signature Changes | patterns
*Related to: review-1771784092725425988*

When evolving function signatures (e.g., StatusWriter adding a deadline parameter), propagate changes through: (1) the type definition (OrchestratorConfig), (2) all call sites (orchestrator.go), (3) all implementations (constructor.go closure), and (4) all test doubles (orchestrator_test.go fakes). The StatusWriter deadline addition was a clean example — the new parameter flowed naturally through all four layers without breaking existing behavior for callers that pass zero-value deadlines.

### 2026-02-22 | Three-Layer Requirement Extraction Fallback | patterns
*Related to: review-1771784092725425988*

Requirement extraction uses a 3-layer fallback: Layer 1 (ExpectedOutputs field from bead JSON) → Layer 2 (description parsing with headers, bullets, inline headers, comma-separated lists) → Layer 3 (LLM extraction via haiku). The inline-header ("Functions: X, Y, Z") and comma-list improvements in Layer 2 reduce dependency on the expensive LLM layer. Each layer is independently testable. When adding parsing heuristics, prefer broadening Layer 2 over relying on Layer 3 — deterministic parsing is cheaper and more predictable than LLM extraction.

### 2026-02-22 | Decompose Prompt Expected Outputs as Leverage | conventions
*Related to: review-1771784092725425988*

Including expected_outputs in the decompose prompt template is high-leverage: decomposition quality determines downstream TDD cycle granularity (one red-green cycle per expected output). When the LLM doesn't produce expected_outputs, the system falls back to acceptance_criteria parsing, which may be coarser-grained. Explicitly instructing "list each individual deliverable as a separate entry — these drive TDD RED-GREEN cycles" produces fine-grained outputs that match the system's mechanical needs.

### 2026-02-22 | SCOPE_GATE_ERROR_PROPAGATION_CHANGE | ARCHITECTURE
*Related to: review-1771788120407657627*

runScopeGate in gate.go now propagates decomposition failures as errors instead of falling through to Block decision. This is a deliberate semantic shift — transient decomposition failures (network, bd CLI) will error the gate rather than blocking the bead for retry. Callers should handle gate errors accordingly.

### 2026-02-22 | DECOMPOSER_ADAPTER_MINIMAL_DECOMPOSITION | ARCHITECTURE
*Related to: review-1771788120407657627*

The decomposerAdapter implementation creates a single child bead with title "(decomposed)" suffix and closes the parent. This is a minimal decomposition — it doesn't split work into multiple sub-beads based on expected outputs. Real decomposition intelligence comes from the pipeline.Decompose() path (provider-parity-decompose-retro spec).

### 2026-02-22 | TEST_HELPER_DUPLICATION_IN_GATE_TESTS | TEST_QUALITY
*Related to: review-1771788120407657627*

gate_test.go accumulated 6+ near-identical mock decomposer/bead-client types across iterative TDD work. When adding test doubles incrementally, check if existing mocks can be parameterized rather than creating new types. Per project rule: "2+ tests sharing 10+ lines of setup: extract a setup helper."

### 2026-02-22 | SCOPE_GATE_DECOMPOSITION_NEEDS_STATE_SAFETY | ARCHITECTURE
*Related to: review-1771797265171555605*

Scope-gate decomposition is resilient to provider failures (falls back to Block), but sequential child creation without idempotency safeguards can leave partial state and duplicate work on retries. Decomposition paths should enforce either rollback or deduped re-entry semantics before parent close.

### 2026-02-22 | BEHAVIOR_FIRST_TESTS_OVER_SOURCE_READING | TEST_QUALITY
*Related to: review-1771797265171555605*

Source-reading tests (e.g., checking function text with os.ReadFile + strings.Contains) are brittle and violate project testing guidance. Prefer behavioral assertions on public interfaces/contracts and compile-time guarantees, then use shared helpers when setup repeats.

### 2026-02-22 | DECOMPOSITION_OUTPUT_CONTRACTS_MUST_BE_STRICT | CONVENTIONS
*Related to: review-1771797265171555605*

JSON parsing alone is not enough for LLM decomposition output. Gate paths should validate concrete quality constraints (sub-task count bounds, non-empty titles, bounded expected outputs, no degenerate parent echo) and define deterministic fallback behavior when outputs violate the contract.

---

## Archived

*Previously archived learnings.*

### 2026-02-21 | Compile-Time Invariant Enforcement vs Dead Code Patterns | conventions
*Related to: f668688fead2f958, 5912b9aee59cce5e, code-review*

Use package-level `var _ Interface = (*Impl)(nil)` declarations in non-test `.go` files to enforce architectural invariants at compile time. A check inside a test function body gates test compilation only — it does not gate production builds. Avoid tests that use `os.ReadFile`+`strings.Contains` on `.go` source files — they break silently on renames/moves. Distinguish from forced-import keep-alive patterns (`var _ = Type{}`): these exist solely to prevent the compiler from removing an unused import, indicate incomplete refactoring, and should be removed. When removing package usage, search for these keep-alive patterns in the same file and related consumers.

*Archived from provisional: filtered: generic engineering advice*

