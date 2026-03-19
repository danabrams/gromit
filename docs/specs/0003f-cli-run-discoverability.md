DONE 2026-03-19
# Spec 0003f — CLI Run Discoverability

## spec_id
0003f-cli-run-discoverability

## Vision
Running `gromit-next exec spec` today requires knowing the run ID in advance to resume it, knowing where to find the events log, and knowing the spec path off the top of your head. None of this information is surfaced at the right moment. This spec makes the CLI self-documenting: it tells you the run ID and events path when a run starts, lets you pick a spec from what's available when you don't supply one, shows you which specs are already awaiting review (and where their worktrees are), and lets you pick a paused run to resume rather than hunting for its ID.

## Summary
Four CLI UX improvements to `gromit-next exec spec`: (1) print the run ID and events file path to the terminal when a run starts; (2) when `--spec` is omitted, display a numbered list of executable specs to choose from, marking `ready_for_review` specs with an asterisk and their worktree path and branch; (3) when `--resume` is provided without a run ID, display a numbered list of resumable runs showing spec name, status, and last-run time.

## Goals
### Primary
- Surface the run ID and events log path at run start so they can be found without digging
- Make `--spec` optional with an interactive picker that shows only specs worth running
- Make `--resume` work without a run ID by presenting a picker of resumable runs
- Show worktree path and branch for `ready_for_review` specs so they can be copied to an LLM for review

## Non-goals
- Not changing how spec status is derived
- Not adding fzf or any external picker dependency
- Not adding a picker for `--project` (still required)
- Not persisting picker selections across sessions
- `--spec` is the only flag changing from required to optional; `--resume` remains optional (not required)

## Architecture

All changes are in `cmd/gromit-next/exec.go` and `cmd/gromit-next/spec.go`. No new packages, no new types in the runstore.

### Feature 1: Print run ID and events path at start

`execSpecRun` gains an `out io.Writer` field. The cobra `RunE` sets it to `cmd.OutOrStdout()`; tests set it to a `bytes.Buffer`. The `run()` method signature stays `(string, error)` — the returned string continues to carry the terminal summary (currently `"Run ID:  %s\nStatus:  %s\n"` with double spaces, unchanged) which the caller prints. The start banner is a separate, immediate write to `e.out` inside `run()`, after `eventLogPath := filepath.Join(e.store.RunDir(rs.RunID), "events.jsonl")` is computed and before `e.stageProvider.BuildStages(...)` is called. The `fmt.Fprintf` error is discarded (consistent with CLI stdout writing conventions):

```go
fmt.Fprintf(e.out, "Run ID: %s\nEvents: %s\n\n", rs.RunID, eventLogPath)
```

Note: the banner uses single space after the colon; the existing terminal summary uses double space. Both are intentional.

`RunE` also constructs `store := runstore.NewStore(storeDir)` before calling `pickSpec` or `pickRun` (currently `store` is only constructed inside `run()`). After the move, add `store *runstore.Store` as a field on `execSpecRun`; remove the `store := runstore.NewStore(e.storeDir)` local variable from `run()` and replace all uses of `store` inside `run()` with `e.store` — this includes `store.Get(e.resumeRunID)` in the resume path, `store.RunDir(rs.RunID)` for the event log path, and `store.Save(rs)` at the end.

### Feature 2: Spec picker

