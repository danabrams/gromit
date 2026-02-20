# Rules

These are non-negotiable constraints for this project.

## Code Style <!-- phases: build, review -->

- Use `go fmt` standard formatting
- Use `error` returns, not panics, for recoverable failures. Exception: panics allowed in test helpers and init() where failure is fatal
- Config fields must have zero-value defaults or explicit defaults in `setDefaults()`
- After unmarshaling or file loading, call `normalizeNilFields()` to convert nil slices to empty slices (prevents nil-check bugs and `null` vs `[]` JSON). Keep `normalizeNilFields()` as a pure nil→empty converter; field mapping (e.g., AcceptanceCriteria→ExpectedOutputs) belongs in a separate resolution step
- Keep packages focused: one package should not reach into another package's internal types
- `bead.Client`: `Ready()`/`CountReady()` return unblocked beads only (`bd ready`); `List()` returns all open beads (`bd list --status open`)
- Functions that run subprocesses or prompts must inject those dependencies for testability
- JSON tags must be snake_case and explicit (e.g. `input_tokens`, `cost_usd`); use `omitempty` only for truly optional fields
- Agent execution must use `agent.Resolve()` + `agent.Launch()` (with `--agent`/`--choose-agent` overrides); never construct `exec.Command` directly
- Spec order: `scope.ValidateSpec(specsDir, specName)` before `scope.ResolveSpec(specName)`
- Interface files must include `var _ InterfaceName = (*Impl)(nil)`. Interface changes must include implementation + mock/adapter updates in the same bead
- Prompt templates: `PROMPT_<name>.md` files registered in `runInit()` via `defaultXxxTemplate` constants; renderers take context structs (`Bead`, `ParentBead`) returning `(string, error)`. Share Rules/Learnings/Task/Spec/Parent sections; vary only Instructions/Completion using `ScopeContext`, `DecomposeContext`, `PrecheckContext`
- Router: choose tier (`low|medium|high`), resolve model (`haiku|sonnet|opus`). Store model names (not tier labels) in fields like `SetEscalatedTo`. Live in `internal/runner/escalation/` using `SelectTier()`, `SelectModel()`, `Handler.ExecuteWithRetry`
- Mocks use FnField pattern: optional function-pointer fields with nil-safe defaults. Tests set only the callbacks needed for the code path under test; don't require full mock setup for a single method
- Per-run accumulator fields (slices, maps) in Runner must be reset at the top of Run(), not in individual phases
- Renderer owns context/state setup. Configure it in NewRunner() via setters; BuildContext() reads those values. Never bypass Renderer state

## Safety <!-- phases: build, review -->

- Never commit secrets, API keys, or credentials
- Never delete data without explicit confirmation in the spec
- Pass large strings to Claude CLI via temp files, not CLI arguments (avoids ARG_MAX)
- Shell scripts with user content: use quoted <<'EOF' heredocs; pass dynamic values as arguments, not string interpolation
- Go subprocesses with user-influenced args (bead IDs, refs, branch names) must use `runArgv`, not `runCmd`, to prevent shell/flag injection

## Test Quality <!-- phases: build, review -->

- Acceptance tests must verify behavior through public API/CLI surfaces, not private helpers
- Do not test Go stdlib behavior (`os.MkdirAll`, `os.WriteFile`, `json.Marshal`). Trust stdlib; test your code
- Never use `t.Skip()` for scenarios that can't run; every committed test must be runnable. Use `//go:build acceptance` for tests requiring external dependencies
- 2+ tests sharing 10+ lines of setup: extract `setupXxx(t *testing.T)` helper. 3+ tests with same structure/different inputs: use table-driven test
- Prefer compile-time checks (`var _ Interface = (*Impl)(nil)`) or behavioral tests over `os.ReadFile`+`strings.Contains` on `.go` source files for structural verification — source-reading tests break on refactoring
- `*_acceptance_test.go` files must have `//go:build acceptance` or test end-to-end CLI behavior. Unit tests verifying acceptance criteria go in `*_test.go`
- Acceptance tests (`//go:build acceptance`) have a 6,000-line total budget; prefer unit tests and account for this in specs

