---
id: git-auto-push
source_ideas: []
created: 2026-02-07
epic: developer-experience
---

# Git Auto-Push After Bead Completion

## Specification

After each successful bead completion, gromit pushes the current branch to the remote so that collaborators can see progress in real time.

A new `git` config section in `gromit.yaml` controls this behavior:

```yaml
git:
  auto_push: true          # Push to remote after each successful bead (default: true)
  push_failure: "warn"     # What to do when push fails: "warn" | "stop" (default: "warn")
```

**Behavior:**
- When `auto_push` is true, gromit runs `git push` after each successful bead completion (after the bead is closed and `bd sync` runs, but before the between-iterations command).
- When push fails and `push_failure` is `"warn"`, gromit logs a warning and continues to the next bead.
- When push fails and `push_failure` is `"stop"`, gromit stops the loop with an error.
- Gromit pushes to the current branch's upstream tracking ref. If no upstream is configured, it behaves as a push failure (warn or stop based on config).

## Acceptance Criteria

- After a successful bead, `git push` is executed when `auto_push` is true.
- When `auto_push` is false, no push occurs.
- When push fails with `push_failure: "warn"`, the failure is logged and the loop continues.
- When push fails with `push_failure: "stop"`, the loop terminates with an error.
- Defaults are `auto_push: true` and `push_failure: "warn"` when the `git` section is omitted from config.

## Decisions

1. **Push timing: after bd sync, before between-iterations command.** The push should include the committed bead work and the `bd sync` state, but happen before the between-iterations command so that the push status is known before any custom commands run.

2. **Default on.** Most users want collaborators to see progress. Users who don't want auto-push can opt out explicitly.

3. **Warn-and-continue as default failure mode.** A network hiccup shouldn't kill a productive run. Users who need guaranteed remote sync can set `push_failure: "stop"`.

4. **Top-level `git` config section.** This is a new concern distinct from existing config sections. Using a dedicated `git` section keeps it clean and allows future git-related config (e.g., auto-commit settings) to live alongside it.

5. **Push to tracking upstream only.** No `--set-upstream` or branch creation — gromit pushes to wherever the branch already tracks. If there's no upstream, that's a push failure handled by the configured failure mode.

## Research & Context

### Current State

- Git helper functions live in `internal/runner/runner.go` (lines 976-1005): `getGitHead()`, `getGitDiffStat()`, `getGitDiff()`.
- The runner loop in `internal/runner/runner.go` (`Run()` method, lines 167-459) handles bead success at approximately lines 400-450: it calls `bd close`, then `bd sync`, then the between-iterations command, then review.
- Config is defined in `internal/config/config.go` with a `Config` struct and per-section types. Defaults are set in `setDefaults()`.
- The reference config is `gromit.yaml` at the repo root.
- There is no existing git push or remote interaction anywhere in the codebase.
