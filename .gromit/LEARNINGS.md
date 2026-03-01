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

### 2026-02-28 | Retro Worktree Cannot See Runtime Logs, Causing Zero-Data Efficiency Reports | reliability
*Related to: retro-1772302209902158129, gromit-r0x (previous experiment also hit this)*

The retro efficiency report reads `.gromit/logs/run-*.jsonl` for cost/duration/token metrics, but these JSONL files are runtime artifacts (correctly gitignored). When retro runs in a session worktree, `.gromit/logs/` does not exist, so efficiency computation returns $0.0000 for all metrics. This is a recurring bug — the previous experiment ("Periodic Full Validation Gate") also had "data initially invisible due to worktree logs bug." The retro data collector must read logs from the main repo path, not the worktree-local path.

### 2026-02-28 | Coordinator Stuck-State Causes Cascading Integration Queue Failures | reliability
*Related to: 1bb40c0c, review-1772300695650836737, integration-queue conflict entries*

Coordinator bugs in branch checkout sequencing and stuck-state handling can cause concurrent session commits to fail, leaving integration queue entries stuck in `conflict:session_commit_failed` with empty changed_files_hash. Beads still complete (work happens in session worktrees), but integration-to-main fails. The fix requires: (1) deterministic branch checkout with pre-checkout state validation, (2) epilogue signal propagation even on failure, and (3) subprocess safety (process group kill + stderr capture). Retro symptom: cost-per-bead metrics report $0.00 when the retro data collector can't see completed beads through the broken integration path — a data visibility bug, not actual zero output.

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

### 2026-02-28 | All Subprocess Launch Sites Must Follow Full procutil Lifecycle | conventions
*Related to: review-1772244209301323387, review-1772300695650836737, review-1772322141608097349, review-1772366501939692738*
*Consolidated from: Process Capacity Gating Before Subprocess Start + Subprocess Wrappers Must Follow procutil Pattern + Subprocess Launch Must Use Full procutil Pattern Including ReapProcessTree*

All subprocess launch sites must follow the full procutil lifecycle pattern: process-group setup (`SetProcessGroupKill`), capacity gating (`WaitForProcessCapacity`), cancellation descendant kill (`KillDescendantsOnCancel`), stderr capture, and process-tree reap (`ReapProcessTree`, not the shallower `ReapProcessGroup`).

### 2026-02-28 | Epilogue Close/Sync Failures Must Suppress All Success Signals | patterns
*Related to: review-1772244209301323387, review-1772322141608097349*
*Consolidated from: Epilogue Lifecycle Failure Suppresses Success Signals + Epilogue Lifecycle Failure Suppresses All Success Signals*

Epilogue close/sync failures must suppress all success signals (events, logs, merge triggers) and publish a failed lifecycle outcome. When close/sync fails, BeadCompleteEvent is not emitted and spec merge triggering is skipped to prevent downstream consumers from acting on incomplete state.

### 2026-02-28 | Provider Router Requires Mutex for Concurrent Access | patterns
*Related to: review-1772280289214510883*

Provider router (internal/provider/router.go) was a genuine data race — counts and unavailable maps accessed from multiple goroutines without locking. Now uses sync.Mutex on all read/write paths. New provider infrastructure must protect shared state similarly.

### 2026-02-28 | Integration Queue Lifecycle Is Table-Driven via ApplyTransition With Mandatory Persistence | architecture
*Related to: review-1772280289214510883, retro-1772302209902158129, review-1772322141608097349*
*Consolidated from: Integration Queue State Machine Has 7 States With Validated Transitions + Integration Queue State Machine Is Table-Driven via ApplyTransition*

Integration queue lifecycle is table-driven (7 states: draft/ready/integrating/merged/conflict/failed_gates/lane_violation) and must mutate state only through `ApplyTransition`, with every error-path transition persisted before return; direct assignment is forbidden. Error paths (push failure, rebase conflict) must persist state transitions before returning errors to avoid leaving entries stuck in `StateIntegrating`.

### 2026-02-28 | gromit-scfw | patterns
Queue payload validation requires all fields (base_ref, session reference, etc.) to be set when creating records - check integration queue schema and ensure all required fields are initialized in test scenarios and concurrent session workflows

