# Rules

These are non-negotiable constraints for this project. Gromit will always follow these.

## Code Style <!-- phases: build, review -->

- Use `go fmt` standard formatting
- Use `error` return values, not panics, for recoverable failures. Exception: panic is acceptable in test helpers and init() where failure is unrecoverable
- Config struct fields must have sensible zero-value defaults or explicit defaults in `setDefaults()`
- After unmarshaling JSON structs and after file loading, use `normalizeNilFields()` to convert nil slices to empty slices — prevents bugs with nil checks, range operations, and JSON serialization (`nil` → `null` vs `[]` → `[]`)
- Keep packages focused: one package should not reach into another package's internal types
- `bead.Client` semantics: `Ready()`/`CountReady()` are unblocked-only (`bd ready`); `List()` returns all open beads (`bd list --status open`)
- Functions that run subprocesses or prompts must inject those dependencies for testability
- JSON tags must be explicit and snake_case (for example `input_tokens`, `cost_usd`); use `omitempty` only for truly optional fields
- Agent execution must go through `agent.Resolve()` + `agent.Launch()` (with `--agent`/`--choose-agent` overrides), never direct `exec.Command` construction
- Spec handling order is strict: call `scope.ValidateSpec(specsDir, specName)` before `scope.ResolveSpec(specName)`
- Interface files must include compile-time checks (`var _ InterfaceName = (*Impl)(nil)`). Interface signature changes must include implementation + mock/adapter updates in the same bead
- Prompt templates follow load-populate-render: `PROMPT_<name>.md` files are registered in `runInit()` via `defaultXxxTemplate` constants, and renderers take context structs (`Bead`, `ParentBead`) returning `(string, error)`. Reuse common sections (Rules/Learnings/Task/Spec/Parent) and vary only Instructions/Completion using patterns like `ScopeContext`, `DecomposeContext`, and `PrecheckContext`
- Router selection is tier -> model: choose `low|medium|high`, then resolve `haiku|sonnet|opus`. Fields like `SetEscalatedTo` store model names, not tier labels. Keep this in `internal/runner/escalation/` using `SelectTier()`, `SelectModel()`, and `Handler.ExecuteWithRetry`
- Mock implementations use the FnField pattern: optional function pointer fields with nil-safe defaults (returning zero values or no-op behavior). Tests set only the callbacks needed for the specific code path under test. Do not require full mock setup when only one method is being exercised
- State fields in Runner that accumulate per-run (e.g., slices, maps) must be reset at the start of Run(), not in individual phases. This ensures a fresh accumulator within each invocation while allowing safe in-run mutation and non-destructive reads
- Renderer is the central coordinator for context/state setup. Modifications to how data flows into rendering (e.g., character limits, feature flags) happen in NewRunner() by calling Renderer setter methods after initialization; BuildContext() reads those configured values. Do not bypass the Renderer to inject rendering state directly

## Safety <!-- phases: build, review -->

- Never commit secrets, API keys, or credentials
- Never delete data without explicit confirmation in the spec
- When passing large strings to Claude CLI, write to temp files instead of using CLI arguments to avoid exceeding OS ARG_MAX limits
- Shell scripts handling user content must use quoted <<'EOF' heredocs (not unquoted <<EOF) to prevent variable/command expansion, and pass dynamic values via arguments rather than string interpolation to avoid injection

## Test Quality <!-- phases: build, review -->

- Acceptance tests must verify behavior through public API/CLI surfaces, not private helpers
- Do not test Go standard library behavior (that `os.MkdirAll` creates directories, that `os.WriteFile` writes files, that `json.Marshal` produces JSON). Trust stdlib; test your code
- Do not write tests with `t.Skip()` for scenarios that can't run in the test environment. Every committed test must be runnable. If a test needs external dependencies, use `//go:build acceptance` so it runs in the right context
- When two or more tests share 10+ lines of identical setup, extract a shared `setupXxx(t *testing.T)` helper. When three or more tests share the same structure and differ only in inputs/assertions, use a table-driven test
- Files named `*_acceptance_test.go` must either use `//go:build acceptance` or genuinely test end-to-end behavior through the command surface. Unit tests that happen to verify acceptance criteria belong in `*_test.go`
- Acceptance tests (`//go:build acceptance`) have a 6,000-line total budget; prefer unit tests and account for this budget in specs (enforced by `final_verification_test.go`)

