# Learnings

Accumulated operational knowledge from Gromit iterations.
This file is automatically updated. Review periodically with `gromit retro`.

---

## Confirmed

*Patterns seen multiple times - high confidence.*

### 2026-02-24 | Decomposition Contract-Field Parity Across Layers | architecture
*Related to: gromit-jysme, gromit-o9i5v, gromit-btk9n, review-1771835747422178794, gromit-9947, review-1771832540735638835*

Decomposition quality depends on contract parity across validator, mapping, prompt/reprompt context, and telemetry fixtures. Required fields (title, expected_outputs, dependency fields) must be present in candidate mapping and reprompt context shown to the model; prompt/schema/fixture changes for those fields must ship together. Partial adoption creates persistent retry churn, misleading telemetry, and test brittleness.

### 2026-02-24 | Session Worktree and Mergeback Safety Contract | architecture
*Related to: gromit-r7lcc, gromit-1fjzj, gromit-9948, gromit-9949, review-1771880675971102580*

Session worktree and mergeback behavior must follow a single ownership contract: deterministic lifecycle order (create→callback→record pending→merge attempt→cleanup or conflict handoff), typed retryable/non-retryable conflict classification using git output plus exit status, and merge-state safety that never aborts unrelated pre-existing merges. MergeBack cleanup may abort only merge state created by the current operation; pre-existing MERGE_HEAD must return a typed error and preserve the user's in-progress merge state.

### 2026-02-26 | Orchestrator Shared Path and Stage Wiring | architecture
*Related to: code-review, review-1771733992016921570, review-1771855648673321351, review-1771854448297640630, 9980dae8, gromit-22nrv, 8d85f5d7*

Orchestrator policy progression, stage wiring, and cross-cutting metrics/status behavior must stay on one shared execution path with explicit DI; split-stage architecture is fine only when this shared policy path remains authoritative.

*Consolidated from: Orchestrator Cross-Cutting Concerns on Shared Path + Pipeline Stage Dependency Injection and Soft Failure Patterns + Multi-Stage Pipeline Orchestrator Pattern*

### 2026-02-25 | Deterministic Artifact Boundaries | safety
*Related to: review-1772037612174531122, gromit-d7j9*

Deterministic artifact boundaries require one governance policy: block ephemeral/runtime artifacts (`.gromit/state.json`, `.gromit/stats.json`, `.gromit/interactive-state.json`, `.gromit/metrics/*.jsonl`) from git, and require schema-first curated fixtures with provenance for any versioned provider/test artifact. If runtime artifacts become tracked, remove them from the git index with `git rm --cached` and keep `.gitignore` as the source of truth.

*Consolidated from: Runtime State Artifacts Must Stay Untracked + Provider Contract Fixtures*

### 2026-02-07 | Status File Management | patterns
*Related to: nalr, k8c2, kydj, ead1, xpfn, lm34, 2y2d, yj2h, vpyl, kim2*

Status struct fields require backward-compatible changes (omitempty for new optional fields). Use ReadStatus()/IsProcessAlive() for state + liveness checks. Return nil,nil for missing optional files (not an error). StatusWriter handles both lifecycle states and preserves completed iteration count on shutdown. Round-trip tests verify serialization fidelity. Stale resource cleanup integrates into status reporting via process liveness checks. Process utilities (IsProcessAlive) are co-located with Status in status.go. Test file I/O uses t.TempDir() for isolation.

### 2026-02-07 | Methodology Label Activation | patterns
*Related to: ralph-runner-4a3f, ralph-runner-nzue*

Methodologies use label-based activation ("methodology:true"/"false") with global config fallback via bead.IsMethodologyActive(). When active, replace the build prompt with a specialized RenderXXXBuild method. Check parent labels before adding globally-active methodology labels to sub-beads to avoid duplicates. Order methodology checks carefully for precedence when multiple methodologies are active.

### 2026-02-11 | Prompt Template Structure | conventions
*Related to: gromit-rpne*

