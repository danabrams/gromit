# Rules

These are non-negotiable constraints for this project. Gromit will always follow these.

## Code Style

- Use `go fmt` standard formatting
- Use `error` return values, not panics, for recoverable failures. Exception: panic is acceptable in test helpers and init() where failure is unrecoverable
- Config struct fields must have sensible zero-value defaults or explicit defaults in `setDefaults()`
- Keep packages focused: one package should not reach into another package's internal types
- bead.Client methods have semantic distinctions: Ready()/CountReady() return only unblocked beads (`bd ready`), while List() returns all open beads (`bd list --status open`). Choose the correct method based on whether you need actionable or total counts

## Safety

- Never commit secrets, API keys, or credentials
- Never delete data without explicit confirmation in the spec
- Shell scripts handling user content must use quoted <<'EOF' heredocs (not unquoted <<EOF) to prevent variable/command expansion, and pass dynamic values via arguments rather than string interpolation to avoid injection

## Process

- Always run tests before committing
- Follow existing patterns in the codebase
- When adding config fields, always provide a default value and test that the default is applied when the field is omitted from YAML
- Beads that touch more than 2 files should be split before attempting. If a bead fails and the failure analysis suggests scope was too broad, escalate to splitting rather than retrying at a higher model tier
- Validation commands in gromit.yaml must match the project's build system. Verify against language markers (go.mod, package.json) before running. For this project: use `go test`, `go vet`, `go build` — never pnpm or npm