## Terminology <!-- phases: build, review, refine, plan -->

- "Backlog" always means the ideas backlog (`gromit add` / `.gromit/backlog.jsonl`), not beads. Beads are work items; backlog items are raw ideas awaiting refinement

## Process <!-- phases: build -->

- Pipeline methods should use input/output structs, validate dependencies first, use renderer processing, resolve agents via `agents.ResolveByName`, and define post-processing change detection. Keep pipeline tests next to their command files
- Always run tests before committing
- Follow existing patterns in the codebase
- When adding config fields, set defaults in `setDefaults()` (numeric sentinel: `0`), update `NormalizeNilFields()` for new nested structs/slices, and test omitted-YAML defaults. Use `json:"field,omitempty"` for optional serialized fields. If `IterationResult` gains fields, mirror them into `IterationLog` in `writeIterationLog()`
- Split beads that touch 6+ files across unrelated packages; target 1-3 files per leaf bead for highest first-pass success. For large facades (1000+ lines), delegate in batches of 3-5 methods per bead. Decompose to 5+ levels deep if needed to achieve narrow leaf scope
- Never split natural units: Interface + implementation + mock updates, implementation + tests, companion methods, command flags+wiring, or template+registration
- If failure analysis says scope is too broad, decompose instead of escalating model tier. If the highest tier times out or fails, split into sub-beads and retry those
- Pre-split likely broad work (titles containing infrastructure/E2E/consolidate/extract/shared/refactor, or 3+ new types plus new behavior). Preferred decomposition orders: test infra (fixtures -> helpers -> tests), interface extraction (interface -> mocks -> callers), package extraction (create package -> move impl -> wire callers -> remove old), and cross-package helper extraction (pure helpers -> dependency-heavy helpers -> caller wiring)
- Shared-package refactors (for example learnings/config) must rerun affected dependent test suites after each commit; verify each diff still matches intent
- Test-only bead detection belongs in `bead.IsTestOnlyBead()` (prefix patterns like "Add tests for"/"Write tests for"), alongside `IsMethodologyActive()`
- When bead operations fail in the run loop, add the bead ID to `skippedBeads` (do not handle inline). If a bead reaches 3+ consecutive cross-run failures (state/JSONL tracked), skip next run and surface for review/decomposition
- `Run()` sequencing is strict: validate -> execute -> persist state -> between-iteration hooks -> continue. Do not reorder or add side effects between persist/hooks. Log timeout warnings instead of early return, and keep nil-safe receiver/config checks at method entry
- `LEARNINGS.md` headers must be `### YYYY-MM-DD | DESCRIPTIVE_TITLE | CATEGORY_NAME` with valid dates, human titles (not bead IDs), and a separate `*Related to: id1, id2*` line for consolidated entries
- When adding new config types or fields, update the actual `gromit.yaml` to match — project-config integration tests validate that the config file parses correctly with the current schema
- Validation recovery should try injected auto-fix (`gofmt`/`goimports`) first, re-validate after each fix, and only escalate to Claude build if auto-fix fails (`MaxValidationRetries`, default 1)
- Contract tests in `test/contracts/` verify git call order during runs; keep harness initialization and expected `git diff --stat` sequencing intact
- Validation commands in gromit.yaml must match the project's build system. Verify against language markers (go.mod, package.json) before running. For this project: use `go test`, `go vet`, `go build` — never pnpm or npm
- During build phases (including TDD), run test/vet only on touched packages for fast feedback. Validation phase runs the full suite (`go test ./...`, `go vet ./...`, `go build ./...`)
