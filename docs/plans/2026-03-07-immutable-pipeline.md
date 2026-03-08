# Immutable Pipeline Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Wire the already-built pipeline components (StageCommitter, FileSubscriber, SquashPerBead) into the v2 loop so every stage produces a structured git commit with a cumulative event log, and add the `gromit debug2` command.

**Architecture:** All core components exist: `pipeline/commit_message.go`, `pipeline/stage_committer.go`, `pipeline/squash.go`, `event/file_subscriber.go`, and git adapter methods (Log, Show, SquashCommits). The work is wiring these into `BeadLoop`, `SpecLoop`, and `run2_components.go`, then adding integration tests and the debug command. The Present stage already has `WithSquasher`.

**Tech Stack:** Go 1.24, git CLI, cobra (CLI framework), JSONL event log

---

## Architecture

### Components Already Built
- `internal/v2/pipeline/commit_message.go` — `FormatCommitMessage` / `ParseCommitMessage`
- `internal/v2/pipeline/stage_committer.go` — `StageCommitter.CommitStage(ctx, worktree, beadID, stageName, iteration, decision)`
- `internal/v2/pipeline/squash.go` — `SquashPerBead(ctx, git, worktree, beads)`
- `internal/v2/event/file_subscriber.go` — `FileSubscriber` writes JSONL to a path
- `internal/v2/adapter/git/exec_git_adapter.go` — `Log`, `Show`, `SquashCommits` implemented
- `internal/v2/stage/present/present.go` — `WithSquasher` option exists
- `internal/v2/testutil/fake_git.go` — Full fake with all methods

### Wiring Needed
1. **BeadLoop**: Add `StageCommitter` field, call `CommitStage` after each stage (build, validate, review), replace legacy `commitBeadWork`
2. **SpecLoop**: Add `StageCommitter` field, call `CommitStage` after spec-level stages (plan, decompose, accept, present)
3. **SpecLoop**: Wire `FileSubscriber` to typed emitter after checkout, targeting `.gromit/v2/events.jsonl` in worktree
4. **run2_components.go**: Create `StageCommitter`, pass to `BeadLoop` and `SpecLoop`, wire squasher into Present stage
5. **FakeGit**: Add `StatusOutput` field so tests can simulate dirty worktrees
6. Integration tests for commit history and branch lifecycle
7. `cmd/gromit/debug2.go` — new CLI command

### Data Flow
```
Stage runs → events emitted → FileSubscriber appends to .gromit/v2/events.jsonl
          → StageCommitter.CommitStage() creates structured commit (includes events.jsonl)
          → On present: SquashPerBead squashes stage commits into per-bead commits
          → On success: worktree removed (branch deleted)
          → On failure: worktree preserved for debug
```

## Test Strategy

- **Unit tests**: FakeGit `StatusOutput` field, StageCommitter calls in BeadLoop/SpecLoop via fakes
- **Integration tests**: Real git repo, fake stages, verify commit history via `git log`, verify event log contents, verify squash behavior, verify branch lifecycle (preserve on failure, delete on success)
- **Debug command tests**: Prepared worktree with known event log and commit history

---

## Implementation Tasks

### Task 1: Add StatusOutput to FakeGit

**Files:**
- Modify: `internal/v2/testutil/fake_git.go`

**Step 1: Write the failing test**

Add to `internal/v2/testutil/fake_git_test.go`:

```go
func TestFakeGit_StatusOutput(t *testing.T) {
	fg := NewFakeGit()
	fg.StatusOutput = "M  file.go\n"
	got, err := fg.Status(context.Background(), "/tmp/wt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "M  file.go\n" {
		t.Errorf("got %q, want %q", got, "M  file.go\n")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/v2/testutil/ -run TestFakeGit_StatusOutput -v`
Expected: FAIL — `StatusOutput` field does not exist

**Step 3: Write minimal implementation**

In `fake_git.go`, add `StatusOutput string` field to `FakeGit` struct and return it from `Status()`:

```go
// In FakeGit struct, add:
StatusOutput string

// In Status method, change return to:
return f.StatusOutput, nil
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/v2/testutil/ -run TestFakeGit_StatusOutput -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/v2/testutil/fake_git.go internal/v2/testutil/fake_git_test.go
git commit -m "feat: add StatusOutput field to FakeGit for dirty-worktree simulation"
```

