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

### 2026-02-21 | Multi-Stage Pipeline Orchestrator Pattern | patterns
*Related to: 8d85f5d7*

Replace God Object pattern with pure orchestration: hold only stage references and config, no business logic. The Orchestrator struct contains just a config field; all per-stage logic lives in internal/pipeline/<stage>/. Enforce import discipline at the orchestrator level—import only internal/pipeline and internal/logger. Wire stages at construction time via OrchestratorConfig, making dependency graph explicit and mockable. Assign iteration numbers monotonically regardless of outcome (including beads blocked at Gate), preserving failure chains. Flow inter-stage outputs into subsequent iterations: ValidationFailures from Validate→Build Input, TouchedPackages from Epilogue→next iteration Input. Keep failed stages in Epilogue for logging/cleanup rather than early-exit—this ensures consistent logging and status updates. Handle optional stages (Review) via nil checks at runtime, not construction time. Merge global stats atomically at completion, preserving prior entries—use read-modify-write with idempotency checks. Benefit: stages become independently testable, sequencing is explicit and debuggable, and stage coupling is minimal.


## Emerging

*Newly observed — needs validation across more tasks.*

### 2026-02-23 | Skip-Validation Must Not Bypass Decomposition Batch Contracts | conventions
*Related to: gromit-xjeu3, review-1771835747422178794*

Even when `SkipValidation` is true, decomposition output must still fail fast on `batch_size_min`/`batch_size_max` violations. Post-loop truncation fallbacks create hidden behavior drift and can silently drop planned work.

### 2026-02-23 | Shared Validator Coverage Must Include Required-Field Contracts | architecture
*Related to: gromit-btk9n, review-1771835747422178794*

Using one runtime/pipeline decompose validator only prevents drift if required-field rules (for example empty title or missing expected outputs) are also centralized there. Keeping those checks in call sites preserves divergence.

### 2026-02-23 | Estimate-Only Complexity Scoring Is Fragile | gotchas
*Related to: gromit-fu70d, review-1771835747422178794*

Complexity classification based only on `estimated_files` is easy for model output to underreport or omit. Retain non-estimate signals or enforce strict estimated-files contracts so high-scope decompositions cannot slip through as low risk.

### 2026-02-23 | Decomposition Batch-Size Policy Must Be Retry-Enforced, Not Truncated | conventions
*Related to: gromit-9946, review-1771832540735638835*

When decompose output violates batch bounds, enforce `batch_size_min`/`batch_size_max` in the retry validation loop so the model is reprompted with contract feedback. Do not silently truncate oversized output; at retry cap, return a clear contract error instead of proceeding with dropped work.

### 2026-02-23 | Shared Decompose Validator Prevents Rule Drift Across Entry Points | architecture
*Related to: gromit-9947, review-1771832540735638835*

Pipeline decompose and runtime scope-gate decomposition must use one shared validation entry point with explicit mode flags for context-specific rules. Duplicated call-site checks drift over time and create parity bugs between plan-time and runtime behavior.

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

### 2026-02-23 | DECOMPOSE_VALIDATION_RULE_CHANGES_REQUIRE_CONTRACT_PARITY | ARCHITECTURE
*Related to: gromit-jysme, gromit-o9i5v*

When decompose validation rules change (for example, expected_outputs requirements or complexity signal expansion), the prompt contract, fixture payloads, retry-loop behavior, and telemetry expectations must be updated together. Partial adoption creates persistent retry churn, misleading ValidationStats, and test brittleness.

### 2026-02-23 | SESSION_WORKTREE_CONTENTION_HANDLING_NEEDS_EXPLICIT_CONTRACT | ARCHITECTURE
*Related to: gromit-r7lcc, gromit-1fjzj*

Session worktree retry behavior should be driven by an explicit retryable/non-retryable error contract, not ad-hoc message matching alone. Defining the contract first avoids locale/version-sensitive drift in contention detection and keeps retries deterministic.

### 2026-02-23 | REPROMPT_CONTRACT_FIELDS_MUST_BE_VISIBLE_TO_MODEL | ARCHITECTURE
*Related to: gromit-h1s0f*

If reprompt instructions require preserving fields such as `depends_on_index` and `expected_outputs`, those fields must be rendered in the candidate context shown to the model. Telling the model to keep fields unchanged without displaying them creates avoidable contract drift and retry churn.

### 2026-02-23 | REVIEW_LEARNINGS_DEPENDENCY_CONTEXT_PARITY | ARCHITECTURE
*Related to: review-1771839601749019692*

Decomposition/validation contract fields should move together across pipeline mapping and reprompt rendering. Propagating `depends_on_index` into `validate.BeadCandidate` and showing both `depends_on_index` plus `expected_outputs` in reprompt candidate context keeps model repair loops aligned with validator expectations and reduces avoidable retries.

---

## Archived

*Previously archived learnings.*

### 2026-02-21 | Compile-Time Invariant Enforcement vs Dead Code Patterns | conventions
*Related to: f668688fead2f958, 5912b9aee59cce5e, code-review*

Use package-level `var _ Interface = (*Impl)(nil)` declarations in non-test `.go` files to enforce architectural invariants at compile time. A check inside a test function body gates test compilation only — it does not gate production builds. Avoid tests that use `os.ReadFile`+`strings.Contains` on `.go` source files — they break silently on renames/moves. Distinguish from forced-import keep-alive patterns (`var _ = Type{}`): these exist solely to prevent the compiler from removing an unused import, indicate incomplete refactoring, and should be removed. When removing package usage, search for these keep-alive patterns in the same file and related consumers.

*Archived from provisional: filtered: generic engineering advice*