### 2026-02-28 | ListBeads/QueryBeads Silently Return Empty for Unsupported Status | gotchas
*Related to: review-1772300695650836737*

Pipeline.ListBeads and QueryBeads only support status="" or status="ready". Any other status value (e.g., "closed") silently returns an empty result with no error. Callers should be aware of this limitation or the methods should be extended to support additional statuses.

### 2026-02-28 | Vision Metrics Rollups May Use Asymmetric Carve-Out Denominators | patterns
*Related to: review-1772300695650836737, review-1772322141608097349*
*Consolidated from: Vision Metrics Rollup Has Asymmetric Carve-Out Handling + Vision Metrics Rollup Has Intentional Asymmetric Carve-Outs*

Vision metrics rollups may intentionally use asymmetric carve-out denominators (e.g., AcceptedWithoutReworkRate excludes rework_vision_change from denominator but FirstIntegrationPassRate includes them), but each metric must document carve-out policy explicitly.



### 2026-02-28 | Queue/Board Pipeline Methods Must Use NewPipelineDeps, Not Package-Level Factories | tech_debt
*Related to: review-1772322141608097349, review-1772366501939692738*
*Consolidated from: Board and Queue Commands Bypass Deps DI Pattern + Queue and Board Pipeline Methods Bypass Centralized DI Pattern*

Queue/board pipeline methods must consume dependencies through `NewPipelineDeps` (no package-level concrete client factories). Currently board.go and queue.go bypass centralized DI using package-level factory vars — this means these methods cannot be tested with mock dependencies.

### 2026-02-28 | TUI Store Copy-on-Read Returns Shallow Pointer Copies | gotchas
*Related to: review-1772322141608097349*

TUI store uses sync.RWMutex with copy-on-read for thread safety, but copied slices of pointer elements still allow mutation of store data without holding the lock.

### 2026-02-28 | Config Bool Fields With Non-Zero Defaults Must Use Pointer Type | gotchas
*Related to: review-1772322141608097349*

Config types must use *bool for boolean fields with non-zero defaults to distinguish 'unset' from 'explicitly false' in YAML deserialization. Plain bool zero value (false) is indistinguishable from explicit false.

### 2026-02-28 | Session Worktree Uses Enqueue-to-Queue Instead of Direct Merge | architecture
*Related to: review-1772322141608097349*

Session worktree lifecycle now uses enqueue-to-integration-queue instead of direct merge, following the single-writer coordinator pattern.

### 2026-03-01 | Integration Queue Store Lacks File Locking for Concurrent Access | reliability
*Related to: review-1772366501939692738*

Integration queue has no file locking — concurrent CLI processes (sessions + coordinator) can corrupt the queue file via TOCTOU races in the load-mutate-write cycle (Store.Save, SaveQueue, RecoverFromMalformedQueue). Add flock-based advisory locking around the load/modify/write cycle.

### 2026-03-01 | RecoverFromMalformedQueue Never Persists and Uses Invalid Transition | reliability
*Related to: review-1772366501939692738*

RecoverFromMalformedQueue resets integrating entries to StateDraft (not a valid transition from integrating per the transition table) and never calls SaveQueue(), so recovery only exists in memory. Coordinator.RecoverFromCrash correctly uses ApplyTransition to StateReady and persists. The two recovery paths are inconsistent.

### 2026-03-01 | constructor_adapters.go Exceeds 550-Line Limit at 1147 Lines | tech_debt
*Related to: review-1772366501939692738*

internal/runner/constructor_adapters.go is 2x the 550-line file size limit. Contains ~130 lines of dead specGateAdapter code (deprecated per constructor.go comment) and dead childWithDedupeLabelExists method. Primary extraction target for splitting into logical adapter groups.

### 2026-03-01 | Epilogue Stage Mutates Caller's IterationLog Through Input Pointer | architecture
*Related to: review-1772366501939692738*

Epilogue stage sets in.Result.Success = false on lifecycle failure, mutating the caller's data through a pointer. This side-effect violates stage output isolation — the orchestrator should read success status from the epilogue Output, not the mutated Input.

