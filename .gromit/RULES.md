# Rules

These are non-negotiable constraints for this project.

## Code Style <!-- phases: build, review -->

- Use `go fmt` standard formatting
- Use `error` returns, not panics, for recoverable failures. Exception: test helpers and init()
- After unmarshaling or file loading, call `normalizeNilFields()` to convert nil slices to empty slices. Keep it as a pure nil→empty converter; field mapping belongs in a separate resolution step
- Keep packages focused: one package should not reach into another package's internal types
- `bead.Client`: `Ready()`/`CountReady()` return unblocked beads only (`bd ready`); `List()` returns all open beads (`bd list --status open`)
- Functions that run subprocesses or prompts must inject those dependencies for testability
- JSON tags must be snake_case and explicit (e.g. `input_tokens`, `cost_usd`); use `omitempty` only for truly optional fields
- Agent execution must use `agent.Resolve()` + `agent.Launch()`; never construct `exec.Command` directly
- Spec order: `scope.ValidateSpec(specsDir, specName)` before `scope.ResolveSpec(specName)`
- Interface files must include `var _ InterfaceName = (*Impl)(nil)`. Interface changes must include implementation + mock updates in the same bead
- Prompt templates: `PROMPT_<name>.md` files registered in `runInit()` via `defaultXxxTemplate` constants; renderers take context structs returning `(string, error)`. Share common sections; vary only Instructions/Completion per context type. Never ignore loader errors (`LoadRulesForPhase`, template reads); propagate or emit structured warnings
- Router: choose tier (`low|medium|high`), resolve model (`haiku|sonnet|opus`). Store model names (not tier labels) in fields like `EscalatedTo`. Lives in `internal/runner/escalation/`
- Mocks use FnField pattern: optional function-pointer fields with nil-safe defaults. Tests set only the callbacks needed for the code path under test; don't require full mock setup for a single method
- Per-run accumulator fields (slices, maps) in Runner must be reset at the top of Run(), not in individual phases
- Renderer owns context/state setup. Configure it in NewRunner() via setters; BuildContext() reads those values. Never bypass Renderer state
- Do not discard errors from renderer/template/rules loaders (no `_, _ :=` for fallible setup). Propagate the error or log a structured warning with phase and file context

## Safety <!-- phases: build, review -->

- Never commit secrets, API keys, or credentials
- Never delete data without explicit confirmation in the spec
- Pass large strings to Claude CLI via temp files, not CLI arguments (avoids ARG_MAX)
- Shell scripts with user content: use quoted <<'EOF' heredocs; pass dynamic values as arguments, not string interpolation
- Go subprocesses with user-influenced args (bead IDs, refs, branch names) must use `runArgv`, not `runCmd`, to prevent shell/flag injection

## Test Quality <!-- phases: build, review -->

- Acceptance tests must verify behavior through public API/CLI surfaces, not private helpers
- Do not test Go stdlib behavior (`os.MkdirAll`, `os.WriteFile`, `json.Marshal`). Trust stdlib; test your code
- Never use `t.Skip()` — use `//go:build acceptance` for tests requiring external dependencies
- 2+ tests sharing 10+ lines of setup: extract a setup helper. 3+ tests with same structure/different inputs: use table-driven tests
- Prefer compile-time checks or behavioral tests over `os.ReadFile`+`strings.Contains` on `.go` source files — source-reading tests break on refactoring
- `*_acceptance_test.go` files must have `//go:build acceptance` or test end-to-end CLI behavior. Unit tests verifying acceptance criteria go in `*_test.go`
- Acceptance tests (`//go:build acceptance`) have a 6,000-line total budget; prefer unit tests and account for this in specs

## Terminology <!-- phases: build, review, refine, plan -->

- "Backlog" always means the ideas backlog (`gromit add` / `.gromit/backlog.jsonl`), not beads. Beads are work items; backlog items are raw ideas awaiting refinement

## Architecture <!-- phases: build, review -->