---

### Task 2: Add StageCommitter to BeadLoop

**Files:**
- Modify: `internal/v2/loop/bead_loop.go`
- Test: `internal/v2/loop/bead_loop_test.go`

**Step 1: Write the failing test**

Add a test that verifies StageCommitter is called after each stage in the pipeline. Use FakeGit with `StatusOutput` set to simulate dirty worktree, and a `StageCommitter` backed by that FakeGit:

```go
func TestBeadLoop_StageCommitter_CommitsAfterEachStage(t *testing.T) {
	fg := testutil.NewFakeGit()
	fg.StatusOutput = "M  file.go\n"
	committer := &pipeline.StageCommitter{Git: fg}

	loop, err := NewBeadLoop(BeadLoopConfig{
		Gate:           &fakeStage{name: "gate", decision: stage.DecisionProceed},
		Build:          &fakeStage{name: "build", decision: stage.DecisionProceed},
		Validate:       &fakeStage{name: "validate", decision: stage.DecisionProceed},
		Review:         &fakeStage{name: "review", decision: stage.DecisionProceed},
		Epilogue:       &fakeStage{name: "epilogue", decision: stage.DecisionProceed},
		Git:            fg,
		StageCommitter: committer,
	})
	if err != nil {
		t.Fatal(err)
	}
	loop.SetWorktree("/tmp/wt")
	beads := []*bead.Bead{{ID: "b1", Title: "test bead"}}
	_, err = loop.Run(context.Background(), beads, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect structured commits for build, validate, review (not gate/epilogue per spec)
	wantMessages := []string{
		"[bead:b1/build/iter:1] Proceed",
		"[bead:b1/validate/iter:1] Proceed",
		"[bead:b1/review/iter:1] Proceed",
	}
	if len(fg.CommitMessages) < len(wantMessages) {
		t.Fatalf("got %d commits, want at least %d: %v", len(fg.CommitMessages), len(wantMessages), fg.CommitMessages)
	}
	for i, want := range wantMessages {
		if fg.CommitMessages[i] != want {
			t.Errorf("commit[%d] = %q, want %q", i, fg.CommitMessages[i], want)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/v2/loop/ -run TestBeadLoop_StageCommitter_CommitsAfterEachStage -v`
Expected: FAIL — `StageCommitter` field does not exist on `BeadLoopConfig`

**Step 3: Write minimal implementation**

In `bead_loop.go`:

1. Add import for `pipeline` package and add `StageCommitter` field to `BeadLoopConfig` and `BeadLoop`:
```go
// In BeadLoopConfig:
StageCommitter *pipeline.StageCommitter

// In BeadLoop:
stageCommitter *pipeline.StageCommitter
```

2. Wire in `NewBeadLoop`:
```go
stageCommitter: config.StageCommitter,
```

3. Add a `commitStage` helper method:
```go
func (b *BeadLoop) commitStage(ctx context.Context, beadID, stageName string, iteration int, decision string) {
	if b.stageCommitter == nil {
		return
	}
	if err := b.stageCommitter.CommitStage(ctx, b.worktree, beadID, stageName, iteration, decision); err != nil {
		log.Printf("stage commit %s/%s: %v", beadID, stageName, err)
	}
}
```

4. Call `commitStage` in `runStageEntry` after the `!failed` success branch (before returning nil):
```go
b.commitStage(ctx, beadItem.ID, stageName, iteration, "Proceed")
```

5. Call `commitStage` after failed stages too (before retry logic):
```go
b.commitStage(ctx, beadItem.ID, stageName, iteration, "Fail")
```

