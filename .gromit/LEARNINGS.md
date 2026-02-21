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

### 2026-02-11 | Prompt Template Structure | conventions
*Related to: gromit-rpne*

Prompt templates in .gromit/templates/ use explicit section headers (##) and preserve exact whitespace/structure when updating. Template files follow a consistent structure: context section at top, then Guidelines, then preserved sections like 'Avoiding Sibling Overlap' and ATDD blocks. When modifying sections, maintain blank lines between sections and ensure downstream blocks remain unchanged. Acceptance tests for template changes must match the exact content being added, including specific phrases and subsection structure.

### 2026-02-16 | Provider Contract Fixtures | patterns
*Related to: gromit-d7j9*

Contract tests consume canonical provider fixtures under test/fixtures/ using scenario-driven naming: `{provider}[_stream]_{outcome}.{format}`. Fixtures (codex_success.txt, codex_failure.txt, codex_stream_success.jsonl, codex_stream_failure.jsonl, claude_stream_success.jsonl) must include brief provenance comments describing the source and refresh workflow. Payloads should be minimal but realistic—Codex plain-text fixtures show output structure (touched/tests lines), JSONL fixtures emit `{"type":"assistant",...}` and `{"type":"result",...}` events. Fixture environment variables (CODEX_FIXTURE, CLAUDE_FIXTURE) point fake CLIs to fixture paths. Test assertions verify output matches canonical payloads, enabling both roundtrip validation and contract evolution tracking. Provenance comments facilitate fixture refresh workflow without manual intervention.

---

## Provisional

*Seen once - may be specific to one task.*

### 2026-02-20 | Package-Level Routing Consolidation Leaves Dead Helpers | gotchas
*Related to: code-review*

When migrating inline routing logic (e.g., manual test-only tier override in setupBeadContext) to a package-level function (escalation.SelectTier), check for helper functions at the old call site that become dead code. escalation.SelectTier() consolidates low-complexity signal counting, test-only routing, complexity:high bypass, and config fallback into one function, eliminating the need for hasComplexityLabelOverride and similar inline helpers.

### 2026-02-20 | Cost/Token Accounting Needs Consistent Delta Semantics | gotchas
*Related to: code-review*

Cost/token tracking uses inconsistent accumulation patterns: (1) PhaseMetric recording — the green phase uses before/after usage snapshots via snapshotIterationUsage() but red and refactor phases use recordPhaseMetric() without snapshots, mixing per-phase deltas with raw values. (2) Codex stream events — turn.completed overwrites usage while response.completed and result events merge via mergeCodexUsage(). Both patterns should use explicit before/after snapshots for phases and consistent merge semantics for stream events to make cost attribution reliable for retrospective analysis.

### 2026-02-21 | Clean Exit False With Successful Iterations Suggests Post-Loop Failure | gotchas
When analyzing Gromit failures, check both the run logs and state.json. A clean_exit=false with successful recorded iterations suggests the failure occurred during post-iteration cleanup or during the final validation gate itself, possibly a timeout or signal interruption.

### 2026-02-21 | ATDD Fallback Generalization Uses Classification-Policy-Execution Separation | patterns
*Related to: code-review*

classifyATDDFailure + isATDDFallbackEligible + runTransientFallback cleanly separates failure classification from eligibility policy from fallback execution. Adding new transient failure categories (beyond transport_disconnect and startup_error) requires only adding signals to the classification function and updating isATDDFallbackEligible, without touching the fallback execution flow.

### 2026-02-21 | Scoped Spec Orchestration Decouples Per-Bead ATDD From Spec-Level Verification | patterns
*Related to: code-review*

Scoped spec orchestration (author before loop, verify after bead exhaustion with retry budget) correctly separates per-bead build work from spec-level acceptance verification. When granularity=spec, per-bead ATDD is always skipped regardless of spec label presence because acceptance testing happens at the spec boundary via verifyScopedSpecAcceptance.

### 2026-02-21 | t.Chdir Migration Must Include New Test Code Not Just Existing | conventions
*Related to: code-review*

When migrating tests from manual os.Getwd/os.Chdir/defer patterns to t.Chdir(), apply the migration uniformly to all new test code in the same changeset, not just existing tests being modified. New tests using the old pattern create inconsistency that compounds over time.

---

## Archived

*No longer relevant or superseded.*

*No archived learnings.*

