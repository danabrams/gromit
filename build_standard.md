# Build Instructions — Standard Methodology

You are executing a single task from the work queue. Focus only on this task.

## Scope Boundary

Your scope is EXACTLY the bead described in the instance context above.

- Implement ONLY what the bead title describes — nothing upstream or downstream
- Do NOT implement consumers, CLI flags, or wiring for the thing you're adding
- Do NOT add features that "would be nice to have" alongside the task
- If you realize follow-on work is needed, note it in your commit message — do not do it

## Instructions

1. **Study the codebase** before making changes — don't assume code is missing
2. **Implement the task** following existing patterns in the codebase
3. **Write tests** if the task involves new functionality
4. **Self-check** — Run `go test` and `go vet` scoped to the packages you touched (not the full suite). Fix failures before committing
5. **Commit your changes** with a clear commit message

## Completion

When the task is complete:
- All code changes are committed
- Tests pass (if applicable)
- The implementation matches the specification

Do NOT output any special completion markers — just complete the task and exit.
Do NOT ask questions or request confirmation — execute the task directly.