`MarkFlagRequired("spec")` is removed; `MarkFlagRequired("project")` is unchanged. When `specPath == ""` after flag parsing in `RunE`, `pickSpec()` is called before constructing `execSpecRun`. `RunE` already resolves `storeDir` (from `--store-dir`, defaulting to `".gromit-next"`) and constructs `store := runstore.NewStore(storeDir)` before calling `pickSpec`. `specsDir` is resolved in `RunE` (not inside `pickSpec`) using the `root workspace.Root` already resolved at the top of `RunE` (`root, _ := resolver.Resolve()` — the existing `RunE` discards this error; follow the same pattern). If root resolution failed silently, `ResolveProjectConfigPath(root, project)` will fail with a meaningful error that is propagated from `RunE`. Resolution chain: `ResolveProjectConfigPath(root, project)` → `LoadProjectConfig(projectDir)` → `cfg.SpecsDir`. `ProjectConfig` has two fields: `SpecsDir string` and `RepoPath string` (both already present in the existing struct). If `cfg.SpecsDir == ""` and `cfg.RepoPath != ""`, fall back to `filepath.Join(cfg.RepoPath, "specs")`; if both are empty, return an error from `RunE` (e.g., `"cannot resolve specs directory: project config has no specs_dir or repo_path"`). Errors from `ResolveProjectConfigPath` or `LoadProjectConfig` are propagated from `RunE`.

`RunE` passes `cmd.InOrStdin()` as the `in io.Reader` to `pickSpec` and `pickRun`.

Add a `--specs-dir` flag to `exec spec` (same as `spec list` already has): `cmd.Flags().String("specs-dir", "", "Override specs directory (for testing)")`. In `RunE`, read it with `specsDir, _ := cmd.Flags().GetString("specs-dir")`. When `specsDir` is non-empty, use it directly and skip `ResolveProjectConfigPath`/`LoadProjectConfig` entirely — do not propagate errors from those calls. This allows tests to supply a temp directory without a real workspace.

```go
func pickSpec(project, specsDir string, store *runstore.Store,
    branchResolver func(worktreePath string) string,
    in io.Reader, out io.Writer) (specPath string, err error)
```

`branchResolver` is passed as a plain function parameter. The default implementation passed by `RunE` runs `git -C <path> symbolic-ref --short HEAD` via `os/exec` and returns `"(unknown)"` on error. Tests pass a stub closure.

**Ordering constraint in `RunE`:** the `RealStageProvider` construction (which takes `SpecPath: specPath`) must occur after the `pickSpec` empty-return guard. If `pickSpec` returns `""`, `RunE` returns nil immediately before constructing the provider. Do not construct the provider with `specPath = ""` and rely on the guard to prevent it from being used — that would silently pass an empty path into `RealStageProvider`.

Steps:
1. Call `DiscoverSpecs(specsDir)` → `([]string, error)`; propagate any error immediately. Note: `DiscoverSpecs` returns bare names without the `.md` extension (e.g., `"0003e-persistent-failure-contract-audit"`, not `"0003e-persistent-failure-contract-audit.md"`)
2. Call `store.List(project)` → `([]*runstore.RunState, error)`; propagate any error immediately; convert the pointer slice to `[]runstore.RunState` with an explicit loop (same pattern as `newSpecListCmd`)
3. For each spec name, filter `allRuns` down to `specRuns` — the subset where `r.SpecID == name` (`RunState.SpecID` is the bare spec name, set via `specIDFromPath` when the run is created, matching exactly what `DiscoverSpecs` returns) — then read content via `os.ReadFile(filepath.Join(specsDir, name+".md"))` (errors are silently ignored; empty content is passed, which is treated as non-draft by `DeriveSpecStatusFromContent`), and call `DeriveSpecStatusFromContent(name, specRuns, string(content))` — note the `string()` cast, as `os.ReadFile` returns `[]byte`
4. Filter using a whitelist: keep specs whose derived status is exactly `"ready"` or `"ready_for_review"`; discard everything else
5. For each `ready_for_review` spec, find the run in `specRuns` where `r.Status == runstore.StatusReadyForReview` and `r.StartedAt` is latest (most recent); get its `WorktreePath` (`RunState` has a `WorktreePath string` field), call `branchResolver(worktreePath)` to get the branch name
6. Print numbered list to `out`:
   ```
   1. 0003d-repeated-failure-escalation
   2. 0003e-persistent-failure-contract-audit * (ready_for_review)
        worktree: /Users/foo/gromit/.worktrees/spec-0003e
        branch:   feature/spec-0003e
   ```
