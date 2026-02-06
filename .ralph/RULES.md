# Rules

These are non-negotiable constraints for this project. Ralph will always follow these.

## Code Style

- Use `go fmt` standard formatting
- Use `error` return values, not panics, for recoverable failures
- Config struct fields must have sensible zero-value defaults or explicit defaults in `setDefaults()`
- Keep packages focused: one package should not reach into another package's internal types

## Safety

- Never commit secrets, API keys, or credentials
- Never delete data without explicit confirmation in the spec

## Process

- Always run tests before committing
- Follow existing patterns in the codebase
- When adding config fields, always provide a default value and test that the default is applied when the field is omitted from YAML
- Beads that touch more than 2 files should be split before attempting
- Validation commands in ralph.yaml must match the project's build system. Verify against language markers (go.mod, package.json) before running. For this project: use `go test`, `go vet`, `go build` — never pnpm or npm