## Terminology <!-- phases: build, review, refine, plan -->

- "Backlog" always means the ideas backlog (`gromit add` / `.gromit/backlog.jsonl`), not beads. Beads are work items; backlog items are raw ideas awaiting refinement

## Architecture <!-- phases: build, review -->

- `internal/runner/*/` sub-packages must not import siblings; cross-cutting types live in `runtypes/`. Parent `runner` package uses type aliases for backward compatibility. Production files: <550 lines; facade files: <1000 lines

## Build Process <!-- phases: build -->

- Pipeline methods: typed input/output structs, validate deps first, renderer processing, resolve agents via `agent.Resolve()`, post-processing change detection. Keep pipeline tests next to command files
- Always run tests before committing
- Follow existing patterns in the codebase
- Config fields: defaults in `setDefaults()` (sentinel `0`; `*int`/`-1` when zero is valid), `NormalizeNilFields()` for nested structs/slices, test omitted-YAML defaults, `json:"field,omitempty"` for optional. Mirror new `IterationResult` fields into `IterationLog` in `writeIterationLog()`
- Shared-package refactors (e.g., learnings/config): rerun all affected dependent test suites after each commit; verify each diff still matches intent
- Test-only bead detection: use `bead.IsTestOnlyBead()` (e.g., "Add tests for") alongside `IsMethodologyActive()`
- On bead failure: add to `skippedBeads`, not inline. After 3+ cross-run consecutive failures (JSONL-tracked via `MaxCrossRunFailures`), skip and surface for review/decomposition
- `Run()` order: validate → execute → persist state → between-iteration hooks → continue. No reordering; log timeout warnings (not early return); nil-safe receiver/config checks at method entry
- New config types/fields: update `gromit.yaml` to match — project-config tests validate the live file against the schema
- Validation recovery: auto-fix (`gofmt`/`goimports`) first, re-validate, then Claude escalation only if still failing (`MaxValidationRetries`, default 1)
- `test/contracts/` contract tests verify git call order (rev-parse before `git diff --stat`); keep harness init and sequencing intact
- Validation commands in gromit.yaml must match the build system (check go.mod/package.json). For this project: `go test`, `go vet`, `go build` only — never pnpm/npm
- Build phases: run test/vet on touched packages only. Full validation: `go test ./...`, `go vet ./...`, `go build ./...`
- `test_touched.sh` tests all branch-modified packages. Pre-existing failures in touched packages block new beads — verify target packages pass before starting dependent work

## Decomposition <!-- phases: plan -->

- Split beads touching 6+ files across unrelated packages; target 1-3 files per leaf bead. For large facades (1000+ lines), batch 3-5 methods per bead. Decompose 5+ levels deep for narrow scope. Skip single-child decompositions (already atomic)
- Never split natural units: Interface + implementation + mock updates, implementation + tests, companion methods, command flags+wiring, or template+registration
- If scope too broad, decompose instead of escalating tier. If apex tier times out or fails, split into sub-beads and retry
- Pre-split likely broad work (titles containing infrastructure/E2E/consolidate/extract/shared/refactor, or 3+ new types plus new behavior). Preferred decomposition orders: test infra (fixtures -> helpers -> tests), interface extraction (interface -> mocks -> callers), package extraction (create package -> move impl -> wire callers -> remove old), and cross-package helper extraction (pure helpers -> dependency-heavy helpers -> caller wiring)

## Retro Formatting <!-- phases: retro -->

- `LEARNINGS.md` headers must be `### YYYY-MM-DD | DESCRIPTIVE_TITLE | CATEGORY_NAME` with valid dates, human titles (not bead IDs), and a separate `*Related to: id1, id2*` line for consolidated entries. Titles like "final" or bead IDs (e.g., "gromit-qth8i") are rejected — use a concise phrase describing the pattern (e.g., "Touched-Package Validation Gotcha")
