---
created: 2026-02-07T00:00:00Z
decomposed: true
decomposed_at: "2026-02-07T22:34:44-05:00"
id: git-auto-push
source_spec: git-auto-push
---

# Git Auto-Push Implementation Plan

**Goal:** Automatically push to the remote after each successful bead completion so collaborators can see progress in real time.

**Architecture:** Add a `GitConfig` section to gromit's config and a `runGitAutoPush()` method on Runner that pushes to the current branch's upstream tracking ref after each successful bead, with configurable failure behavior (warn or stop).

**Tech Stack:** Go, exec.Command for git operations

**Spec:** `.gromit/specs/git-auto-push.md`

---

## Architecture

**Overview:**
A new `git:` config section controls auto-push behavior. A new `runGitAutoPush()` Runner method executes `git push` after bead success, slotted between status.json write and the between-iterations command.

**Key Components:**
1. **`GitConfig` struct** (`internal/config/config.go`): Config section with `AutoPush *bool` (default true) and `PushFailure string` (default "warn"), plus `IsAutoPushEnabled()` helper.
2. **`runGitAutoPush()` method** (`internal/runner/runner.go`): Runner method that checks config, runs `git push`, and handles failure per the configured mode.

**Integration Points:**
- `Config` struct gets a `Git GitConfig` field with `yaml:"git"` tag
- `setDefaults()` sets auto_push=true and push_failure="warn"
- `Run()` loop calls `runGitAutoPush()` before `runBetweenIterationsCommand()` in the success path

**Data Flow:**
1. Config loads `git:` section → `setDefaults()` fills defaults
2. On bead success: bd close → bd sync → status.json write → **git push** → between-iterations command → reviews
3. `runGitAutoPush()` checks `IsAutoPushEnabled()` → if false, returns nil
4. Runs `git push` → on failure: warn mode logs and returns nil; stop mode returns error

**Files to Modify:**
- `internal/config/config.go` — Add GitConfig struct, field, defaults, helper
- `internal/runner/runner.go` — Add runGitAutoPush() method, wire into success path
- `gromit.yaml` — Add git: section to reference config

**Tradeoffs:**
- No separate `internal/git/` package — single method doesn't warrant a new package; follows existing pattern of git helpers in runner.go
- `*bool` for AutoPush — required to distinguish "user set false" from "unset" since default is true
- No `--set-upstream` fallback — per spec, push to tracking upstream only; no branch creation on remote

---

## Test Strategy

**Unit Tests** (Option B — test logic paths, trust exec pattern):

Config tests (`internal/config/config_test.go`):
- Defaults set correctly when git: section omitted
- IsAutoPushEnabled() returns true when nil, true when true, false when false
- YAML with explicit git: section parses correctly

Runner tests (`internal/runner/runner_test.go`):
- runGitAutoPush() returns nil on nil config (nil-safe)
- runGitAutoPush() returns nil when auto_push is false
- Failure-mode branching: warn logs and continues, stop returns error

**Mocking Strategy:**
- Config: temp YAML files with config.Load(), no mocks
- Runner: test config gating and failure branching; actual git push exec follows established pattern

---

## Implementation Tasks

### Task 1: Add GitConfig to config package

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**What to Do:**
Add a `GitConfig` struct with `AutoPush *bool` (`yaml:"auto_push"`) and `PushFailure string` (`yaml:"push_failure"`) fields. Add a `Git GitConfig` field to the `Config` struct with `yaml:"git"` tag. In `setDefaults()`, set AutoPush to true (via `*bool` pattern matching PrecheckConfig.Enabled) and PushFailure to "warn". Add an `IsAutoPushEnabled()` method on `GitConfig` following the `PrecheckConfig.IsEnabled()` pattern. Add table-driven tests for defaults, YAML parsing, and the helper method.

**Acceptance Criteria:**
- `GitConfig` struct exists with correct yaml tags and `IsAutoPushEnabled()` helper
- When `git:` section is omitted from YAML, defaults are auto_push=true, push_failure="warn"
- When `auto_push: false` is set in YAML, `IsAutoPushEnabled()` returns false

**Dependencies:** None

### Task 2: Add runGitAutoPush to runner and wire into loop

**Files:**
- Modify: `internal/runner/runner.go`
- Test: `internal/runner/runner_test.go`

**What to Do:**
Add a `runGitAutoPush() error` method on Runner following the `runBetweenIterationsCommand()` pattern. Start with nil-safe check (`r == nil || r.cfg == nil`). Check `r.cfg.Git.IsAutoPushEnabled()` — return nil if false. Log "Pushing to remote..." before executing. Run `git push` via `exec.Command("git", "push")`, piping stdout/stderr to `r.output`. On failure: if `r.cfg.Git.PushFailure` is "stop", return `fmt.Errorf("git push failed: %w", err)`; otherwise log `"Warning: git push failed: %v"` and return nil. Wire the call into the success path of `Run()` between the status.json write and `runBetweenIterationsCommand()`, with `if err := r.runGitAutoPush(); err != nil { return err }`.

**Acceptance Criteria:**
- `runGitAutoPush()` is nil-safe and returns nil when auto_push is false
- Push failure with "warn" logs warning and returns nil; with "stop" returns an error
- The call is placed after bd sync / status.json write and before between-iterations command

**Dependencies:** Task 1

### Task 3: Update reference config

**Files:**
- Modify: `gromit.yaml`

**What to Do:**
Add a `git:` section to the reference config file with `auto_push: true` and `push_failure: "warn"`, with comments explaining each field. Follow the commenting style of existing sections in the file.

**Acceptance Criteria:**
- `gromit.yaml` contains a documented `git:` section with both fields and explanatory comments

**Dependencies:** Task 1

---

## Notes

- The push uses plain `git push` with no arguments — this pushes to the current branch's configured upstream tracking ref. If no upstream is set, git exits with an error, which is handled as a push failure per the configured mode.
- The `runGitAutoPush()` method is placed near `runBetweenIterationsCommand()` in runner.go since they follow the same pattern.
- No changes needed to `normalizeNilFields()` since GitConfig has no slice or map fields.
