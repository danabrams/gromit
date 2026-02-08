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

## Safety

- Never commit secrets, API keys, or credentials
- Never delete data without explicit confirmation in the spec
- When passing large strings to Claude CLI, write to temp files instead of using CLI arguments to avoid exceeding OS ARG_MAX limits
- Shell scripts handling user content must use quoted <<'EOF' heredocs (not unquoted <<EOF) to prevent variable/command expansion, and pass dynamic values via arguments rather than string interpolation to avoid injection

## Process

- Always run tests before committing
- Follow existing patterns in the codebase
- When adding config fields, always provide a default value in `setDefaults()` and test that the default is applied when the field is omitted from YAML. Use `json:"field,omitempty"` for optional fields in serialized structs to maintain backward compatibility
- Beads that touch more than 2 files should be split before attempting. Cross-cutting refactors (consolidating helpers, extracting interfaces, renaming across files) must be split into per-package or per-concern beads. If a bead fails and the failure analysis suggests scope was too broad, escalate to splitting rather than retrying at a higher model tier. Beads with "infrastructure", "E2E", "consolidate", or "refactor" in the title should be reviewed for scope before execution — test infrastructure tasks should be decomposed into: (1) create test fixtures, (2) create test helpers, (3) write tests using fixtures+helpers. Interface extraction should be decomposed into: (1) define interfaces, (2) create mock implementations, (3) update callers to use interfaces
- When bead operations fail during the run loop, add the bead ID to the skippedBeads map rather than handling errors inline. This centralizes failure handling through the existing stuck-bead detection loop
- LEARNINGS.md entries must follow strict pipe-delimited header format: `### YYYY-MM-DD | DESCRIPTIVE_TITLE | CATEGORY_NAME`. Dates must be valid (not 0001-01-01), BeadID must contain the full learning title (not categories), and consolidated entries must record source IDs in a separate `*Related to: id1, id2*` line in the content body. Validate format before persisting any changes to LEARNINGS.md
- Validation commands in gromit.yaml must match the project's build system. Verify against language markers (go.mod, package.json) before running. For this project: use `go test`, `go vet`, `go build` — never pnpm or npm