- `internal/runner/*/` sub-packages must not import siblings **in production or test files**; cross-cutting types live in `runtypes/`. Parent `runner` package uses type aliases for backward compatibility. Production files: <550 lines; facade files: <1000 lines
- Interactive commands use the session worktree lifecycle: package-level launcher fn var, session command const, `cfg.Worktree.IsEnabled()` opt-out, `sessionConflictSettingsFromConfig`, `runWithSessionWorktreeWithConflictSettings`. Lifecycle: create worktree → callback → record pending branch → merge attempt → cleanup or conflict handoff
- During orchestrator migrations, cross-cutting concerns (state persistence, cost/token metrics, status updates) must be implemented in one shared path, with parity tests if legacy and new paths coexist

## Build Process <!-- phases: build -->

- Pipeline methods: typed input/output structs, validate deps first, renderer processing, post-processing change detection. Keep pipeline tests next to command files
- Config fields: defaults in `SetDefaults()` (use `*int`/`-1` when zero is valid), test omitted-YAML defaults. Mirror new `IterationResult` fields into `IterationLog` in `writeIterationLog()`
- Shared-package refactors (e.g., learnings/config): rerun all affected dependent test suites after each commit; verify each diff still matches intent
- Test-only bead detection: use `bead.IsTestOnlyBead()` (e.g., "Add tests for") alongside `IsMethodologyActive()`
- On bead failure: add to `skippedBeads`, not inline. After 2 consecutive cross-run failures, automatically create/link decomposition sub-beads and block further retries of the parent until at least one sub-bead lands. Keep 3+ threshold for final skip escalation
- `Run()` order: validate → execute → persist state → between-iteration hooks → continue. No reordering; log timeout warnings (not early return); nil-safe receiver/config checks at method entry. Iteration metrics (duration/cost/tokens) must be persisted before any timeout/failure return path and verified by completeness tests
- New config types/fields: update `gromit.yaml` to match — project-config tests validate the live file against the schema
- Validation recovery: auto-fix (`gofmt`/`goimports`) first, re-validate, then Claude escalation only if still failing (`MaxValidationRetries`, default 1)
- `test/contracts/` contract tests verify git call order (rev-parse before `git diff --stat`); keep harness init and sequencing intact
- Validation commands in gromit.yaml must match the build system (check go.mod/package.json). For this project: `go test`, `go vet`, `go build` only — never pnpm/npm. For API deletions/migrations touching exported symbols or lifecycle/orchestrator files, add compile gate: `go test -tags acceptance -run '^$' ./...`
- Usage accounting must use explicit before/after snapshots for every phase (red/green/refactor/validate) and a single merge strategy for provider stream events. Mixing raw totals and deltas in one run is forbidden
- When deleting exported APIs or large orchestration files, run `go test -tags acceptance -run '^$' ./...` as a compile gate before merge; blocked build-tagged references must be resolved in the same bead
- Build phases: run test/vet on touched packages only. Full validation: `go test ./...`, `go vet ./...`, `go build ./...`
- `test_touched.sh` tests all branch-modified packages. Pre-existing failures in touched packages block new beads — verify target packages pass before starting dependent work

## Decomposition <!-- phases: plan -->

- Split beads touching 6+ files across unrelated packages; target 1-3 files per leaf bead. For large facades (1000+ lines), batch 3-5 methods per bead. Decompose 5+ levels deep for narrow scope. Skip single-child decompositions (already atomic)
- Never split natural units: Interface + implementation + mock updates, implementation + tests, companion methods, command flags+wiring, or template+registration
- If scope too broad, decompose instead of escalating tier. If apex tier times out or fails, split into sub-beads and retry
- Pre-split broad work (titles containing infrastructure/E2E/consolidate/extract/shared/refactor, or 3+ new types plus behavior). Decomposition orders: test infra (fixtures→helpers→tests), interface extraction (interface→mocks→callers), package extraction (create→move→wire→remove), helper extraction (pure→dependency-heavy→wiring)

## Retro Formatting <!-- phases: retro -->

- `LEARNINGS.md` headers: `### YYYY-MM-DD | DESCRIPTIVE_TITLE | CATEGORY_NAME`. Titles must be concise pattern descriptions (not bead IDs or vague words like "final"). Consolidated entries: add `*Related to: id1, id2*` line