### 2026-03-01 | Provider Router Mutex Creates False Thread Safety in Select() | gotchas
*Related to: review-1772366501939692738*

Provider router Select() has a TOCTOU race between isAvailable() and selectProvider() calls — each acquires/releases the mutex independently, so availability can change between checks. Currently single-threaded in practice, but the lock pattern is misleading.

### 2026-03-01 | specmerge gh_client.go Is Last Site Missing Full procutil Subprocess Pattern | conventions
*Related to: review-1772366501939692738*

specmerge/gh_client.go is missing KillDescendantsOnCancel after cmd.Start() and uses ReapProcessGroup instead of ReapProcessTree. All other subprocess launch sites (cmd_run.go, specbranch/git_ops.go, benchmark/worktree_run.go, preflight.go, integrationqueue_constructor.go) follow the full pattern.


---

## Archived

*Previously archived learnings.*

### 2026-02-28 | gromit-9hw | conventions
When adding regression coverage tests, verify test expectations match actual implementation behavior before running; avoid hardcoded absolute paths in tests—use temp files or environment-specific config paths instead

*Archived from new: filtered: generic engineering advice*

### 2026-02-28 | gromit-p2m | patterns
When implementing CLI client wrappers for external tools (like gh CLI), ensure subprocess calls have proper timeout handling, context cancellation support, and cleanup. Avoid infinite retry loops without exponential backoff or max attempts.

*Archived from new: filtered: generic engineering advice*

### 2026-02-28 | gromit-m0fl | conventions
When implementing a new feature, verify that changes don't inadvertently affect other commands or flows. Run the full test suite before considering a task complete, not just the tests for the new feature. Git status snapshots may not show all modified files—check git diff for the complete picture.

*Archived from new: filtered: generic engineering advice*

### 2026-02-28 | gromit-8l9o | conventions
When implementing features that touch core commands (add, review, run) or orchestration flows (worktree auto-commit), existing tests may break due to changed behavior or side effects. Always run the full test suite to verify no regressions, and understand whether behavior changes are intentional before updating tests.

*Archived from new: filtered: generic engineering advice*

### 2026-02-28 | gromit-zdru | conventions
When refactoring packages like bead/ that are widely depended on, breaking API changes cascade to all callers. Go build failures in multiple packages often indicate a shared dependency was broken. Always verify all call sites after modifying public APIs or method signatures.

*Archived from new: filtered: generic engineering advice*

### 2026-03-01 | gromit-1n3m | conventions
When modifying YAML config files parsed by Go code, changes to field names or structure require corresponding updates to the parsing/validation code and tests. Always run the build immediately after config changes to catch parser/validator mismatches early.

*Archived from new: filtered: generic engineering advice*

### 2026-02-28 | gromit-m0fl | conventions
When adding new CLI commands in cmd/gromit/, avoid modifying shared code paths, test fixtures, or helper functions that other commands depend on. The codebase has common category/context handling patterns that are tested across multiple commands - changes to these affect multiple test suites.

*Archived: 2026-03-01 — task-specific generic caution; not a durable project-specific pattern.*

### 2026-02-28 | gromit-scfw | conventions
When modifying shared test data files or test fixtures (backlog.jsonl, .gromit/integration-queue.json), verify that changes don't break other test suites that depend on those files. Test data changes can have cascading effects across multiple test packages.

*Archived: 2026-03-01 — generic fixture-change caution; duplicates broader existing fixture governance rules.*

### 2026-02-28 | gromit-crt | conventions
Profile selection tests depend on ranking algorithm behavior and global initProfile state. Changes to ranking logic require coordinated updates to test expectations. Tests in init_profile_test.go cannot run in parallel due to global state modification.

*Archived: 2026-03-01 — narrow test-local reminder tied to one ranking/global-state area; low reuse value.*

### 2026-03-01 | gromit-k85o | conventions
Gromit enforces file size limits on critical files like constructor.go (≤550 lines) via TestConstructorFileSizeLimit. When adding code, extract adapter types and related definitions into constructor_adapters.go. This pattern prevents constructor.go bloat and is actively tested.

*Archived: 2026-03-01 — specific to one file-size test and command; already covered by explicit architecture file-size guardrails.*

