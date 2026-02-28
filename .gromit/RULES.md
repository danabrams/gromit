# Rules

These are non-negotiable constraints for this project.

## Code Style <!-- phases: red, build, green, refactor, review -->

- Use `go fmt` standard formatting
- Use `error` returns, not panics, for recoverable failures. Exception: test helpers and init()
- After unmarshaling or file loading, call `normalizeNilFields()` to convert nil slices to empty slices. Keep it as a pure nil→empty converter; field mapping belongs in a separate resolution step
- Keep packages focused: one package should not reach into another package's internal types
- Error types used in cross-package `errors.As` checks must be defined once in a shared package; duplicate same-name error structs across packages are forbidden
- `bead.Client`: `Ready()`/`CountReady()` return unblocked beads only (`bd ready`); `List()` returns all open beads (`bd list --status open`)
- Functions that run subprocesses or prompts must inject those dependencies for testability
- JSON tags must be snake_case and explicit (e.g. `input_tokens`, `cost_usd`); use `omitempty` only for truly optional fields
- Agent execution must use `agent.Resolve()` + `agent.Launch()`; never construct `exec.Command` directly
- Spec order: `scope.ValidateSpec(...)` before `scope.ResolveSpec(...)`
- Interface files must include `var _ InterfaceName = (*Impl)(nil)`. Interface changes must include implementation + mock updates in the same bead
- Prompt templates: register `PROMPT_<name>.md` in `runInit()` via `defaultXxxTemplate`. Renderers take context structs returning `(string, error)`. Share common sections; vary only Instructions/Completion.
- Router: choose tier (`low|medium|high`), resolve model (`haiku|sonnet|opus`); store model names in fields like `EscalatedTo`. Lives in `internal/runner/escalation/`
- Mocks use FnField pattern with nil-safe defaults; tests set only needed callbacks
- Per-run accumulator fields (slices, maps) in Runner must be reset at the top of Run(), not in individual phases
- Renderer owns context/state; configure via setters in `NewRunner()`; `BuildContext()` reads it; don't bypass
## Safety <!-- phases: red, build, green, refactor, review -->

- Never commit secrets, API keys, or credentials
- Never delete data without explicit confirmation in the spec
- Pass large strings to Claude CLI via temp files, not CLI arguments (avoids ARG_MAX)
- Shell scripts with user content: use quoted <<'EOF' heredocs; pass dynamic values as arguments, not string interpolation
- Go subprocesses with user-influenced args (bead IDs, refs, branch names) must use `runArgv`, not `runCmd`, to prevent shell/flag injection
- Runtime/local state artifacts (`.dolt/`, `.doltcfg/`, `beads_gromit/`, lock files, `.gromit/state.json`, `.gromit/stats.json`, `.gromit/interactive-state.json`) and timestamped benchmark/report outputs must not be committed. Raw run outputs belong in ignored paths (for example `.gromit/reports/runs/`); only deterministic curated fixtures/artifacts in approved paths (for example `test/fixtures/`, `.gromit/reports/curated/`) may be versioned
- Provider capture fixtures must be stored under test/fixtures/gemini/.
- .gromit/plans/fixtures/ is not an approved deterministic fixture path.

## Test Quality <!-- phases: red, build, green, refactor, review -->

- Acceptance tests must verify behavior through public API/CLI surfaces, not private helpers
- Do not test Go stdlib behavior (`os.MkdirAll`, `os.WriteFile`, `json.Marshal`). Trust stdlib; test your code
- Never use `t.Skip()` — use `//go:build acceptance` for tests requiring external dependencies
- 2+ tests sharing 10+ lines of setup: extract a setup helper. 3+ tests with same structure/different inputs: use table-driven tests
- Prefer compile-time checks or behavioral tests over `os.ReadFile`+`strings.Contains` on `.go` source files — source-reading tests break on refactoring
- `*_acceptance_test.go` files must have `//go:build acceptance` or test end-to-end CLI behavior. Unit tests verifying acceptance criteria go in `*_test.go`
- Acceptance tests (`//go:build acceptance`) have a 6,000-line total budget; prefer unit tests and account for this in specs

