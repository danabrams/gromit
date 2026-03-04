---
id: external-project-support
source_spec: external-project-support
created: 2026-03-04
decomposed: false
---

# External Project Support Implementation Plan

**Goal:** Add a `--project-path` (`-P`) global flag so Gromit can operate on a codebase other than the one where the binary is executed.

**Architecture:** Leverage the existing `os.Chdir()`-based design by modifying `ensureRepoRoot()` to chdir to the flag-specified path (after validation) instead of walking up from cwd. All existing relative path resolution continues to work unchanged.

**Tech Stack:** Go, Cobra CLI

**Spec:** `.gromit/specs/external-project-support.md`

---

## Architecture

**Overview:**
The codebase already anchors all path resolution via `os.Chdir()` in `ensureRepoRoot()`. Adding `--project-path` requires: (1) a new persistent flag, (2) updating `ensureRepoRoot()` to use it, and (3) a `ProjectRoot` field on `Config` for downstream access to the absolute path.

**Key Components:**
1. **`cmd/gromit/main.go`**: New `projectPath` global var and persistent flag registration
2. **`cmd/gromit/repo_root.go`**: Updated `ensureRepoRoot()` and `findProjectRoot()` to respect the flag
3. **`internal/config/config_types.go`**: New `ProjectRoot string` field (tagged `yaml:"-"`)

**Integration Points:**
- `PersistentPreRunE` already calls `ensureRepoRoot()` — no wiring changes
- `loadConfig()` reads `gromit.yaml` from cwd — works after chdir
- All `resolveXXXDir()` functions return relative paths — work after chdir
- `bead.Client` inherits process cwd — works after chdir
- `NewPipelineDeps()` receives `gromitDir` as parameter — works with resolved relative path

**Data Flow:**
```
CLI invocation with --project-path /path/to/project
  → PersistentPreRunE
    → ensureRepoRoot() sees projectPath is set
      → resolves to absolute path
      → validates path contains .gromit/ or gromit.yaml
      → os.Chdir(resolved path)
  → SubcommandRunE (everything works as before, cwd is now target project)
```

**Files to Modify:**
- `cmd/gromit/main.go` — Add `projectPath` global var, register persistent flag, set `cfg.ProjectRoot` after loading config
- `cmd/gromit/repo_root.go` — Update `ensureRepoRoot()` and `findProjectRoot()` to use `projectPath` when non-empty
- `internal/config/config_types.go` — Add `ProjectRoot` field

**Tradeoffs:**
- **Immediate chdir** over explicit path threading: Minimizes changes across the codebase. Spec explicitly calls this out as preferred.
- **Global var** for flag value: Consistent with existing pattern (`configPath`, `maxIterations`, etc. are all globals).

## Test Strategy

**Unit Tests** (`cmd/gromit/repo_root_test.go`):
- `ensureRepoRoot()` with `projectPath` set to a valid project directory → chdir succeeds
- `ensureRepoRoot()` with `projectPath` set to a non-existent path → clear error
- `ensureRepoRoot()` with `projectPath` set to directory lacking `.gromit/` or `gromit.yaml` → clear error
- `findProjectRoot()` respects `projectPath` when set, skips walk-up
- `findProjectRoot()` falls back to normal walk-up when `projectPath` is empty (existing behavior preserved)
- Relative path (`../other-project`) resolved to absolute correctly

**Mocking Strategy:**
- Use `t.TempDir()` with marker files (`.gromit/`, `gromit.yaml`) for valid project dirs
- No mocking needed for `os.Chdir` — use real temp dirs and verify cwd
- Restore original cwd in `t.Cleanup()`

**Coverage Goals:**
- All error paths (missing path, invalid path, no markers) produce actionable error messages
- Happy path chdir works with both absolute and relative paths
- Existing behavior unchanged when flag is empty

## Implementation Tasks

### Task 1: Add ProjectRoot field to Config

**Files:**
- Modify: `internal/config/config_types.go`

**What to Do:**
Add a `ProjectRoot string` field to the `Config` struct, tagged with `yaml:"-"` so it's never serialized/deserialized. This field stores the resolved absolute path to the target project root for downstream consumers.

**Acceptance Criteria:**
- `Config` has a `ProjectRoot string` field tagged `yaml:"-"`
- Existing config loading/saving is unaffected (field is ignored by YAML)

**Dependencies:**
- None

### Task 2: Add --project-path flag and update ensureRepoRoot()

**Files:**
- Modify: `cmd/gromit/main.go`
- Modify: `cmd/gromit/repo_root.go`

**What to Do:**
1. In `main.go`: Add a `projectPath` global var. Register `--project-path` / `-P` as a persistent flag on `rootCmd` in `init()`.
2. In `repo_root.go`: Update `findProjectRoot()` — when `projectPath` is non-empty, resolve it to an absolute path, validate it contains `.gromit/` or `gromit.yaml`, and return it directly (skip the walk-up). Update `ensureRepoRoot()` if needed (it may already work since it calls `findProjectRoot()`).
3. In `main.go`: After `loadConfig()` in subcommand `RunE` functions (or in a helper), set `cfg.ProjectRoot` to the resolved absolute cwd.

**Acceptance Criteria:**
- `gromit --project-path /valid/path status` chdir's to `/valid/path` and runs correctly
- `gromit --project-path /invalid/path status` produces a clear error
- `gromit status` (no flag) continues to work as before
- Relative paths are resolved to absolute before validation

**Dependencies:**
- Task 1 (ProjectRoot field must exist)

**Notes:**
- The `PersistentPreRunE` already calls `ensureRepoRoot()`, so the flag integration is mostly in `findProjectRoot()`.
- Error messages should be specific: "target path does not exist" vs "target path is not a Gromit project (missing .gromit/ or gromit.yaml)".

### Task 3: Unit tests for --project-path behavior

**Files:**
- Modify or Create: `cmd/gromit/repo_root_test.go`

**What to Do:**
Write unit tests covering:
1. `findProjectRoot()` with `projectPath` set to a temp dir containing `gromit.yaml` → returns that dir
2. `findProjectRoot()` with `projectPath` set to a temp dir containing `.gromit/` → returns that dir
3. `findProjectRoot()` with `projectPath` set to a non-existent dir → returns error
4. `findProjectRoot()` with `projectPath` set to a dir with no markers → returns error
5. `findProjectRoot()` with `projectPath` empty → falls back to normal walk-up behavior
6. `ensureRepoRoot()` with `projectPath` set → verifies cwd changes to target
7. Relative path resolution works correctly

**Acceptance Criteria:**
- All test cases pass
- Tests restore original cwd via `t.Cleanup()`
- Tests use `t.TempDir()` for isolation

**Dependencies:**
- Task 2 (implementation must exist to test)

---

## Notes

- The `chdir` approach means all existing subcommands automatically work with `--project-path` without individual changes. This is the key design insight.
- The `bead.Client` doesn't need changes because it inherits process cwd (no `Dir` set in `NewClient()`).
- The `resolveMainRepoLogsDir()` function uses `git rev-parse` which respects cwd, so it also works automatically.
- If future code needs the absolute project root (e.g., for logging or display), it can use `cfg.ProjectRoot`.
