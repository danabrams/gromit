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

### 2026-02-23 | Decomposition Contract & State Safety | architecture
*Related to: review-1771788120407657627, review-1771797265171555605*

Scope-gate decomposition must enforce strict output contracts (bounded sub-task count, non-empty titles/expected outputs, no parent-echo), and execution must be state-safe/idempotent (deduped re-entry or rollback on partial child creation). Decomposition failures should have deterministic fallback semantics and explicit error handling paths. Error propagation in gate.go uses errors (not fall-through to Block) for transient failures. Minimal decomposerAdapter creates single-child beads; real decomposition intelligence comes from the pipeline.Decompose() path.

---

## Provisional

*Seen once - may be specific to one task.*

*(empty — all provisional entries consolidated or archived)*

## Emerging

*Newly observed — needs validation across more tasks.*

### 2026-02-23 | Scoped Status Progress Recomputed at Read-Time | PATTERNS
*Related to: gromit-tlhuh*

Status progress denominator should be recomputed at display/read time for scoped runs instead of trusting persisted totals. Persisted `iteration_total` can become stale across process lifetimes, so status rendering should infer scope from `scope_label` (or active bead labels as fallback) and recalculate totals from current open non-epic work.

### 2026-02-23 | Retry Classification Must Not Depend on Broad Error Substrings | RELIABILITY
*Related to: gromit-tlhuh*

Session worktree retries should only trigger on explicit contention signals (branch/worktree collision). Broad substring matching like `already exists` can mask non-contention failures and reduce debuggability; retry paths need precise classification and tests for exhaustion and mixed failure sequences.

### 2026-02-22 | SPC Display Formatting Two-Tier Pattern | patterns
*Related to: review-1771784092725425988*

SPC (Statistical Process Control) formatting follows a two-tier pattern: formatSPCSummary orchestrates sections (window, control limits, anomalies), while formatSPCLine/formatSPCValue handle individual metric values. simplifySPCMetric provides human-friendly labels for anomaly display (e.g., "rolling_success_rate" → "success"). Keep metric name constants in sync between the logger package (which produces them) and the runner/format package (which displays them) — string-based coupling requires test coverage since there's no compile-time check.

### 2026-02-22 | Three-Layer Requirement Extraction Fallback | patterns
*Related to: review-1771784092725425988*

Requirement extraction uses a 3-layer fallback: Layer 1 (ExpectedOutputs field from bead JSON) → Layer 2 (description parsing with headers, bullets, inline headers, comma-separated lists) → Layer 3 (LLM extraction via haiku). The inline-header ("Functions: X, Y, Z") and comma-list improvements in Layer 2 reduce dependency on the expensive LLM layer. Each layer is independently testable. When adding parsing heuristics, prefer broadening Layer 2 over relying on Layer 3 — deterministic parsing is cheaper and more predictable than LLM extraction.

### 2026-02-22 | Decompose Prompt Expected Outputs as Leverage | conventions
*Related to: review-1771784092725425988*

Including expected_outputs in the decompose prompt template is high-leverage: decomposition quality determines downstream TDD cycle granularity (one red-green cycle per expected output). When the LLM doesn't produce expected_outputs, the system falls back to acceptance_criteria parsing, which may be coarser-grained. Explicitly instructing "list each individual deliverable as a separate entry — these drive TDD RED-GREEN cycles" produces fine-grained outputs that match the system's mechanical needs.

### 2026-02-22 | gromit-urweh.3 | conventions
Queue output has strict section ordering conventions (auth before api) that must be preserved when modifying queue.go - test expectations document the required ordering

### 2026-02-22 | gromit-3poct | gotchas
When a task is closed with reason 'Closed' and subtasks are created, it indicates planned decomposition rather than failure. Check subtask status for actual work state.

### 2026-02-23 | Failure Log Capture Completeness | gotchas
*Related to: gromit-p0iei, gromit-n2xw8, gromit-jp44h*

When tasks fail in gromit, ensure error_output and failure_category are populated in iteration logs. Blank failure_category indicates a logging/capture issue, not task success. Check .gromit/logs/ JSONL files and git status for implementation state when failure details are missing from the prompt.

---

## Archived

*Previously archived learnings.*

### 2026-02-21 | Compile-Time Invariant Enforcement vs Dead Code Patterns | conventions
*Related to: f668688fead2f958, 5912b9aee59cce5e, code-review*

Use package-level `var _ Interface = (*Impl)(nil)` declarations in non-test `.go` files to enforce architectural invariants at compile time. A check inside a test function body gates test compilation only — it does not gate production builds. Avoid tests that use `os.ReadFile`+`strings.Contains` on `.go` source files — they break silently on renames/moves. Distinguish from forced-import keep-alive patterns (`var _ = Type{}`): these exist solely to prevent the compiler from removing an unused import, indicate incomplete refactoring, and should be removed. When removing package usage, search for these keep-alive patterns in the same file and related consumers.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-22 | gromit-urweh | conventions
When analyzing task failures, always provide the actual error output or test failure logs - task status alone doesn't indicate whether implementation was successful.

*Archived from new: filtered: generic engineering advice*

### 2026-02-23 | gromit-xnp4e | conventions
When requesting failure analysis, include the error message, assertion output, or failure log. Task closure status alone is insufficient to diagnose root cause.

*Archived from new: filtered: generic engineering advice*

### 2026-02-23 | Usage Accounting & Migration Parity (consolidated) | conventions
*Related to: code-review, review-1771733992016921570*

Usage accounting and orchestrator migration require one shared semantics path: explicit before/after phase snapshots, single stream-event merge strategy, and parity tests while legacy/new paths coexist.

*Archived from provisional: codified in Build Process rules (snapshot semantics, merge strategy, telemetry completeness gate)*

### 2026-02-23 | Test Helper & Behavioral Assertion Hygiene (consolidated) | test_quality
*Related to: review-1771788120407657627, review-1771797265171555605*

Test suites should prefer behavioral assertions and shared helpers over brittle source-reading checks and duplicated mock/setup scaffolding.

*Archived from emerging: redundant with Test Quality rules (setup-helper extraction, source-reading ban)*

### 2026-02-23 | Interface Evolution Through Signature Changes | patterns
*Related to: review-1771784092725425988*

When evolving function signatures (e.g., StatusWriter adding a deadline parameter), propagate changes through: (1) the type definition (OrchestratorConfig), (2) all call sites (orchestrator.go), (3) all implementations (constructor.go closure), and (4) all test doubles (orchestrator_test.go fakes).

*Archived from emerging: generic interface-evolution advice with low project-specific leverage*