Prompt templates in .gromit/templates/ use explicit section headers (##) and preserve exact whitespace/structure when updating. Template files follow a consistent structure: context section at top, then Guidelines, then preserved sections like 'Avoiding Sibling Overlap' and ATDD blocks. When modifying sections, maintain blank lines between sections and ensure downstream blocks remain unchanged. Acceptance tests for template changes must match the exact content being added, including specific phrases and subsection structure.

---

## Provisional

*Seen once - may be specific to one task.*

### 2026-02-25 | Scanner-Based Call Log Parsers Need Explicit Buffer Limits | PATTERNS
*Related to: review-1772029155449854699*

`bufio.Scanner` defaults can fail on long CLI invocation lines; call-log utilities should set an explicit scanner buffer and test oversized-line behavior to avoid latent parsing failures in CI logs.

### 2026-02-26 | Shared Path Ownership Includes Telemetry Completeness | architecture
*Related to: code-review, review-1771855648673321351, review-1771854448297640630, gromit-8w81a, gromit-sq2a3, review-1771733992016921570, review-1771880675971102580*

Shared orchestrator/runtime path ownership must include telemetry completeness enforcement (row presence + attribution + numeric fields) as one contract. Runtime-path parity and post-run completeness assertions must fail closed on missing rows/fields and report actionable diagnostics.

*Consolidated from: Telemetry Integrity Contract + Post-Run Completeness Assertions + Orchestrator Shared Path Parity*


## Emerging

*Newly observed — needs validation across more tasks.*

### 2026-02-26 | Tracker Adapter Metadata Serialization Must Use JSON | gotchas
*Related to: code-review, review-1772124256835385050, gromit-qdjqk, review-1772143302280772186*

Tracker adapter metadata must use one canonical JSON encoder (encodeJSONIfNonEmpty) for labels/expected_outputs/criteria across all adapter entry points; fmt.Sprintf/comma formats are forbidden because they break roundtrip parsing.

### 2026-02-26 | Session Worktree Cleanup-Before-Merge Lifecycle | patterns
*Related to: code-review, review-1772124256835385050, review-1772143302280772186*

Session worktree lifecycle should run add → cleanup → merge → remove to avoid checked-out-branch deletion failures, while preserving merge-state safety guarantees.

### 2026-02-26 | Profile-Aware Init Three-Function Pattern | patterns
*Related to: code-review, review-1772124256835385050*

Profile-aware init bootstrap uses three functions: rulesForProfile() for RULES.md content, nextStepsForProfile() for terminal output, seedProfileAwareCommandExamples() for template injection. Profile detection precedence: explicit --profile flag > gromit.yaml config > filesystem signals (go.mod, package.json, pyproject.toml) > "custom" default.

### 2026-02-26 | UnwrapBDAdapter Creates Leaky Abstraction | architecture
*Related to: code-review, review-1772124256835385050, gromit-67j3a*

decomposerAdapter and beadCreatorAdapter use bead.UnwrapBDAdapter() to downcast tracker.Client back to *bead.Client for methods not on the interface (CreateWithParent, ListWithLabel). This defeats the abstraction — non-BD tracker backends will fail at runtime. Either extend tracker.Client or use a tiered interface before adding a second backend.

### 2026-02-23 | Estimate-Only Complexity Scoring Is Fragile | gotchas
*Related to: gromit-fu70d, review-1771835747422178794*

Complexity classification based only on `estimated_files` is easy for model output to underreport or omit. Retain non-estimate signals or enforce strict estimated-files contracts so high-scope decompositions cannot slip through as low risk.

### 2026-02-22 | SPC Display Formatting Two-Tier Pattern | patterns
*Related to: review-1771784092725425988*

SPC (Statistical Process Control) formatting follows a two-tier pattern: formatSPCSummary orchestrates sections (window, control limits, anomalies), while formatSPCLine/formatSPCValue handle individual metric values. simplifySPCMetric provides human-friendly labels for anomaly display (e.g., "rolling_success_rate" → "success"). Keep metric name constants in sync between the logger package (which produces them) and the runner/format package (which displays them) — string-based coupling requires test coverage since there's no compile-time check.

### 2026-02-22 | Three-Layer Requirement Extraction Fallback | patterns
*Related to: review-1771784092725425988*

Requirement extraction uses a 3-layer fallback: Layer 1 (ExpectedOutputs field from bead JSON) → Layer 2 (description parsing with headers, bullets, inline headers, comma-separated lists) → Layer 3 (LLM extraction via haiku). The inline-header ("Functions: X, Y, Z") and comma-list improvements in Layer 2 reduce dependency on the expensive LLM layer. Each layer is independently testable. When adding parsing heuristics, prefer broadening Layer 2 over relying on Layer 3 — deterministic parsing is cheaper and more predictable than LLM extraction.

### 2026-02-22 | Decompose Prompt Expected Outputs as Leverage | conventions
*Related to: review-1771784092725425988*

Including expected_outputs in the decompose prompt template is high-leverage: decomposition quality determines downstream TDD cycle granularity (one red-green cycle per expected output). When the LLM doesn't produce expected_outputs, the system falls back to acceptance_criteria parsing, which may be coarser-grained. Explicitly instructing "list each individual deliverable as a separate entry — these drive TDD RED-GREEN cycles" produces fine-grained outputs that match the system's mechanical needs.

### 2026-02-22 | SCOPE_GATE_DECOMPOSITION_NEEDS_STATE_SAFETY | ARCHITECTURE
*Related to: review-1771797265171555605*

Scope-gate decomposition is resilient to provider failures (falls back to Block), but sequential child creation without idempotency safeguards can leave partial state and duplicate work on retries. Decomposition paths should enforce either rollback or deduped re-entry semantics before parent close.

### 2026-02-22 | DECOMPOSITION_OUTPUT_CONTRACTS_MUST_BE_STRICT | CONVENTIONS
*Related to: review-1771797265171555605*

JSON parsing alone is not enough for LLM decomposition output. Gate paths should validate concrete quality constraints (sub-task count bounds, non-empty titles, bounded expected outputs, no degenerate parent echo) and define deterministic fallback behavior when outputs violate the contract.

### 2026-02-23 | PIPELINE_STAGE_CONFIG_ACCESS_REQUIRES_EXPLICIT_NIL_GUARDS | CONVENTIONS
*Related to: review-1771880675971102580*

Stage code that accepts injected config pointers should fail fast with a typed error before dereferencing config helpers. Mixed nil-tolerant and non-nil-safe calls in the same method create panic paths that tests may miss.

### 2026-02-23 | GATE_DECISION_PATHS_SHOULD_PRESERVE_ROUTING_METADATA | PATTERNS
*Related to: review-1771880675971102580*

When Gate computes complexity routing metadata, all decision outcomes (Proceed/Skip/Block) should propagate that metadata. Returning it only on Proceed creates observability drift and inconsistent iteration logs.

### 2026-02-24 | LEGACY_COMPATIBILITY_MARKERS_REQUIRE_USER-VISIBLE SURFACING | ARCHITECTURE
*Related to: gromit-1xv79, gromit-gr1st, gromit-k0eke*

Adding deprecation-marker fields to compatibility resolution is not sufficient by itself; migration guardrails only work when those markers are surfaced in debug/status output and runtime warnings, with end-to-end tests proving explicit-vs-legacy behavior.

### 2026-02-25 | Thorough Review Rules Must Use Phase Filtering | conventions
When wiring CLI adapters for thorough review prompt rendering, load rules via `LoadRulesForPhase("thorough_review")` instead of `LoadRules()` so build-only sections do not leak into review prompts.

### 2026-02-25 | Record Retro Should Clear Pending Control Alerts | patterns
If run health logic sets a persistent control-limit alert flag in `state.json`, clear that flag as part of `RecordRetro()` so the alert lifecycle is one-shot and does not remain stale after a retro is completed.

### 2026-02-26 | Duplicate Error Types Across Packages Break errors.As | gotchas
*Related to: code-review, review-1772143302280772186*

Identical error types defined in separate packages (e.g. specbranch.ConflictError vs specmerge.ConflictError) cause silent errors.As failures since Go's type system treats them as distinct. Error types used in cross-package errors.As matching must be defined in exactly one package; other packages import that definition.

### 2026-02-26 | Display Package Extraction Must Deduplicate Constants | conventions
*Related to: code-review, review-1772143302280772186*

When extracting display/formatting logic to a new sub-package, ensure metric string constants are defined in one place only. The display package extracted from runner/format.go left duplicate constant sets — changes to metric names must now be updated in two files with no compile-time safety net.

### 2026-02-26 | Dead Code Accumulation in Gemini Provider Stream/Helper Functions | tech_debt
*Related to: code-review, review-1772143302280772186*

parseGeminiStream, extractGeminiAssistantText/Tokens/Cost helpers are defined and tested but not called from any production code path. Scaffolded code without production callers creates maintenance burden and confusion about which parsing path is canonical.

### 2026-02-28 | Centralized DI via NewPipelineDeps Is the Adapter Wiring Pattern | architecture
*Related to: review-1772244209301323387*

All CLI commands wire dependencies through NewPipelineDeps() in cmd/gromit/adapter_deps.go. New commands should use this single entry point rather than constructing adapters inline. Adapters are split: adapters.go (LLM/tracker) and cli_adapters.go (prompt renderers, state, logging).

### 2026-02-28 | TUI Store Uses RWMutex With Copy-on-Read for Thread Safety | patterns
*Related to: review-1772244209301323387*

The TUI store (internal/tui/store.go) uses sync.RWMutex. All mutations hold the write lock; map function reads hold the read lock. View rendering uses copy-on-read to minimize lock duration.

### 2026-02-28 | Process Capacity Gating Before Subprocess Start | patterns
*Related to: review-1772244209301323387*

procutil.WaitForProcessCapacity is called before cmd.Start() in claude, codex, and gemini providers to prevent EAGAIN fork failures under cgroup PID pressure. This is the established pattern for all new subprocess launch sites.

### 2026-02-28 | Epilogue Lifecycle Failure Suppresses Success Signals | patterns
*Related to: review-1772244209301323387*

When close/sync fails in epilogue, the iteration is marked failed, success logging is suppressed, BeadCompleteEvent is not emitted, and spec merge triggering is skipped. This prevents downstream consumers from acting on incomplete state.

---

## Archived

*Moved to LEARNINGS_ARCHIVE.md to reduce prompt context overhead.*
