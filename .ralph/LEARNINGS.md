# Learnings

Accumulated operational knowledge from Ralph iterations.
This file is automatically updated. Review periodically with `ralph retro`.

---

## Confirmed

*Patterns seen multiple times - high confidence.*

*No confirmed learnings yet.*

---

## Provisional

*Seen once - may be specific to one task.*

### 2026-02-06 | ralph-runner-r3h | gotchas
Validation commands in ralph.yaml must match the project's technology stack. For Go projects, use 'go test' and 'go fmt'/'golangci-lint' instead of Node.js tools. Always verify the validation config against the actual build system.

### 2026-02-06 | ralph-runner-ehn | gotchas
For Go projects, validation commands should use go test, go build, and golangci-lint instead of pnpm. Verify the validation commands in ralph.yaml match the project's actual build system before running validation.

---

## Archived

*No longer relevant or superseded.*

*No archived learnings.*