## Terminology <!-- phases: build, review, refine, plan -->

- "Backlog" always means the ideas backlog (`gromit add` / `.gromit/backlog.jsonl`), not beads. Beads are work items; backlog items are raw ideas awaiting refinement

## Architecture Guardrails <!-- phases: red, green, refactor -->

- Keep `internal/runner/` packages independent, expose shared contracts via `runtypes/`, enforce production file limits (≤550 lines, facades ≤1000), and reuse shared state/metrics/persistence paths with parity tests during migrations.
- Interactive commands keep a single merge/cleanup owner, use typed conflict classifiers, and never abort pre-existing `MERGE_HEAD`; decomposition entry points call the shared validator with all required fields.

## Architecture <!-- phases: red, build, green, refactor, review -->

- Observability fields (cost/tokens/duration/model/provider/current_run_row attribution) must be produced through the same runtime execution path used in production; any alternate/test path must have parity contract tests proving identical field population semantics and non-empty current-run row generation
- Any bead touching provider stream usage/event handling must add a stream-event matrix contract test (turn/response/result paths) that covers both positive attribution (known model/provider) and negative completeness cases (missing current-run rows fail closed)
- `internal/runner/*/` sub-packages must not import siblings **in production or test files**; cross-cutting types live in `runtypes/`. Parent `runner` package uses type aliases for backward compatibility. Production files: <550 lines; facade files: <1000 lines
- Compatibility/deprecation markers are incomplete unless surfaced in user-visible status/debug output and covered by end-to-end behavior tests
- Interactive commands use the session worktree lifecycle with a single merge/cleanup owner and typed conflict classification from git output + exit status. Do not classify conflicts by message fragments alone. Merge-back cleanup may abort only merge state created by the current operation; pre-existing `MERGE_HEAD` must return a typed non-destructive error.
- All decomposition entry points must call the same shared validator. Required-field rules (non-empty title, expected_outputs contract, dependency-field validity) must not live in call-site-only checks. Any field required by validation must be present in candidate mapping and reprompt context; prompt/schema/fixture changes for those fields must ship together.
- Tracker adapters must not downcast `tracker.Client` via `UnwrapBDAdapter` in production paths. If a capability is needed (e.g., `CreateWithParent`/`ListWithLabel`), add it to tracker interfaces (or a typed sub-interface) and update mocks in the same bead

## Process <!-- phases: build, retro -->

- Post-run efficiency validation must fail closed on missing current-run rows or missing efficiency fields and include per-field diagnostics (missing row vs missing attribution vs missing numeric fields); keep/revert/extend experiment decisions are blocked until at least one complete current-run dataset with non-empty model/provider attribution is recorded and baseline metrics are non-null/non-zero where required
- Retro/experiment Study-Act steps are blocked unless at least one current-run row has non-empty model/provider attribution and non-zero efficiency fields; otherwise emit a data-quality-blocked status
- RecordRetro() must clear one-shot control-limit alert flags in state so previously acknowledged alerts do not persist across subsequent healthy runs

## Build Process <!-- phases: build -->