7. If no eligible specs exist, print `"no specs available to run\n"` to `out` and return `("", nil)` — the caller treats an empty string return as a no-op and returns nil from `RunE` without running a spec
8. Read a line from `in`, parse as a 1-based integer `n` (1 ≤ n ≤ len(eligible)); both non-integer strings and out-of-range integers return an error immediately without re-prompting
9. Let `selectedName` be the bare spec name at index `n-1` in the eligible list. Return `filepath.Join(specsDir, selectedName+".md")`

### Feature 3: Run picker for `--resume`

After the flag is registered (`cmd.Flags().String("resume", "", "...")`), add:

```go
cmd.Flags().Lookup("resume").NoOptDefVal = "__pick__"
```

This makes `--resume` (no value) set the flag to `"__pick__"`, while `--resume=run-abc123` sets it to `"run-abc123"`. Run IDs are generated as `"run-"` followed by 16 hex characters (8 random bytes hex-encoded, e.g., `"run-45dcbc628184bad1"`), so `"__pick__"` cannot collide with a valid run ID. The sentinel is checked in `RunE` before constructing `execSpecRun`.

**Mutual exclusivity with `--spec`:** when `resumeRunID == "__pick__"`, `pickRun` is called and its return value replaces `resumeRunID` — explicitly assign `resumeRunID = pickedRunID` before constructing `execSpecRun`, so that `e.resumeRunID` holds a real run ID (not the sentinel `"__pick__"`). If `store.Get("__pick__")` were ever called, it would produce a confusing error; ensuring the sentinel never reaches `execSpecRun` prevents this. `pickSpec` is skipped entirely regardless of whether `--spec` was supplied. A resumed run carries its `SpecID` in the stored `RunState`; there is no need to resolve a spec path. `pickSpec` is only called when `resumeRunID` is empty (no `--resume` flag at all) and `specPath` is empty.

**`RunE` restructuring:** the existing `if p == nil` block that constructs `RealStageProvider` must move to after all picker calls and their empty-return guards. The required order in `RunE` is:
1. Resolve flags (`specPath`, `resumeRunID`, `storeDir`, `project`, etc.)
2. Construct `store := runstore.NewStore(storeDir)`
3. If `resumeRunID == "__pick__"`: call `pickRun`; on empty return, return nil; else assign `resumeRunID`
4. If `resumeRunID == ""` and `specPath == ""`: call `pickSpec`; on empty return, return nil; else assign `specPath`
5. Construct `RealStageProvider` with the now-resolved `specPath`
6. Construct `execSpecRun` with `store` and `out` fields set

**`SpecPath` on resume:** when resuming (any non-empty `resumeRunID` after picker or direct flag), `specPath` may be empty. This is safe because `filterStagesForResume` removes the `compile` stage before execution, so `passthruCompiler.Compile()` — which reads `SpecPath` — is never called on resume. Pass `specPath` as-is (possibly `""`) to `RealStageProvider`; do not attempt to reconstruct it from `rs.SpecID`. **Known accepted risk:** if a future stage other than `compile` reads `SpecPath`, it will receive `""`. This is a deliberate tradeoff — the alternative (reconstructing the path from `SpecID`) is deferred to a future spec if needed.

**Ordering constraint for `pickRun` empty return:** same as `pickSpec` — if `pickRun` returns `""`, `RunE` returns nil immediately, before constructing `RealStageProvider`.

```go
func pickRun(project string, store *runstore.Store,
    in io.Reader, out io.Writer) (runID string, err error)
```

