# Rules

These are non-negotiable constraints for this project. Gromit will always follow these.

## Code Style

- Use `go fmt` standard formatting
- Use `error` return values, not panics, for recoverable failures. Exception: panic is acceptable in test helpers and init() where failure is unrecoverable
- Config struct fields must have sensible zero-value defaults or explicit defaults in `setDefaults()`
- After unmarshaling JSON structs and after file loading, use `normalizeNilFields()` to convert nil slices to empty slices — prevents bugs with nil checks, range operations, and JSON serialization (`nil` → `null` vs `[]` → `[]`)
- Keep packages focused: one package should not reach into another package's internal types
- bead.Client methods have semantic distinctions: Ready()/CountReady() return only unblocked beads (`bd ready`), while List() returns all open beads (`bd list --status open`). Choose the correct method based on whether you need actionable or total counts
- Functions that call subprocesses or prompt users should accept these dependencies as injected function parameters, not call them directly. This enables testing with simple mocks rather than requiring stdin/stdout management or actual subprocess execution
- JSON struct tags use snake_case field names (e.g., `input_tokens`, `cost_usd`). All serialized fields must have explicit JSON tags — omitting tags causes fields to be excluded from output. Use `omitempty` only for optional fields; omit it when the field should always be present in output
- Agent selection and execution uses `agent.Resolve()` + `agent.Launch()` — never construct `exec.Command` with Claude binary/flags directly. `agent.Resolve()` handles priority-based selection (flag override > interactive picker > phase config > default), and CLI commands expose `--agent` and `--choose-agent` flags for override
- Interface files must include compile-time satisfaction checks (`var _ InterfaceName = (*Impl)(nil)`) at the top of the file to catch implementation drift at compile time rather than runtime — see internal/runner/interfaces.go for the pattern
- Prompt templates follow the load-populate-render pattern: (1) Templates are named PROMPT_<name>.md, registered in runInit()'s templates map, with constants named defaultXxxTemplate in init.go. (2) Renderer methods accept a context struct (with Bead and ParentBead fields) and return (string, error). (3) Template variants reuse common context sections (Rules, Learnings, Task, Spec, Parent) and customize only Instructions and Completion sections. (4) New context types and render methods should follow existing patterns like ScopeContext, DecomposeContext, PrecheckContext

## Safety

- Never commit secrets, API keys, or credentials
- Never delete data without explicit confirmation in the spec
- When passing large strings to Claude CLI, write to temp files instead of using CLI arguments to avoid exceeding OS ARG_MAX limits
- Shell scripts handling user content must use quoted <<'EOF' heredocs (not unquoted <<EOF) to prevent variable/command expansion, and pass dynamic values via arguments rather than string interpolation to avoid injection

## Test Quality

- Acceptance tests must test behavior through the public API or command surface — never call private/internal helper functions directly. If a test calls a private function and asserts on its return value, it's a unit test regardless of what the filename says
- Do not test Go standard library behavior (that `os.MkdirAll` creates directories, that `os.WriteFile` writes files, that `json.Marshal` produces JSON). Trust stdlib; test your code
- Do not write tests with `t.Skip()` for scenarios that can't run in the test environment. Every committed test must be runnable. If a test needs external dependencies, use `//go:build acceptance` so it runs in the right context
- When two or more tests share 10+ lines of identical setup, extract a shared `setupXxx(t *testing.T)` helper. When three or more tests share the same structure and differ only in inputs/assertions, use a table-driven test
- Files named `*_acceptance_test.go` must either use `//go:build acceptance` or genuinely test end-to-end behavior through the command surface. Unit tests that happen to verify acceptance criteria belong in `*_test.go`

## Process

- Always run tests before committing
- Follow existing patterns in the codebase
- When adding config fields, always provide a default value in `setDefaults()` and test that the default is applied when the field is omitted from YAML. Zero is the sentinel for 'not configured' for numeric fields — use `if field == 0` checks in setDefaults(). Nested structs need their own defaults set in the parent's setDefaults() after the struct is populated. Use `json:"field,omitempty"` for optional fields in serialized structs to maintain backward compatibility
- Beads that touch 6+ files across unrelated packages should be split before attempting. Cross-cutting refactors (consolidating helpers, extracting interfaces, renaming across files) must be split into per-package or per-concern beads. If a bead fails and the failure analysis suggests scope was too broad, escalate to splitting rather than retrying at a higher model tier. Beads with "infrastructure", "E2E", "consolidate", or "refactor" in the title should be reviewed for scope before execution — test infrastructure tasks should be decomposed into: (1) create test fixtures, (2) create test helpers, (3) write tests using fixtures+helpers. Interface extraction should be decomposed into: (1) define interfaces, (2) create mock implementations, (3) update callers to use interfaces. Refactoring beads that modify shared packages (e.g., learnings, config) must verify all dependent test suites pass after each commit — changes cascade to tests across the codebase. Review each commit's actual diff against intent; formatting or style commits can accidentally delete functional code. Never split Interface + implementation + mock updates, implementation + tests, companion methods in same package, command flags+wiring, or template+registration — these are single units of work
- Test-only beads are detected by `bead.IsTestOnlyBead()` using title prefix patterns (e.g., "Add tests for", "Write tests for"). This heuristic lives in the bead package alongside `IsMethodologyActive()`. When adding new test-only detection patterns, add them as prefix matches in `IsTestOnlyBead()` rather than creating separate detection mechanisms
- When bead operations fail during the run loop, add the bead ID to the skippedBeads map rather than handling errors inline. This centralizes failure handling through the existing stuck-bead detection loop
- LEARNINGS.md entries must follow strict pipe-delimited header format: `### YYYY-MM-DD | DESCRIPTIVE_TITLE | CATEGORY_NAME`. Dates must be valid (not 0001-01-01), BeadID must contain the full learning title (not categories), and consolidated entries must record source IDs in a separate `*Related to: id1, id2*` line in the content body. Validate format before persisting any changes to LEARNINGS.md
- When adding new config types or fields, update the actual `gromit.yaml` to match — project-config integration tests validate that the config file parses correctly with the current schema
- Validation commands in gromit.yaml must match the project's build system. Verify against language markers (go.mod, package.json) before running. For this project: use `go test`, `go vet`, `go build` — never pnpm or npm