6. Remove the legacy `commitBeadWork` call from `processBead` (line 284). The per-stage commits replace it.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/v2/loop/ -run TestBeadLoop_StageCommitter -v`
Expected: PASS

**Step 5: Verify no regressions**

Run: `go test ./internal/v2/loop/... -v`
Expected: All tests pass. Existing tests that don't set `StageCommitter` get nil (no-op) behavior.

**Step 6: Commit**

```bash
git add internal/v2/loop/bead_loop.go internal/v2/loop/bead_loop_test.go
git commit -m "feat: wire StageCommitter into BeadLoop for per-stage commits"
```

**Dependencies:** Task 1

---

### Task 3: Add Decision.String() method

**Files:**
- Modify: `internal/v2/stage/stage.go`
- Test: `internal/v2/stage/stage_test.go`

Check first whether `Decision` already has a `String()` method. If so, skip this task. If not:

**Step 1: Write the failing test**

```go
func TestDecision_String(t *testing.T) {
	tests := []struct {
		d    Decision
		want string
	}{
		{DecisionProceed, "Proceed"},
		{DecisionSkip, "Skip"},
		{DecisionBlock, "Block"},
		{DecisionFail, "Fail"},
		{Decision(99), "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.d.String(); got != tt.want {
			t.Errorf("Decision(%d).String() = %q, want %q", tt.d, got, tt.want)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/v2/stage/ -run TestDecision_String -v`
Expected: FAIL — `String()` method does not exist

**Step 3: Write minimal implementation**

In `stage.go`:

```go
func (d Decision) String() string {
	switch d {
	case DecisionProceed:
		return "Proceed"
	case DecisionSkip:
		return "Skip"
	case DecisionBlock:
		return "Block"
	case DecisionFail:
		return "Fail"
	default:
		return "Unknown"
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/v2/stage/ -run TestDecision_String -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/v2/stage/stage.go internal/v2/stage/stage_test.go
git commit -m "feat: add Decision.String() for structured commit messages"
```

---

### Task 4: Add StageCommitter to SpecLoop for spec-level stages

**Files:**
- Modify: `internal/v2/loop/spec_loop.go`
- Test: `internal/v2/loop/spec_loop_test.go`

**Step 1: Write the failing test**

```go
func TestSpecLoop_StageCommitter_CommitsAfterSpecStages(t *testing.T) {
	fg := testutil.NewFakeGit()
	fg.StatusOutput = "M  plan.md\n"
	committer := &pipeline.StageCommitter{Git: fg}

	adapters := testutil.DefaultAdapterSet(fg)
	cfg := testutil.MinimalConfig()
	gate := &alwaysReadyGate{}

	sl, err := NewSpecLoop(adapters, cfg, gate,
		WithStageCommitter(committer),
		WithPlanStage(fakePlanStage()),
		WithDecomposeStage(fakeDecomposeStage()),
		WithBeadLoop(&noopBeadRunner{}),
		WithAcceptStage(fakeAcceptStage()),
		WithPresentStage(fakePresentStage(), &present.SummaryContext{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	err = sl.Run(context.Background(), "test-spec", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify structured commits exist for spec-level stages
	var structured []string
	for _, msg := range fg.CommitMessages {
		if _, ok := pipeline.ParseCommitMessage(msg); ok {
			structured = append(structured, msg)
		}
	}
	if len(structured) == 0 {
		t.Error("expected structured commits for spec-level stages, got none")
	}
	// Should include plan and decompose at minimum
	foundPlan := false
	foundDecompose := false
	for _, msg := range structured {
		info, _ := pipeline.ParseCommitMessage(msg)
		if info.StageName == "plan" {
			foundPlan = true
		}
		if info.StageName == "decompose" {
			foundDecompose = true
		}
	}
	if !foundPlan {
		t.Error("missing structured commit for plan stage")
	}
	if !foundDecompose {
		t.Error("missing structured commit for decompose stage")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/v2/loop/ -run TestSpecLoop_StageCommitter -v`
Expected: FAIL — `WithStageCommitter` does not exist

**Step 3: Write minimal implementation**

In `spec_loop.go`:

1. Add import for `pipeline` package and add field + option:
```go
// In SpecLoop struct:
stageCommitter *pipeline.StageCommitter

// New option:
func WithStageCommitter(sc *pipeline.StageCommitter) SpecLoopOption {
	return func(s *SpecLoop) {
		s.stageCommitter = sc
	}
}
```

2. Add helper method:
```go
func (s *SpecLoop) commitSpecStage(ctx context.Context, worktree, stageName string, iteration int, decision string) {
	if s.stageCommitter == nil {
		return
	}
	// Empty beadID → "spec" scope in commit message
	if err := s.stageCommitter.CommitStage(ctx, worktree, "", stageName, iteration, decision); err != nil {
		log.Printf("spec stage commit %s: %v", stageName, err)
	}
}
```

3. Call `commitSpecStage` after each spec-level stage in `Run()`:
   - After plan stage succeeds (and plan is persisted): `s.commitSpecStage(ctx, worktree, "plan", 1, "Proceed")`
   - After decompose: `s.commitSpecStage(ctx, worktree, "decompose", 1, "Proceed")`
   - After accept succeeds in `ensureAcceptance`: `s.commitSpecStage(ctx, worktree, "accept", 1, "Proceed")`
   - After present succeeds: `s.commitSpecStage(ctx, worktree, "present", 1, "Proceed")`

**Step 4: Run test to verify it passes**

Run: `go test ./internal/v2/loop/ -run TestSpecLoop_StageCommitter -v`
Expected: PASS

**Step 5: Verify no regressions**

Run: `go test ./internal/v2/loop/... -v`
Expected: All tests pass

**Step 6: Commit**

```bash
git add internal/v2/loop/spec_loop.go internal/v2/loop/spec_loop_test.go
git commit -m "feat: wire StageCommitter into SpecLoop for spec-level stage commits"
```

**Dependencies:** Task 3

---

### Task 5: Wire FileSubscriber to typed emitter in SpecLoop

**Files:**
- Modify: `internal/v2/loop/spec_loop.go`
- Test: `internal/v2/loop/spec_loop_test.go`

**Step 1: Write the failing test**

```go
func TestSpecLoop_FileSubscriber_WritesEventsToWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	fg := testutil.NewFakeGit()
	fg.WorktreeRoot = tmpDir

	typedEmitter := event.NewEmitter()
	defer typedEmitter.Close()

	adapters := testutil.DefaultAdapterSet(fg)
	cfg := testutil.MinimalConfig()
	gate := &alwaysReadyGate{}

	sl, err := NewSpecLoop(adapters, cfg, gate,
		WithTypedEmitter(typedEmitter),
		WithPlanStage(fakePlanStage()),
		WithDecomposeStage(fakeDecomposeStage()),
		WithBeadLoop(&noopBeadRunner{}),
		WithAcceptStage(fakeAcceptStage()),
		WithPresentStage(fakePresentStage(), &present.SummaryContext{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = sl.Run(context.Background(), "test-spec", nil)

	eventsPath := filepath.Join(tmpDir, "test-spec", ".gromit", "v2", "events.jsonl")
	data, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("event log not written: %v", err)
	}
	if len(data) == 0 {
		t.Error("event log is empty")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/v2/loop/ -run TestSpecLoop_FileSubscriber -v`
Expected: FAIL — `WithTypedEmitter` does not exist

**Step 3: Write minimal implementation**

In `spec_loop.go`:

1. Add typed emitter field and option:
```go
// In SpecLoop struct:
typedEmitter *event.Emitter

// New option:
func WithTypedEmitter(e *event.Emitter) SpecLoopOption {
	return func(s *SpecLoop) {
		s.typedEmitter = e
	}
}
```

2. After checkout succeeds in `Run()`, wire a FileSubscriber:
```go
// After worktree is known, wire event log subscriber
var fileSubscriber *event.FileSubscriber
if s.typedEmitter != nil {
	gromitDir := s.cfg.Paths.GromitDir
	if gromitDir == "" {
		gromitDir = defaultGromitDir
	}
	eventsPath := filepath.Join(worktree, gromitDir, v2DirName, "events.jsonl")
	fileSubscriber = event.NewFileSubscriber(eventsPath)
	fileSubscriber.SubscribeTo(s.typedEmitter)
}
defer func() {
	if fileSubscriber != nil {
		_ = fileSubscriber.Close()
	}
}()
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/v2/loop/ -run TestSpecLoop_FileSubscriber -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/v2/loop/spec_loop.go internal/v2/loop/spec_loop_test.go
git commit -m "feat: wire FileSubscriber to typed emitter after checkout in SpecLoop"
```

**Dependencies:** None (independent of Tasks 2-4)

---

### Task 6: Wire everything in run2_components.go

**Files:**
- Modify: `internal/v2/loop/run2_components.go`
- Test: `internal/v2/loop/run2_components_test.go`

**Step 1: Write the failing test**

```go
func TestNewRun2LoopComponents_CreatesStageCommitter(t *testing.T) {
	components, err := NewRun2LoopComponents(testCfg(), testAdapters(), nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if components.StageCommitter == nil {
		t.Error("expected StageCommitter to be created")
	}
}
```

Note: `testCfg()` and `testAdapters()` are test helpers — use whatever pattern already exists in `run2_components_test.go`. If no test file exists, create helpers following the project pattern.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/v2/loop/ -run TestNewRun2LoopComponents_CreatesStageCommitter -v`
Expected: FAIL — `StageCommitter` field does not exist on `Run2LoopComponents`

**Step 3: Write minimal implementation**

In `run2_components.go`:

1. Add import for `pipeline` package.

2. Add `StageCommitter` and `TypedEmitter` to `Run2LoopComponents`:
```go
StageCommitter *pipeline.StageCommitter
TypedEmitter   *event.Emitter
```

3. In `NewRun2LoopComponents`, create the committer:
```go
stageCommitter := &pipeline.StageCommitter{Git: adapters.Git}
```

4. Pass to BeadLoop config:
```go
StageCommitter: stageCommitter,
```

5. Wire squasher into Present stage via `WithSquasher`:
```go
presentStage, err := present.New(cfg, adapters.Presenter, summaryCtx,
	present.WithSquasher(func(ctx context.Context) error {
		return pipeline.SquashPerBead(ctx, adapters.Git, summaryCtx.Worktree, summaryCtx.BeadSummaries)
	}),
)
```

6. Add to returned struct:
```go
StageCommitter: stageCommitter,
TypedEmitter:   typedEmitter,
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/v2/loop/ -run TestNewRun2LoopComponents_CreatesStageCommitter -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/v2/loop/run2_components.go internal/v2/loop/run2_components_test.go
git commit -m "feat: wire StageCommitter, squasher, and typed emitter in run2_components"
```

**Dependencies:** Tasks 2, 4

---

### Task 7: Wire StageCommitter and TypedEmitter in run2.go

**Files:**
- Modify: `cmd/gromit/run2.go`
- Test: `cmd/gromit/run2_test.go`

**Step 1: Write the failing test**

```go
func TestRun2_PassesStageCommitterToSpecLoop(t *testing.T) {
	// Capture the options passed to newSpecLoopFn
	var capturedOpts []loop.SpecLoopOption
	origFn := newSpecLoopFn
	t.Cleanup(func() { newSpecLoopFn = origFn })
	newSpecLoopFn = func(adapters adapter.AdapterSet, cfg *config.Config, gate loop.DependencyGate, opts ...loop.SpecLoopOption) (specLoop, error) {
		capturedOpts = opts
		return &fakeSpecLoop{}, nil
	}
	// ... invoke run2 with minimal setup (follow existing test patterns) ...
	// Assert capturedOpts length increased (StageCommitter + TypedEmitter added)
}
```

Follow the existing test patterns in `run2_test.go` for how to set up the test harness with injectable function vars.

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/gromit/ -run TestRun2_PassesStageCommitterToSpecLoop -v`
Expected: FAIL

**Step 3: Write minimal implementation**

In `run2.go`, add `WithStageCommitter` and `WithTypedEmitter` to `baseOpts`:

```go
baseOpts := []loop.SpecLoopOption{
	newSpecLoopEmitterFn(emitter),
	loop.WithPlanStage(components.PlanStage),
	loop.WithPresentStage(components.PresentStage, components.PresentSummaryContext),
	loop.WithDecomposeStage(components.DecomposeStage),
	loop.WithBeadLoop(components.BeadLoop),
	loop.WithAcceptStage(components.AcceptStage),
	loop.WithRemediationRunner(components.RemediationRunner),
	loop.WithStageCommitter(components.StageCommitter),
	loop.WithTypedEmitter(components.TypedEmitter),
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/gromit/ -run TestRun2_PassesStageCommitterToSpecLoop -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/gromit/run2.go cmd/gromit/run2_test.go
git commit -m "feat: pass StageCommitter and TypedEmitter to SpecLoop in run2"
```

**Dependencies:** Task 6

---

### Task 8: Integration test — commit-per-stage with real git

**Files:**
- Create: `internal/v2/pipeline/integration_test.go`

**Step 1: Write the integration test**

This test uses a real git repo (via `t.TempDir()` + `git init`), the real `StageCommitter` backed by `ExecGitAdapter`, and verifies the resulting commit history:

```go
func TestIntegration_CommitPerStage(t *testing.T) {
	// 1. git init in t.TempDir(), create initial commit
	// 2. Create StageCommitter backed by ExecGitAdapter
	// 3. Write a file, CommitStage for build
	// 4. Write another file, CommitStage for validate
	// 5. Write another file, CommitStage for review
	// 6. Use git.Log() to get entries
	// 7. Verify 3 structured commits exist, each parseable
	// 8. Verify commit order: build → validate → review (most recent first in log)
}

func TestIntegration_CommitPerStage_RetryPreservation(t *testing.T) {
	// 1. git init, initial commit
	// 2. CommitStage for build iter:1 (Fail)
	// 3. CommitStage for build iter:2 (Proceed)
	// 4. CommitStage for validate iter:2 (Proceed)
	// 5. Verify all 3 commits exist — iter:1 is preserved in history
	// 6. Verify ParseCommitMessage extracts correct iteration numbers
}
```

Key assertions:
- `git log` returns commits matching `[bead:b1/build/iter:1] Proceed`, etc.
- Each commit message is machine-parseable via `ParseCommitMessage`
- Retry attempts produce separate commits with correct iteration numbers

**Step 2: Implement and run**

Run: `go test ./internal/v2/pipeline/ -run TestIntegration -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/v2/pipeline/integration_test.go
git commit -m "test: integration test for commit-per-stage with real git"
```

**Dependencies:** Tasks 2, 5

---

### Task 9: Integration test — branch lifecycle (preserve on failure, delete on success)

**Files:**
- Create: `internal/v2/loop/branch_lifecycle_integration_test.go`

**Step 1: Write the integration test**

```go
func TestIntegration_BranchLifecycle_PreservedOnFailure(t *testing.T) {
	// 1. Set up SpecLoop with preserveOnFailure=true (default)
	// 2. FakeGit tracks RemoveWorktree calls
	// 3. Configure accept stage to fail
	// 4. Run the spec loop — expect error
	// 5. Assert RemoveWorktree was NOT called (worktree preserved)
	// 6. Assert partial-work commit was made
}

func TestIntegration_BranchLifecycle_DeletedOnSuccess(t *testing.T) {
	// 1. Set up SpecLoop with all stages succeeding
	// 2. Run the spec loop
	// 3. Assert RemoveWorktree WAS called
}
```

Use existing test patterns from `spec_loop_worktree_cleanup_test.go` as reference.

**Step 2: Implement and run**

Run: `go test ./internal/v2/loop/ -run TestIntegration_BranchLifecycle -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/v2/loop/branch_lifecycle_integration_test.go
git commit -m "test: integration tests for branch lifecycle on success/failure"
```

**Dependencies:** Tasks 4, 5

---

### Task 10: Integration test — per-bead squash

**Files:**
- Modify or create: `internal/v2/pipeline/integration_test.go`

**Step 1: Write the test**

```go
func TestIntegration_SquashPerBead(t *testing.T) {
	// 1. git init in t.TempDir(), create initial commit
	// 2. Create structured commits for 2 beads (b1: build, validate, review; b2: build, validate, review)
	// 3. Call SquashPerBead with bead summaries
	// 4. Verify git log shows a single squashed commit (or one per bead)
	// 5. Verify squashed commit message contains bead titles
}
```

**Step 2: Implement and run**

Run: `go test ./internal/v2/pipeline/ -run TestIntegration_SquashPerBead -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/v2/pipeline/integration_test.go
git commit -m "test: integration test for per-bead squash"
```

**Dependencies:** Task 8

---

### Task 11: Add `gromit debug2` command

**Files:**
- Create: `cmd/gromit/debug2.go`
- Create: `cmd/gromit/debug2_test.go`

**Step 1: Write the failing test**

```go
func TestDebug2_FindsPreservedWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	specName := "test-spec"
	wtPath := filepath.Join(tmpDir, "spec-worktrees", specName)

	// Create worktree dir with event log
	eventsDir := filepath.Join(wtPath, ".gromit", "v2")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	eventLine := `{"type":"stage.completed","stage_name":"build","bead_id":"b1"}` + "\n"
	if err := os.WriteFile(filepath.Join(eventsDir, "events.jsonl"), []byte(eventLine), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run debug2 and capture output
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	// ... invoke debug2RunE with specName, configured to use tmpDir as gromit dir ...
	// Assert output contains "Spec: test-spec" and event data
}

func TestDebug2_ReportsNoWorktreeFound(t *testing.T) {
	tmpDir := t.TempDir()
	// Run debug2 for a spec that doesn't exist
	// Assert error contains "no preserved worktree found"
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/gromit/ -run TestDebug2 -v`
Expected: FAIL — file doesn't exist

**Step 3: Write minimal implementation**

Create `cmd/gromit/debug2.go`:

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	gitadapter "github.com/danabrams/gromit/internal/v2/adapter/git"
	"github.com/danabrams/gromit/internal/v2/pipeline"
	"github.com/spf13/cobra"
)

var debug2Cmd = &cobra.Command{
	Use:   "debug2 <spec-name>",
	Short: "Diagnose and fix a failed v2 spec execution",
	Args:  cobra.ExactArgs(1),
	RunE:  debug2RunE,
}

func init() {
	rootCmd.AddCommand(debug2Cmd)
}

func debug2RunE(cmd *cobra.Command, args []string) error {
	specName := args[0]
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	gromitDir := resolveGromitDir(cfg)
	worktreesDir := filepath.Join(gromitDir, "spec-worktrees")
	wtPath := filepath.Join(worktreesDir, specName)

	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		return fmt.Errorf("no preserved worktree found for spec %q at %s", specName, wtPath)
	}

	eventsPath := filepath.Join(wtPath, ".gromit", "v2", "events.jsonl")
	eventsData, err := os.ReadFile(eventsPath)
	if err != nil {
		return fmt.Errorf("reading event log: %w", err)
	}

	gitAdapter := gitadapter.NewExecGitAdapter(".", worktreesDir)
	entries, err := gitAdapter.Log(cmd.Context(), wtPath, 100)
	if err != nil {
		return fmt.Errorf("reading git log: %w", err)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Spec: %s\n", specName)
	fmt.Fprintf(out, "Worktree: %s\n", wtPath)
	fmt.Fprintf(out, "Events: %d bytes\n", len(eventsData))
	fmt.Fprintf(out, "Commits: %d\n\n", len(entries))

	for _, e := range entries {
		info, ok := pipeline.ParseCommitMessage(e.Message)
		if ok {
			scope := "spec"
			if info.BeadID != "" {
				scope = info.BeadID
			}
			fmt.Fprintf(out, "  %s %s/%s iter:%d -> %s\n",
				e.Hash[:min(8, len(e.Hash))], scope, info.StageName, info.Iteration, info.Decision)
		} else {
			fmt.Fprintf(out, "  %s %s\n", e.Hash[:min(8, len(e.Hash))], e.Message)
		}
	}

	fmt.Fprintf(out, "\nEvent log:\n%s\n", string(eventsData))
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/gromit/ -run TestDebug2 -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/gromit/debug2.go cmd/gromit/debug2_test.go
git commit -m "feat: add gromit debug2 command for failed spec diagnosis"
```

**Dependencies:** None (fully independent)

---

## Task Dependency Graph

```
Task 1 (FakeGit StatusOutput)
  └─→ Task 2 (BeadLoop StageCommitter)
        └─→ Task 6 (run2_components wiring)
              └─→ Task 7 (run2.go wiring)

Task 3 (Decision.String)
  └─→ Task 4 (SpecLoop StageCommitter)
        └─→ Task 6
        └─→ Task 9 (branch lifecycle integration)

Task 5 (FileSubscriber in SpecLoop) ← independent
  └─→ Task 8 (commit-per-stage integration)
        └─→ Task 10 (squash integration)
  └─→ Task 9

Task 11 (debug2 command) ← fully independent
```

**Parallel tracks:**
- Track A: Tasks 1 → 2 → 6 → 7
- Track B: Tasks 3 → 4
- Track C: Task 5
- Track D: Task 11
- Then: Tasks 8, 9, 10 (integration tests, after tracks converge)
