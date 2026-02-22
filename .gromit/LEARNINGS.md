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

### 2026-02-21 | Pipeline Stage Dependency Injection and Soft Failure Patterns | patterns
*Related to: 9980dae8, gromit-22nrv*

Pipeline stages use local dependency interfaces injected via builder pattern methods (WithAutoFixer, WithPrechecker, WithStuckDetector), allowing optional composition. Nil checks in Run() enable graceful degradation when a dependency isn't configured—errors from optional dependencies are logged as warnings, not pipeline blockers. Compile-time checks (`var _ pipeline.Stage = (*Impl)(nil)`) enforce architectural contracts. The Validate stage uses a soft-failure pattern: unresolved validation failures populate ValidationFailures for the next Build input rather than blocking the pipeline. Auto-fix (gofmt/goimports) runs first, re-validates, and returns Proceed regardless. Periodic full validation is gated via modulo arithmetic. Mandatory command prefix enforcement happens upfront via checkMandatoryPrefixes(). Decision ordering matters in Gate: precheck (Skip) runs before stuck detection (Block) to ensure already-completed work is closed promptly.

### 2026-02-20 | Cost/Token Accounting Needs Consistent Delta Semantics | gotchas
*Related to: code-review*

Cost/token tracking uses inconsistent accumulation patterns: (1) PhaseMetric recording — the green phase uses before/after usage snapshots via snapshotIterationUsage() but red and refactor phases use recordPhaseMetric() without snapshots, mixing per-phase deltas with raw values. (2) Codex stream events — turn.completed overwrites usage while response.completed and result events merge via mergeCodexUsage(). Both patterns should use explicit before/after snapshots for phases and consistent merge semantics for stream events to make cost attribution reliable for retrospective analysis.

### 2026-02-21 | Compile-Time Invariant Enforcement vs Dead Code Patterns | conventions
*Related to: f668688fead2f958, 5912b9aee59cce5e, code-review*

Use package-level `var _ Interface = (*Impl)(nil)` declarations in non-test `.go` files to enforce architectural invariants at compile time. A check inside a test function body gates test compilation only — it does not gate production builds. Avoid tests that use `os.ReadFile`+`strings.Contains` on `.go` source files — they break silently on renames/moves. Distinguish from forced-import keep-alive patterns (`var _ = Type{}`): these exist solely to prevent the compiler from removing an unused import, indicate incomplete refactoring, and should be removed. When removing package usage, search for these keep-alive patterns in the same file and related consumers.

### 2026-02-21 | Multi-Stage Pipeline Orchestrator Pattern | patterns
*Related to: 8d85f5d7*

Replace God Object pattern with pure orchestration: hold only stage references and config, no business logic. The Orchestrator struct contains just a config field; all per-stage logic lives in internal/pipeline/<stage>/. Enforce import discipline at the orchestrator level—import only internal/pipeline and internal/logger. Wire stages at construction time via OrchestratorConfig, making dependency graph explicit and mockable. Assign iteration numbers monotonically regardless of outcome (including beads blocked at Gate), preserving failure chains. Flow inter-stage outputs into subsequent iterations: ValidationFailures from Validate→Build Input, TouchedPackages from Epilogue→next iteration Input. Keep failed stages in Epilogue for logging/cleanup rather than early-exit—this ensures consistent logging and status updates. Handle optional stages (Review) via nil checks at runtime, not construction time. Merge global stats atomically at completion, preserving prior entries—use read-modify-write with idempotency checks. Benefit: stages become independently testable, sequencing is explicit and debuggable, and stage coupling is minimal.


## Emerging

*Newly observed — needs validation across more tasks.*

### 2026-02-22 | Orchestrator Migration Adapter Patterns | patterns
*Related to: code-review, review-1771733992016921570*

The Orchestrator migration introduces adapter proliferation (12+ types in constructor.go) bridging stage interfaces to infrastructure — future stage interfaces should minimize this surface. Key patterns: (1) Consolidation — one exported function in the parent package (e.g., BuildFromReviewLabels), child packages import it; remove deprecated wrappers once callers migrate. (2) File extraction — enforced by file_size_test.go with a 550-line limit. (3) Dual-path risk — Orchestrator and legacy Runner have separate code for the same operations (cost tracking, state saving); features wired in one path may be silently missing in the other. (4) Asymmetric state — validationFailures clear on success while touchedPackages accumulate across the run (intentional but non-obvious). (5) Copy-paste bugs — adapters with similar methods (RenderBuild/RenderRefactor) need independent delegation target verification. (6) FnField mocks — nil-safe with explicit nil check, injected via deps struct. Always sort map keys in logging functions for deterministic output.

### 2026-02-22 | Silent Error Swallowing in Render Builder Functions | gotchas
*Related to: review-1771763626626526682*

The TDD render builder functions (buildRenderRedFn, buildRenderGreenFn) discard errors from `renderer.LoadRulesForPhase("build")` via `rules, _ :=`. This is graceful degradation — prompts render without rules — but silently hides configuration problems. When building closure-based dependency injectors, surface or log errors from fallible setup calls rather than discarding them; silent degradation in prompt assembly can produce subtly wrong LLM outputs that are hard to trace.

### 2026-02-22 | Architectural Migration Safety Checklist | conventions
*Related to: code-review*

When removing public APIs or deleting large files during migration: (1) Systematically find all call sites and complete migrations before deletion — interdependencies between orchestration components require careful refactoring order. (2) Verify all exported symbols are either migrated or unreferenced. (3) Run `go test -tags acceptance -run '^$'` to verify compilation of build-tag-gated tests — these are invisible to normal `go test ./...` and symbols can be silently lost (e.g., PrintStatus was lost when lifecycle.go was deleted). (4) Search for forced-import keep-alive patterns (`var _ = Type{}`) left behind by incomplete refactoring and remove them.


### 2026-02-22 | Builder Pattern Pointer Receiver Mutation | gotchas
*Related to: review-1771784092725425988*

Builder-pattern methods that mutate the pointer receiver (like Gate.WithDecomposer setting g.decomposer = d) work correctly even when the return value is discarded. The return is for optional method chaining; the mutation happens on the receiver regardless. When reviewing builder calls like `obj.WithX(val)` without assignment, check whether the method mutates the receiver — if it does, the call is correct despite looking like a no-op.

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