Steps:
1. Call `store.List(project)` → `([]*runstore.RunState, error)`; propagate any error immediately. The function may work directly with `[]*runstore.RunState` (dereferencing as needed) or convert to `[]runstore.RunState` — either is acceptable since the runstore is not modified
2. Filter to runs where `rs.Status` is one of the raw `RunState` status constants: `runstore.StatusRunning`, `runstore.StatusNeedsHuman`, `runstore.StatusBlocked`, `runstore.StatusReadyForReview` (note: `"needs_human"` is the stored constant; `"needs_attention"` is only a derived display string from `DeriveSpecStatus` and must not be used here)
3. Sort by `StartedAt` descending
4. Print numbered list to `out`, displaying spec ID, a human-readable label for the status, and timestamp formatted as `"2006-01-02 15:04:05"`:
   ```
   1. 0003e-persistent-failure-contract-audit   ready_for_review   2026-03-18 10:42:01
   2. 0003d-repeated-failure-escalation          needs_attention    2026-03-17 15:10:33
   ```
   Display labels for raw status constants (do not call `DeriveSpecStatus` or `DeriveSpecStatusFromContent` here — use a local switch on `rs.Status`): `StatusRunning` → `"running"`, `StatusReadyForReview` → `"ready_for_review"`, `StatusBlocked` → `"blocked"`, `StatusNeedsHuman` → `"needs_attention"`
5. If no eligible runs exist, print `"no runs available to resume\n"` to `out` and return `("", nil)` — the caller treats an empty string return as a no-op and returns nil from `RunE` without resuming
6. Read a line from `in`, parse as 1-based integer `n` (1 ≤ n ≤ len(eligible)); both non-integer strings and out-of-range integers return an error immediately without re-prompting
7. Return the selected run's `RunID`

## Acceptance Criteria

1. When a run starts, the run ID and events file path are printed to `e.out` after `rs.RunID` is known and before `stageProvider.BuildStages` is called
2. When `--spec` is omitted, the command does not error; instead it prints a numbered list of specs with derived status `ready` or `ready_for_review` and reads a selection from stdin
3. Specs with derived status `completed`, `draft`, `running`, or `needs_attention` do not appear in the spec picker
4. `ready_for_review` specs in the picker are marked with `*` and include the worktree path and branch on indented lines below; branch is resolved via an injectable `branchResolver` (default: `git -C <path> symbolic-ref --short HEAD`)
5. When no eligible specs exist, the command prints `"no specs available to run\n"` and exits with code 0
6. When `--resume` is passed without a value (cobra sets the flag to the sentinel `"__pick__"`), the command prints a numbered list of resumable runs filtered on raw `RunState.Status` values (`StatusRunning`, `StatusNeedsHuman`, `StatusBlocked`, `StatusReadyForReview`) and reads a selection from stdin
7. The resume picker displays spec ID, a human-readable status label, and last-run timestamp formatted as `"2006-01-02 15:04:05"`, sorted most-recent first
8. When `--resume=<run-id>` is passed with an explicit run ID, no picker is shown and existing resume behavior is unchanged
9. The `out io.Writer` on `execSpecRun` and the `in`/`out` parameters on `pickSpec`/`pickRun` are injectable, allowing tests to drive input via a `strings.Reader` and capture output via a `bytes.Buffer`
10. All existing `exec spec` tests continue to pass after updating them for the new `store` and `out` fields on `execSpecRun`; the existing `TestExecCmd_RequiresSpecFlag` test must be replaced with a test that uses `--specs-dir` pointing to a temp directory and asserts the spec picker is invoked when `--spec` is omitted

## Scenarios

### Scenario: run start prints run ID and events path
**Given:** An `execSpecRun` with `storeDir` pointing to a temp directory, `store = runstore.NewStore(storeDir)`, a stub `stageProvider` that does nothing (returns no stages), `out` set to a `bytes.Buffer`, `specPath = "testdata/my-spec.md"`, and `projectID = "gromit"`
**When:** `r.run(ctx)` is called directly in the test (not via cobra)
**Then:** `run()` returns `(summary, nil)` — `summary` is the terminal string returned to the caller (printed by `RunE` in production, but here it is just a return value). `e.out` (the buffer) receives the banner written directly inside `run()` before stages execute. These are distinct destinations: the test reads both the return value and the buffer separately. Extract `<runID>` from `summary` (the return value) by splitting on `"Run ID:  "` (two spaces) and taking the first token of the remainder (e.g., `strings.Fields(strings.SplitN(summary, "Run ID:  ", 2)[1])[0]`). Then assert that the buffer (not the summary) begins with:
```
Run ID: <runID>
Events: <storeDir>/runs/<runID>/events.jsonl

```
The banner appears before any stage output (trivially satisfied when the stub provider returns no stages).


