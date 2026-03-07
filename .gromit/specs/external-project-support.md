---
id: external-project-support
source_ideas: []
created: 2026-03-04
epic: codebase-health
accepted: true
---

# External Project Support

## Specification

Introduce a `--project-path` (or `-P`) global flag to Gromit that allows it to operate on a codebase other than the one where the binary is being executed. This is a critical "bootstrapping" feature to allow Gromit v1 to build Gromit v2.

## Problem

Gromit is currently "self-aware" and highly coupled to the current working directory. It uses `ensureRepoRoot()` to find and `chdir` to the nearest `.gromit` or `gromit.yaml` marker. This makes it impossible to run Gromit from project A while targeting project B without manual directory switching, and even then, some path resolutions (like logs or worktrees) may still assume the original context.

## Goals

1. Allow targeting any project directory via `--project-path <path>`.
2. Ensure all path resolutions (`.gromit`, `specs/`, `plans/`, `logs/`) are relative to the targeted project root.
3. Direct the `bead.Client` (the tracker) to the target project's `bd` state.
4. Support all core commands (`run`, `status`, `plan`, `decompose`, `refine`, `retro`) in external mode.

## Non-Goals

- Supporting multiple concurrent target projects in a single command invocation.
- Automatic migration of state between projects.

## Design

### 1. Global Flag
Add a persistent flag to `rootCmd`:
```go
rootCmd.PersistentFlags().StringVarP(&projectPath, "project-path", "P", "", "Target project directory (defaults to current directory)")
```

### 2. Config Extension
Add a `ProjectRoot` field to `config.Config` (marked as `yaml:"-"`) to store the resolved absolute path to the target project.

### 3. Root Discovery Refactor
Update `ensureRepoRoot()` in `cmd/gromit/repo_root.go`:
- If `--project-path` is provided, validate that it contains a `.gromit` or `gromit.yaml` marker.
- `chdir` to the resolved target path immediately.
- Update `findProjectRoot()` to respect the provided path if present.

### 4. Dependency Injection
Update `NewPipelineDeps(cfg *config.Config, gromitDir string)` in `cmd/gromit/adapter_deps.go`:
- Ensure `gromitDir` passed here is the resolved path in the target project (e.g., `filepath.Join(cfg.ProjectRoot, ".gromit")`).
- The `bead.Client` should have its `Dir` field set to the target project root so `bd` commands run in the correct context.

### 5. Path Resolution
Refactor `cmd/gromit/resolve.go` to ensure all `resolveXXXDir` functions use the targeted `gromitDir` rather than assuming a local `./.gromit`.

## Acceptance Criteria

- `gromit --project-path ../gromit2 status` correctly lists beads from the `gromit2` project.
- `gromit --project-path ../gromit2 plan <spec>` creates the plan file in `../gromit2/.gromit/plans/`.
- `gromit --project-path ../gromit2 run` processes beads in the target project.
- Logs for external runs are stored in the target project's `.gromit/logs/`.
- Errors are clearly surfaced if the target path is not a valid Gromit project.

## Decisions

1. **Immediate Chdir:** To minimize side effects in the existing codebase, we will `chdir` to the target project root early in `PersistentPreRunE`. This allows many existing relative path assumptions to remain valid while still targeting the correct files.
2. **Explicit Flag:** We prefer an explicit flag over environment variables for clarity in bootstrapping scripts.
