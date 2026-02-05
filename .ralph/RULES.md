# Rules

These are non-negotiable constraints for ralph-runner development.

## Code Style

- This is a Go project - use idiomatic Go patterns
- Use `fmt.Errorf("context: %w", err)` for error wrapping
- Keep functions focused and small
- Use descriptive variable names

## Architecture

- CLI commands go in `cmd/ralph/`
- Internal packages go in `internal/`
- Keep packages loosely coupled
- No circular dependencies

## Safety

- Never commit secrets or API keys
- Always handle errors - no silent failures
- Use context for cancellation

## Process

- Run `go build ./cmd/ralph` before committing
- Run `go test ./...` to verify tests pass
- Follow existing patterns in the codebase
- Distinguish environment failures from code failures — missing tools or runtime dependencies are environment issues, not code bugs