- Pipeline methods use typed input/output structs; validate deps first; keep pipeline tests next to command files
- Config defaults live in `SetDefaults()`; mirror new `IterationResult` fields into `IterationLog` via `writeIterationLog()`, and add schema-parity contract tests that compare run logs, iteration metrics, and trend inputs so new observability fields cannot be dropped
- Shared-package refactors rerun test suites after commits and verify each diff matches intent
- Test-only bead detection: use `bead.IsTestOnlyBead()` (e.g., "Add tests for") alongside `IsMethodologyActive()`
- On bead failure: add to `skippedBeads`. For broad/high-risk scope (cross-package, umbrella titles, or >=6 touched files), decompose after the first failure; otherwise decompose after 2 consecutive failures. Create/link decomposition sub-beads with expected_outputs and bounded scope. Block parent retries until at least one child lands and is linked; no retries on partial/non-idempotent decomposition; emit decomposition-attempt event; fail if skipped; skip escalation after 3+.
- On timeout, if elapsed time exceeds 75% of budget apply timeout-first decomposition and forbid same-scope retries until decomposition or explicit escalation.
- `Run()` order: validate → execute → persist state → between-iteration hooks → continue. No reordering; log timeout warnings (no early return); nil-safe receiver/config checks at method entry. Persist iteration metrics (including current-run row identity and attribution fields) before any timeout/failure return, and fail validation when comparative metrics would otherwise zero-fill
- New config types/fields: update `gromit.yaml` to match — project-config tests validate the live file against the schema
- Validation recovery auto-fixes (`gofmt`/`goimports`), re-validates, escalates to Claude only if still failing (`MaxValidationRetries` default 1)
- `test/contracts/` contract tests verify git call order (`rev-parse` before `git diff --stat`) and keep harness init and sequencing intact
- Validation commands in gromit.yaml must match the build system (go test/vet/build); never pnpm/npm. API/lifecycle/orchestrator deletions or migrations must add compile gate `go test -tags acceptance -run '^$' ./...`.
- When deleting exported APIs or large orchestration files, run `go test -tags acceptance -run '^$' ./...` as a compile gate before merge; blocked build-tagged references must be resolved in the same bead
- Build phases run `go test`/`go vet` on touched packages only. Trigger full validation when either (a) every N successes OR (b) risk signal fires (cross-package touch, architecture/runner/provider paths, or control-limit build-failure signal)
- `test_touched.sh` tests branch-modified packages; existing failures in those packages block new beads, so verify they pass before starting dependent work
- Benchmark run outputs are ephemeral and must write to ignored artifact paths; committed benchmark/test artifacts must be deterministic curated fixtures (under `test/fixtures/`)
- Validate `token_efficiency.routing` overrides after normalization: unknown categories or non-`low|medium|high` tier values are hard validation errors

## Decomposition <!-- phases: plan -->

- Split beads touching 6+ files across unrelated packages; target 1-3 files per leaf bead. For large facades (1000+ lines), batch 3-5 methods per bead. Decompose 5+ levels deep for narrow scope. Skip single-child decompositions (already atomic)
- Never split natural units: Interface + implementation + mock updates, implementation + tests, companion methods, command flags+wiring, or template+registration
- If scope is too broad, decompose instead of escalating tier. Decomposition must pass output-contract checks (bounded child count, non-empty titles/expected_outputs, no parent-echo, batch_size_min/max). These checks must run in the retry validation loop for all modes (including SkipValidation); silent truncation fallbacks are forbidden. At retry cap, return a contract error and do not proceed with dropped work. If apex tier times out/fails, split into sub-beads with idempotent creation semantics (rollback or deduped re-entry) before any retry
- Pre-split broad work (titles containing infrastructure/E2E/consolidate/extract/shared/refactor/concurrent/conflict, or 3+ new types plus behavior). Require deterministic harness setup before timing-sensitive acceptance assertions. Decomposition orders: test infra (fixtures→helpers→tests), interface extraction (interface→mocks→callers), package extraction (create→move→wire→remove), helper extraction (pure→dependency-heavy→wiring)
- Complexity classification cannot use estimated_files alone; use multi-signal scoring (scope keywords, type count, dependency indicators) or fail validation when required estimate fields are missing. Persist `complexity_source` and signal-count fields into iteration metrics for every run

## Retro Formatting <!-- phases: retro -->

- `LEARNINGS.md` headers: `### YYYY-MM-DD | DESCRIPTIVE_TITLE | CATEGORY_NAME`. Titles must be concise pattern descriptions (not bead IDs or vague words like "final"). Consolidated entries: add `*Related to: id1, id2*` line. Marker-only entries whose body is only `*Consolidated into:*` must be archived immediately and moved to the Archived section
