# Plan (Cycle 1)

## t-001

Add `out io.Writer` and `store *runstore.Store` fields to `execSpecRun`. Move store construction from `run()` into `RunE` (and test helpers). Update `run()` to use `e.store` instead of a local `store` variable. Set `e.out` to `cmd.OutOrStdout()` in `RunE`.

## t-002

Update all existing tests in exec_test.go and resume_test.go to supply the new `store` and `out` fields on `execSpecRun`. Ensure all existing tests pass without behavior changes.

## t-003

In `run()`, after computing `eventLogPath` and before `stageProvider.BuildStages`, write the start banner to `e.out`: `fmt.Fprintf(e.out, "Run ID: %s\nEvents: %s\n\n", rs.RunID, eventLogPath)`. The banner uses single spaces after colons (distinct from the double-space terminal summary returned by `run()`).

## t-004

Write the test scenario: 'run start prints run ID and events path'. Create a test that calls `r.run(ctx)` directly with `out` set to a `bytes.Buffer`, a stub stage provider returning no stages, and asserts the buffer contains the banner with the run ID and correct events path. Also verify the returned summary string contains the run ID with double-space formatting.

## t-005

Implement `pickSpec` function in spec.go. Signature: `func pickSpec(project, specsDir string, store *runstore.Store, branchResolver func(string) string, in io.Reader, out io.Writer) (string, error)`. Steps: discover specs, list runs, derive status via `DeriveSpecStatusFromContent`, filter to `ready`/`ready_for_review`, print numbered list with `*` and worktree/branch for `ready_for_review` entries, read selection from stdin, return `filepath.Join(specsDir, name+".md")`.

## t-006

Write tests for `pickSpec`: (1) 'spec picker with mixed statuses' scenario — temp specsDir with alpha (ready), beta (ready_for_review with worktree), gamma (completed), delta (running); input '2\n' selects beta; verify output format with worktree/branch lines. (2) 'no eligible specs' scenario — all specs completed/draft; verify output and empty return.

## t-007

Implement `pickRun` function in spec.go. Signature: `func pickRun(project string, store *runstore.Store, in io.Reader, out io.Writer) (string, error)`. Steps: list runs, filter to resumable statuses (StatusRunning, StatusNeedsHuman, StatusBlocked, StatusReadyForReview), sort by StartedAt descending, print numbered list with spec ID, human-readable status label, and timestamp, read selection, return RunID.

## t-008

Write tests for `pickRun`: (1) 'resume picker' scenario — three runs (ready_for_review, needs_human, completed); input '1\n' selects first; verify completed excluded, display format correct. (2) 'resume picker includes blocked and running' scenario — two runs (blocked, running); input '2\n' selects running; verify both appear with correct labels.

## t-009

Restructure `RunE` in exec.go: (1) Remove `MarkFlagRequired("spec")`. (2) Add `--specs-dir` flag. (3) Add `NoOptDefVal = "__pick__"` to the `--resume` flag. (4) Construct store early. (5) Wire picker flow: if resumeRunID=="__pick__" call pickRun; if resumeRunID=="" and specPath=="" call pickSpec (resolving specsDir from project config or --specs-dir flag); guard empty returns. (6) Construct RealStageProvider after pickers. (7) Pass `cmd.InOrStdin()` to pickers, `cmd.OutOrStdout()` to `e.out`. (8) Default branchResolver runs `git -C <path> symbolic-ref --short HEAD`.

## t-010

Replace `TestExecCmd_RequiresSpecFlag` with a test that uses `--specs-dir` pointing to a temp directory and asserts the spec picker is invoked when `--spec` is omitted. Add test for explicit `--resume=run-id` bypassing picker. Ensure all tests pass.

