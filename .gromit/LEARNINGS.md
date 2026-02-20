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

### 2026-02-19 | Agent Resolver Adapter Duplication | patterns
*Related to: code-review*

Agent resolver adapters (cliAgentResolver, agentResolverAdapter, exploreAgentResolver) are copy-pasted across cmd/gromit files — any interface change requires updating 3+ places.

### 2026-02-20 | Runner argv injection propagates through spec orchestration | conventions
*Related to: successful bead*

Threading `r.argvRunnerFn` into `SpecOrchestrator` construction keeps acceptance-test orchestration on the injected runner path instead of default behavior. Ensure all spec orchestrator creation paths copy that runner function so git/test argv commands remain testable and environment-consistent end-to-end.

### 2026-02-20 | Client.run uses RunFn as the only test hook before subprocess | patterns
*Related to: successful bead*

`(*Client).run()` should check only `RunFn` first for override execution; when `RunFn` is nil it must execute the existing subprocess flow (`exec.Command` + `c.Dir` + `Output` + `*exec.ExitError` stderr wrapping) unchanged. This keeps injectable tests isolated while preventing accidental divergence in real `bd` invocation behavior.

### 2026-02-20 | PhaseMetric Must Record Per-Phase Deltas Not Aggregates | gotchas
*Related to: code-review*

PhaseMetric cost/token tracking needs a clear convention: record per-phase deltas (via before/after snapshots) for all phase types, not raw cumulative values. The green phase already uses snapshots but red and refactor phases do not.

### 2026-02-20 | Codex Stream Events Need Consistent Usage Merge Semantics | gotchas
*Related to: code-review*

Codex stream event handling uses two different patterns for usage accumulation: overwrite (turn.completed) vs merge (response.completed, result). Multi-turn sessions need consistent merge semantics.

### 2026-02-20 | Phase-Specific Renderer Interfaces Eliminate Stub Methods | patterns
*Related to: code-review*

Splitting a monolithic PromptRenderer into five phase-specific interfaces (RefineRenderer, PlanRenderer, DecomposeRenderer, ReviewRenderer, ExploreRenderer) eliminates stub not-implemented methods from adapters and enables compile-time single-responsibility enforcement. Each adapter only implements the method its pipeline needs.

### 2026-02-20 | Signal-Based Low-Complexity Tier Routing | patterns
*Related to: code-review*

Low-complexity tier selection uses countLowComplexitySignals >= threshold (default 2) instead of a single-dimension test-only check. Signals: low-complexity title pattern, test-only bead, tdd:false label, 1-3 expected files, leaf bead. complexity:high label bypasses low-complexity routing entirely.

### 2026-02-20 | OriginalTier/ActualTier Escalation Tracking | conventions
*Related to: code-review*

OriginalTier is set once in setupBeadContext from the initial SelectTier call; ActualTier is captured via deferred bc.Tier snapshot after escalation may have changed it. This pair enables retrospective analysis of escalation effectiveness.

### 2026-02-20 | Runbook Capture Best-Effort Semantics | conventions
*Related to: code-review*

Runbook capture uses best-effort semantics: getHead and runbook.Append failures are logged as warnings, not propagated as errors. This prevents auxiliary diagnostics from interrupting the main iteration loop.

---

## Archived

*Moved to LEARNINGS_ARCHIVE.md to reduce prompt context overhead.*

