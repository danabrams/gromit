# Build Instructions — TDD Methodology

You are executing a single task using Test-Driven Development (TDD). Focus only on this task.

## Scope Boundary

Your scope is EXACTLY the bead described in the instance context above.

- Implement ONLY what the bead title describes — nothing upstream or downstream
- Do NOT implement consumers, CLI flags, or wiring for the thing you're adding
- Do NOT add features that "would be nice to have" alongside the task

## Instructions — Red-Green-Refactor Discipline

You MUST follow red-green-refactor strictly. Each cycle is small and committed separately.

### The Cycle (repeat for each requirement)

**1. RED — Write ONE failing test**
- Write a single test function or test case that calls code which doesn't exist yet or doesn't behave correctly yet
- Run tests: they MUST fail (compilation errors count as failing)
- Commit: `red: test for <what the test verifies>`
- Do NOT write any production code in this step

**2. GREEN — Write minimum production code**
- Write only enough production code to make the failing test pass
- Do NOT modify the test you just wrote
- Do NOT add anything beyond what this one test requires
- Run tests: they MUST pass
- Commit: `green: implement <what you added>`

**3. COMMIT and move to next requirement** — refactoring happens in a separate phase

### Non-Negotiable Rules

- ONE test per red step. Stop after writing it.
- MINIMUM code per green step. No "while I'm here" additions.
- SEPARATE commits for red and green. Each commit message starts with `red:` or `green:`.
- Do NOT batch multiple requirements into one cycle.
- Before completing, run `go test` and `go vet` scoped to touched packages (not `./...`). Fix any failures before committing.
- After all requirements are covered, stop — refactoring happens in a separate phase.

## Completion

When complete:
- Multiple small commits exist, alternating `red:` and `green:` prefixes
- All tests pass
- Each requirement has a corresponding test
- No gold plating — minimum viable implementation only

Do NOT output any special completion markers — just complete the task and exit.
Do NOT ask questions or request confirmation — execute the task directly.
