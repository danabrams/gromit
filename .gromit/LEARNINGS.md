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

### 2026-02-21 | Validation Auto-Fix Pattern with Soft Failures | patterns
*Related to: 9980dae8*

The Validate stage uses a soft-failure pattern where unresolved validation failures don't block the pipeline—instead, ValidationFailures are populated and fed into the next Build stage input. This enables iterative improvement: auto-fix attempts gofmt/goimports on changed files, re-validates, and returns Proceed regardless of outcome. Periodic full validation is gated via modulo arithmetic (`iteration % FullValidationEveryN == 0`) to balance speed on fast iterations with thorough checks at configured boundaries. Mandatory command prefix policy enforcement happens upfront via checkMandatoryPrefixes() before running any commands, preventing silent misconfiguration. The stage uses local dependency interfaces (CommandRunner, AutoFixer) injected via builder pattern methods (WithAutoFixer, WithWorkDir), allowing optional composition and graceful degradation when auto-fix is nil. Compile-time check (`var _ pipeline.Stage = (*Validate)(nil)`) enforces the architectural contract. Table-driven tests cover auto-fix success, auto-fix+still-failing, periodic gate boundary transitions, mandatory prefix violations, validation disabled, and nil auto-fixer scenarios.

### 2026-02-21 | Pipeline Stage Patterns: Dependency Injection with Optional Interfaces | patterns
*Related to: gromit-22nrv*

Pipeline stages use local dependency interfaces (Prechecker, StuckDetector) injected via builder pattern methods (WithPrechecker, WithStuckDetector), allowing optional composition. Nil checks in Run() enable graceful degradation when a dependency isn't configured. Errors from dependencies are logged as warnings but don't block execution—this prevents checker failures from breaking the pipeline. Decision ordering matters: precheck (Skip) runs before stuck detection (Block) to ensure already-completed work is closed promptly, even if the bead has exceeded the failure threshold. Table-driven tests comprehensively cover all paths (Proceed, Skip, Block), nil components, optional configurations, and error handling. Compile-time interface checks (`var _ pipeline.Stage = (*Gate)(nil)`) verify architectural invariants.

### 2026-02-20 | Cost/Token Accounting Needs Consistent Delta Semantics | gotchas
*Related to: code-review*

Cost/token tracking uses inconsistent accumulation patterns: (1) PhaseMetric recording — the green phase uses before/after usage snapshots via snapshotIterationUsage() but red and refactor phases use recordPhaseMetric() without snapshots, mixing per-phase deltas with raw values. (2) Codex stream events — turn.completed overwrites usage while response.completed and result events merge via mergeCodexUsage(). Both patterns should use explicit before/after snapshots for phases and consistent merge semantics for stream events to make cost attribution reliable for retrospective analysis.

### 2026-02-21 | Architectural Invariant Enforcement Pattern | conventions
*Related to: f668688fead2f958, 5912b9aee59cce5e, code-review*

Use package-level `var _ Interface = (*Impl)(nil)` declarations in non-test `.go` files to enforce architectural invariants at compile time. A check inside a test function body gates test compilation only — it does not gate production builds. Avoid tests that use `os.ReadFile`+`strings.Contains` on `.go` source files — they break silently when functions are renamed or files move, skip when the working directory is wrong, and only gate test compilation rather than production builds. Replace source-reading tests with compile-time var checks or behavioral integration tests that exercise the actual behavior being guarded.

### 2026-02-21 | Multi-Stage Pipeline Orchestrator Pattern | patterns
*Related to: 8d85f5d7*

Replace God Object pattern with pure orchestration: hold only stage references and config, no business logic. The Orchestrator struct contains just a config field; all per-stage logic lives in internal/pipeline/<stage>/. Enforce import discipline at the orchestrator level—import only internal/pipeline and internal/logger. Wire stages at construction time via OrchestratorConfig, making dependency graph explicit and mockable. Assign iteration numbers monotonically regardless of outcome (including beads blocked at Gate), preserving failure chains. Flow inter-stage outputs into subsequent iterations: ValidationFailures from Validate→Build Input, TouchedPackages from Epilogue→next iteration Input. Keep failed stages in Epilogue for logging/cleanup rather than early-exit—this ensures consistent logging and status updates. Handle optional stages (Review) via nil checks at runtime, not construction time. Merge global stats atomically at completion, preserving prior entries—use read-modify-write with idempotency checks. Benefit: stages become independently testable, sequencing is explicit and debuggable, and stage coupling is minimal.

---

## Emerging

*Newly observed — needs validation across more tasks.*

### 2026-02-22 | Pipeline-to-Orchestrator Adapter Proliferation and Dual-Level Architecture | patterns
*Related to: code-review*

The migration from a monolithic Pipeline struct to a stage-based Orchestrator introduces structural friction that manifests in three ways: (1) Adapter proliferation — 12+ adapter types in constructor.go (invokerAdapter, renderAdapter, cmdRunnerAdapter, etc.) bridge pipeline stage interfaces to existing infrastructure interfaces, signaling that stage interfaces have diverged from the pre-existing contracts. Future stage interfaces should be designed to minimize adapter surface area. (2) Asymmetric inter-stage state — the Orchestrator correctly clears validationFailures on success but accumulates touchedPackages indefinitely across the run. This is intentional (packages stay touched for the full run) but the asymmetry is non-obvious; document it to prevent accidental "fix" attempts that reset touchedPackages. (3) Semantic duplicate helpers — pipeline review stage's buildFromReviewLabels and the parent pipeline's buildReviewBeadLabels perform overlapping label construction, arising from the dual-level architecture (stage sub-packages vs workflow methods on Pipeline struct). This dual-purpose design is reasonable during migration but needs inline documentation to prevent confusion about which function to call.

### 2026-02-22 | Dead Code via Forced Import Keep-Alive Patterns | gotchas
*Related to: code-review*

The `var _ = Type{}` pattern in files like callbacks_validation.go is used as a forced-import keep-alive — it exists solely to prevent the Go compiler from removing an otherwise-unused import. This pattern indicates incomplete refactoring: the real usage was deleted but the artificial reference was left behind. When removing usage of a package, search for these keep-alive patterns (`var _ =`, `var _ Type`) in the same file and in files that previously consumed the package. Unlike compile-time interface checks (`var _ Interface = (*Impl)(nil)`), which enforce architectural invariants and should be preserved, bare forced-import patterns are dead code and should be removed.

---

## Archived

*Moved to LEARNINGS_ARCHIVE.md to reduce prompt context overhead.*