### Scenario: spec picker with mixed statuses
**Given:** A temp `specsDir` with four `.md` files: `alpha.md` (no runs → `ready`), `beta.md` (one `RunState` with `SpecID = "beta"`, `Status = StatusReadyForReview`, `WorktreePath = "/tmp/wt"`, `StartedAt = 2026-03-18T10:00:00Z`), `gamma.md` (one `RunState` with `SpecID = "gamma"`, `Status = StatusCompleted`, `StartedAt = 2026-03-18T09:00:00Z`), `delta.md` (one `RunState` with `SpecID = "delta"`, `Status = StatusRunning`, `StartedAt = 2026-03-18T08:00:00Z`). A stub `branchResolver` that returns `"feature/foo"` for `/tmp/wt`. All `RunState` objects have `ProjectID = "gromit"`; the store is populated with them before `pickSpec` is called.
**When:** `pickSpec` is called with `in = strings.NewReader("2\n")` and `out = bytes.Buffer`
**Then:** The buffer contains exactly two numbered entries: `1. alpha` and `2. beta * (ready_for_review)` with worktree `/tmp/wt` and branch `feature/foo`. `gamma` and `delta` do not appear. The return value is `filepath.Join(specsDir, "beta.md")`.

### Scenario: spec picker — no eligible specs
**Given:** A `specsDir` where all specs have derived status `completed` or `draft`
**When:** `pickSpec` is called
**Then:** `out` contains `"no specs available to run\n"`. The function returns `("", nil)`. The `RunE` caller treats the empty string as a signal to return nil without executing a run.

### Scenario: resume picker
**Given:** Three runs in the store for project `"gromit"`:
- Run A: `SpecID = "spec-e"`, `Status = StatusReadyForReview`, `StartedAt = 2026-03-18T10:00:00Z`
- Run B: `SpecID = "spec-d"`, `Status = StatusNeedsHuman`, `StartedAt = 2026-03-18T09:00:00Z`
- Run C: `SpecID = "spec-c"`, `Status = StatusCompleted`, `StartedAt = 2026-03-18T08:00:00Z`

**When:** `pickRun` is called with `in = strings.NewReader("1\n")` and `out = bytes.Buffer`
**Then:** The buffer lists exactly two entries. Entry 1 contains `"spec-e"`, `"ready_for_review"`, and `"2026-03-18 10:00:00"`. Entry 2 contains `"spec-d"`, `"needs_attention"`, and `"2026-03-18 09:00:00"`. Run C is excluded. The function returns run A's `RunID`.

### Scenario: resume picker includes blocked and running runs
**Given:** Two runs in the store for project `"gromit"`:
- Run D: `SpecID = "spec-f"`, `Status = StatusBlocked`, `StartedAt = 2026-03-18T11:00:00Z`
- Run E: `SpecID = "spec-g"`, `Status = StatusRunning`, `StartedAt = 2026-03-18T10:30:00Z`

**When:** `pickRun` is called with `in = strings.NewReader("2\n")` and `out = bytes.Buffer`
**Then:** Both entries appear. Entry 1 is run D with label `"blocked"` and timestamp `"2026-03-18 11:00:00"`. Entry 2 is run E with label `"running"` and timestamp `"2026-03-18 10:30:00"`. The function returns run E's `RunID`.

### Scenario: explicit resume ID bypasses picker
**Given:** A run with ID `run-abc1234567890abc` exists in the store (16 hex chars after the `run-` prefix)
**When:** `exec spec --project gromit --resume=run-abc1234567890abc` is run
**Then:** No picker is shown; the run is resumed directly, replicating existing behavior

## Validation
```
go test ./cmd/gromit-next/... -count=1 -timeout 60s
go vet ./cmd/gromit-next/...
go test ./internal/next/runstore/... -count=1 -timeout 60s
```
