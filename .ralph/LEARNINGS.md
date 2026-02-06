# Learnings

Accumulated operational knowledge from Ralph iterations.
This file is automatically updated. Review periodically with `ralph retro`.

---

## Confirmed

*Patterns seen multiple times - high confidence.*

### 2026-02-06 | gotchas | consolidated from r3h, ehn, 0o2, rz1
Validation commands in ralph.yaml must match the project's actual tech stack. Always verify configuration against language-specific markers (go.mod, package.json, requirements.txt). For Go projects, use `go test ./...`, `go vet ./...`, and `go build ./...` — never Node.js tools like pnpm. **Promoted to rule in RULES.md Process section.**

---

## Provisional

*Seen once - may be specific to one task.*

*No provisional learnings.*

---

## Archived

*No longer relevant or superseded.*

### 2026-02-06 | ralph-runner-ehn | gotchas
Archived: duplicate of consolidated validation commands learning. Subsumed by promoted rule.

### 2026-02-06 | ralph-runner-0o2 | gotchas
Archived: duplicate of consolidated validation commands learning. Subsumed by promoted rule.

### 2026-02-06 | ralph-runner-rz1 | gotchas
Archived: duplicate of consolidated validation commands learning. Subsumed by promoted rule.

