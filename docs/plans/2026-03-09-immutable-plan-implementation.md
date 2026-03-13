# Immutable Plan with Additive Remediation — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Prevent the v2 spec loop from replanning or redecomposing a spec after the first run. Remediation creates additive beads via a separate plan-decompose cycle.

**Architecture:** Split `queryExistingBeads` into two queries: one that checks whether beads have ever been created (any status), and one that collects open beads for the bead loop. Refactor `RemediationRunner` to own a plan stage, persist remediation plans separately, and tag new beads with the original spec label.

**Tech Stack:** Go, existing v2 stage/adapter interfaces, `testing` package

---

### Task 1: Split queryExistingBeads into existence check + open bead query

The current `queryExistingBeads` filters `Status: "open"`, so closed beads cause re-decomposition on resume. Split it into two methods: one checks if any beads exist for this spec (any status), the other returns only open beads for the bead loop.

**Files:**
- Modify: `internal/v2/loop/spec_loop.go:451-480`

**Step 1: Write the failing test**

Add to `internal/v2/loop/spec_loop_test.go`:

```go
func TestSpecLoop_ResumeSkipsDecomposeWhenClosedBeadsExist(t *testing.T) {
	t.Parallel()

	git := newFakeGitAdapter(t)
	// Pre-populate a valid plan file so plan stage is skipped.
	git.planContent = validPlanFixture
	tracker := newFakeTaskTrackerAdapter()
	tracker.queryBeadsResponse = &tasktracker.TaskTrackerQueryBeadsResponse{
		Beads: []tasktracker.Bead{
			{ID: "bead-1", Title: "closed bead", Status: "closed", Labels: []string{"spec:test-spec"}},
		},
	}

	decompose := &fakeDecomposeStage{producedBeads: []*bead.Bead{{ID: "new-1", Title: "should not appear"}}}
	beadRunner := &fakeBeadRunner{}
	accept := &fakeAcceptStage{}
	presentStage, summaryCtx := newFakePresentStage()
	sc := &fakeStageCommitter{}

	adapters := newFakeAdapters(git, tracker)
	cfg := newTestConfig()
	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithPlanStage(&fakePlanStage{plan: "should not run"}),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadRunner),
		WithAcceptStage(accept),
		WithPresentStage(presentStage, summaryCtx),
		WithStageCommitter(sc),
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := loopInstance.Run(context.Background(), "test-spec", nil); err != nil {
		t.Fatal(err)
	}

	if decompose.called {
		t.Fatal("decompose stage should NOT have been called when closed beads exist")
	}
}
```

Note: `validPlanFixture` and helper constructors (`newFakeAdapters`, `newTestConfig`, etc.) already exist in `spec_loop_helpers_test.go`. If `fakeTaskTrackerAdapter` does not support `queryBeadsResponse` for multiple statuses, you may need to extend it — check the existing helper before duplicating.

**Step 2: Run the test to verify it fails**

Run: `go test ./internal/v2/loop/ -run TestSpecLoop_ResumeSkipsDecomposeWhenClosedBeadsExist -v`
Expected: FAIL — decompose is called because `queryExistingBeads` filters out the closed bead and returns empty.

**Step 3: Write the implementation**

In `internal/v2/loop/spec_loop.go`, replace `queryExistingBeads` with two methods:

```go
// hasBeadsForSpec returns true if any beads (open, closed, or otherwise) have
// ever been created for this spec. Used to guard against re-decomposition.
func (s *SpecLoop) hasBeadsForSpec(ctx context.Context, specID string) (bool, error) {
	if s.adapters.TaskTracker == nil {
		return false, nil
	}
	label := fmt.Sprintf("spec:%s", specID)
	resp, err := s.adapters.TaskTracker.QueryBeads(ctx, trackertypes.TaskTrackerQueryBeadsRequest{
		Labels: []string{label},
		// Status intentionally empty — matches all statuses.
	})
	if err != nil {
		return false, err
	}
	return resp != nil && len(resp.Beads) > 0, nil
}

// openBeadsForSpec returns only open beads for the bead loop.
func (s *SpecLoop) openBeadsForSpec(ctx context.Context, specID string) ([]*bead.Bead, error) {
	if s.adapters.TaskTracker == nil {
		return nil, nil
	}
	label := fmt.Sprintf("spec:%s", specID)
	resp, err := s.adapters.TaskTracker.QueryBeads(ctx, trackertypes.TaskTrackerQueryBeadsRequest{
		Labels: []string{label},
		Status: "open",
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || len(resp.Beads) == 0 {
		return nil, nil
	}
	beads := make([]*bead.Bead, len(resp.Beads))
	for i, b := range resp.Beads {
		beads[i] = &bead.Bead{
			ID:          b.ID,
			Title:       b.Title,
			Description: b.Description,
			Priority:    b.Priority,
			Labels:      b.Labels,
			Status:      b.Status,
			DependsOn:   stringsToDependencies(b.DependsOn),
			BlockedBy:   stringsToDependencies(b.BlockedBy),
		}
	}
	return beads, nil
}
```

Then update the decompose guard in `Run()` (lines 338-385) to use both:

```go
s.recordStage("decompose")
var beads []*bead.Bead

hasBeads, err := s.hasBeadsForSpec(ctx, specID)
if err != nil {
	return fmt.Errorf("check existing beads: %w", err)
}

if hasBeads {
	openBeads, err := s.openBeadsForSpec(ctx, specID)
	if err != nil {
		return fmt.Errorf("query open beads: %w", err)
	}
	beads = openBeads
	s.emit(&events.DecomposeResumedEvent{SpecID: specID, BeadCount: len(beads)})
	if s.selectiveRevalidator != nil && len(beads) > 0 {
		// ... existing revalidation logic unchanged ...
	}
} else {
	s.applyRouting(&req, "decompose")
	beads, err = s.runDecompose(ctx, req)
	if err != nil {
		return err
	}
	if err := s.commitStage(ctx, worktree, "decompose", 0, "proceed"); err != nil {
		return fmt.Errorf("commit after decompose: %w", err)
	}
}
```

Delete the old `queryExistingBeads` method.

**Step 4: Run the test to verify it passes**

Run: `go test ./internal/v2/loop/ -run TestSpecLoop_ResumeSkipsDecomposeWhenClosedBeadsExist -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/v2/loop/spec_loop.go internal/v2/loop/spec_loop_test.go
git commit -m "fix: prevent re-decomposition when closed beads exist for spec"
```

---

### Task 2: Test resume with mixed-status beads

Verify that when both closed and open beads exist, decompose is skipped and only open beads reach the bead loop.

**Files:**
- Modify: `internal/v2/loop/spec_loop_test.go`

**Step 1: Write the failing test**

```go
func TestSpecLoop_ResumeSkipsDecomposeWhenMixedStatusBeadsExist(t *testing.T) {
	t.Parallel()

	git := newFakeGitAdapter(t)
	git.planContent = validPlanFixture
	tracker := newFakeTaskTrackerAdapter()
	// QueryBeads will be called twice: once with no status filter, once with "open".
	// The helper needs to support this. If fakeTaskTrackerAdapter uses a single
	// queryBeadsResponse, extend it with a queryBeadsFn callback instead.
	// For now, seed the tracker with beads directly if the fake supports it.

	decompose := &fakeDecomposeStage{producedBeads: []*bead.Bead{{ID: "wrong", Title: "should not appear"}}}
	beadRunner := &fakeBeadRunner{}
	accept := &fakeAcceptStage{}
	presentStage, summaryCtx := newFakePresentStage()
	sc := &fakeStageCommitter{}

	adapters := newFakeAdapters(git, tracker)
	cfg := newTestConfig()
	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithPlanStage(&fakePlanStage{plan: "unused"}),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadRunner),
		WithAcceptStage(accept),
		WithPresentStage(presentStage, summaryCtx),
		WithStageCommitter(sc),
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := loopInstance.Run(context.Background(), "test-spec", nil); err != nil {
		t.Fatal(err)
	}

	if decompose.called {
		t.Fatal("decompose should not run when beads exist (any status)")
	}
	// Bead runner should receive only the open bead.
	if len(beadRunner.lastBeads) != 1 || beadRunner.lastBeads[0].ID != "open-bead" {
		t.Fatalf("bead runner got %v, want [open-bead]", beadRunner.lastBeads)
	}
}
```

Note: You will need to extend `fakeTaskTrackerAdapter` to support two different QueryBeads calls (one for all statuses, one for "open"). The simplest approach: replace `queryBeadsResponse` with a `queryBeadsFn func(req) (*resp, error)` callback, or seed it with actual beads that the existing filter logic handles. Check `spec_loop_helpers_test.go` for the current shape before deciding.

**Step 2: Run to verify it fails (or passes if Task 1 already covers it)**

Run: `go test ./internal/v2/loop/ -run TestSpecLoop_ResumeSkipsDecomposeWhenMixedStatusBeadsExist -v`

**Step 3: Adjust fake if needed, verify pass**

**Step 4: Commit**

```bash
git add internal/v2/loop/spec_loop_test.go internal/v2/loop/spec_loop_helpers_test.go
git commit -m "test: verify resume with mixed-status beads skips decompose"
```

---

### Task 3: Test that first run still decomposes

Regression test: when no beads exist and no plan file, the full plan → decompose flow runs.

**Files:**
- Modify: `internal/v2/loop/spec_loop_test.go`

**Step 1: Write the test**

```go
func TestSpecLoop_FirstRunDecomposesWhenNoBeadsExist(t *testing.T) {
	t.Parallel()

	git := newFakeGitAdapter(t)
	// No planContent — forces plan stage to run.
	tracker := newFakeTaskTrackerAdapter()
	// No beads seeded — forces decompose to run.

	planStage := &fakePlanStage{plan: "# Fresh plan"}
	decompose := &fakeDecomposeStage{producedBeads: []*bead.Bead{
		{ID: "bead-1", Title: "first bead"},
	}}
	beadRunner := &fakeBeadRunner{}
	accept := &fakeAcceptStage{}
	presentStage, summaryCtx := newFakePresentStage()
	sc := &fakeStageCommitter{}

	adapters := newFakeAdapters(git, tracker)
	cfg := newTestConfig()
	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithPlanStage(planStage),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadRunner),
		WithAcceptStage(accept),
		WithPresentStage(presentStage, summaryCtx),
		WithStageCommitter(sc),
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := loopInstance.Run(context.Background(), "test-spec", nil); err != nil {
		t.Fatal(err)
	}

	if !planStage.called {
		t.Fatal("plan stage should run on first run")
	}
	if !decompose.called {
		t.Fatal("decompose stage should run on first run")
	}
}
```

**Step 2: Run and verify it passes (regression)**

Run: `go test ./internal/v2/loop/ -run TestSpecLoop_FirstRunDecomposesWhenNoBeadsExist -v`
Expected: PASS — existing behavior preserved.

**Step 3: Commit**

```bash
git add internal/v2/loop/spec_loop_test.go
git commit -m "test: regression test confirming first run still plans and decomposes"
```

---

### Task 4: Add PlanStage and worktree path to RemediationRunnerConfig

The remediation runner needs to create its own plan before decomposing. Add the plan stage, spec label, and worktree path so it can persist remediation plans.

**Files:**
- Modify: `internal/v2/remediation/remediation.go:15-25`

**Step 1: Write the failing test**

```go
func TestRemediation_CreatesRemediationPlanNotOriginal(t *testing.T) {
	t.Parallel()

	planCalled := false
	planStage := &testStage{
		name: "plan",
		run: func(_ context.Context, req *stage.StageRequest) (*stage.StageResult, error) {
			planCalled = true
			if !req.Remediation {
				t.Fatal("plan stage should receive Remediation=true")
			}
			if req.GapAnalysis == "" {
				t.Fatal("plan stage should receive gap analysis")
			}
			return &stage.StageResult{Decision: stage.DecisionProceed}, nil
		},
	}

	decomposeCalled := false
	decomposeStage := &testStage{
		name: "decompose",
		run: func(_ context.Context, req *stage.StageRequest) (*stage.StageResult, error) {
			decomposeCalled = true
			return &stage.StageResult{
				Decision:  stage.DecisionProceed,
				Artifacts: &stage.DecomposeArtifacts{Beads: []*bead.Bead{{ID: "fix-1", Title: "fix"}}},
			}, nil
		},
	}

	acceptCalls := 0
	acceptStage := &testStage{
		name: "accept",
		run: func(_ context.Context, _ *stage.StageRequest) (*stage.StageResult, error) {
			acceptCalls++
			if acceptCalls == 1 {
				return &stage.StageResult{
					Decision:  stage.DecisionFail,
					Artifacts: &gapArtifacts{gap: "missing error handling"},
				}, nil
			}
			return &stage.StageResult{Decision: stage.DecisionProceed}, nil
		},
	}

	runner := NewRemediationRunner(RemediationRunnerConfig{
		AcceptStage:    acceptStage,
		PlanStage:      planStage,
		DecomposeStage: decomposeStage,
		BeadRunner:     &testBeadRunner{},
		GenerationCap:  1,
	})

	if err := runner.Run(context.Background(), "test-spec", t.TempDir()); err != nil {
		t.Fatal(err)
	}

	if !planCalled {
		t.Fatal("remediation should invoke plan stage")
	}
	if !decomposeCalled {
		t.Fatal("remediation should invoke decompose stage")
	}
}
```

**Step 2: Run to verify it fails**

Run: `go test ./internal/v2/remediation/ -run TestRemediation_CreatesRemediationPlanNotOriginal -v`
Expected: FAIL — `PlanStage` field does not exist on `RemediationRunnerConfig`.

**Step 3: Implement**

Add the `PlanStage` field to `RemediationRunnerConfig`:

```go
type RemediationRunnerConfig struct {
	AcceptStage     stage.Stage
	PlanStage       stage.Stage // NEW: creates remediation plan from gap analysis
	GapStage        stage.Stage
	DecomposeStage  stage.Stage
	BeadRunner      BeadRunner
	GenerationCap   int
	Presenter       adapter.PresenterAdapter
	Emitter         *events.Emitter
	WorktreeCleaner WorktreeCleaner
}
```

Update `executeRemediation` to call the plan stage before decompose:

```go
func (r *RemediationRunner) executeRemediation(ctx context.Context, req *stage.Request, gapAnalysis string) error {
	specID := req.Bead.ID
	if !r.canRemediate() {
		return r.handleGenerationCap(ctx, specID)
	}

	req.Remediation = true
	req.GapAnalysis = gapAnalysis

	if r.cfg.GapStage != nil {
		if _, err := r.cfg.GapStage.Run(ctx, req); err != nil {
			return err
		}
	}

	// Create a focused remediation plan from the gap analysis.
	if r.cfg.PlanStage != nil {
		if _, err := r.cfg.PlanStage.Run(ctx, req); err != nil {
			return fmt.Errorf("remediation plan: %w", err)
		}
	}

	beads, err := r.decompose(ctx, req)
	if err != nil {
		return err
	}

	if r.cfg.BeadRunner == nil {
		return ErrBeadRunnerRequired
	}
	if err := r.cfg.BeadRunner.Run(ctx, beads); err != nil {
		return err
	}

	r.generationCount++
	return nil
}
```

**Step 4: Run and verify pass**

Run: `go test ./internal/v2/remediation/ -run TestRemediation_CreatesRemediationPlanNotOriginal -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/v2/remediation/remediation.go internal/v2/remediation/remediation_test.go
git commit -m "feat: add plan stage to remediation runner for additive remediation"
```

---

### Task 5: Persist remediation plans as remediation-N.md

The remediation runner should persist each remediation plan to a numbered file so it survives crashes and the original plan stays untouched.

**Files:**
- Modify: `internal/v2/remediation/remediation.go`

**Step 1: Write the failing test**

```go
func TestRemediation_RemediationPlanPersistedSeparately(t *testing.T) {
	t.Parallel()

	worktree := t.TempDir()
	gromitV2Dir := filepath.Join(worktree, ".gromit", "v2")
	if err := os.MkdirAll(gromitV2Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write original plan
	originalPlan := "# Original plan"
	if err := os.WriteFile(filepath.Join(gromitV2Dir, "plan.md"), []byte(originalPlan), 0o644); err != nil {
		t.Fatal(err)
	}

	acceptCalls := 0
	acceptStage := &testStage{
		name: "accept",
		run: func(_ context.Context, _ *stage.StageRequest) (*stage.StageResult, error) {
			acceptCalls++
			if acceptCalls == 1 {
				return &stage.StageResult{
					Decision:  stage.DecisionFail,
					Artifacts: &gapArtifacts{gap: "missing tests"},
				}, nil
			}
			return &stage.StageResult{Decision: stage.DecisionProceed}, nil
		},
	}

	decomposeStage := &testStage{
		name: "decompose",
		run: func(_ context.Context, _ *stage.StageRequest) (*stage.StageResult, error) {
			return &stage.StageResult{
				Decision:  stage.DecisionProceed,
				Artifacts: &stage.DecomposeArtifacts{Beads: []*bead.Bead{{ID: "fix-1"}}},
			}, nil
		},
	}

	planStage := &testStage{
		name: "plan",
		run: func(_ context.Context, _ *stage.StageRequest) (*stage.StageResult, error) {
			return &stage.StageResult{Decision: stage.DecisionProceed}, nil
		},
	}

	runner := NewRemediationRunner(RemediationRunnerConfig{
		AcceptStage:    acceptStage,
		PlanStage:      planStage,
		DecomposeStage: decomposeStage,
		BeadRunner:     &testBeadRunner{},
		GenerationCap:  1,
	})

	if err := runner.Run(context.Background(), "test-spec", worktree); err != nil {
		t.Fatal(err)
	}

	// Original plan unchanged
	data, err := os.ReadFile(filepath.Join(gromitV2Dir, "plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != originalPlan {
		t.Fatalf("original plan modified: got %q", string(data))
	}

	// Remediation plan persisted
	remPath := filepath.Join(gromitV2Dir, "remediation-1.md")
	if _, err := os.Stat(remPath); os.IsNotExist(err) {
		t.Fatal("remediation-1.md should exist")
	}
}
```

**Step 2: Run to verify it fails**

Run: `go test ./internal/v2/remediation/ -run TestRemediation_RemediationPlanPersistedSeparately -v`
Expected: FAIL — no persistence logic yet.

**Step 3: Implement**

Add a `remediationPlanPath` helper and persist after plan stage:

```go
func (r *RemediationRunner) remediationPlanPath(worktree string) string {
	gromitDir := filepath.Join(worktree, ".gromit", "v2")
	return filepath.Join(gromitDir, fmt.Sprintf("remediation-%d.md", r.generationCount+1))
}
```

In `executeRemediation`, after the plan stage call, persist if the plan stage returned artifacts (or persist the gap analysis as the plan content if the plan stage doesn't return a plan artifact):

```go
// Persist remediation plan
planPath := r.remediationPlanPath(req.Worktree)
if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
	return fmt.Errorf("create remediation plan dir: %w", err)
}
planContent := gapAnalysis
if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
	return fmt.Errorf("persist remediation plan: %w", err)
}
```

Add `"os"` and `"path/filepath"` to imports.

**Step 4: Run and verify pass**

Run: `go test ./internal/v2/remediation/ -run TestRemediation_RemediationPlanPersistedSeparately -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/v2/remediation/remediation.go internal/v2/remediation/remediation_test.go
git commit -m "feat: persist remediation plans as remediation-N.md"
```

---

### Task 6: Test second remediation creates remediation-2.md

**Files:**
- Modify: `internal/v2/remediation/remediation_test.go`

**Step 1: Write the test**

```go
func TestRemediation_SecondRemediationCreatesRemediationPlan2(t *testing.T) {
	t.Parallel()

	worktree := t.TempDir()
	gromitV2Dir := filepath.Join(worktree, ".gromit", "v2")
	if err := os.MkdirAll(gromitV2Dir, 0o755); err != nil {
		t.Fatal(err)
	}

	acceptCalls := 0
	acceptStage := &testStage{
		name: "accept",
		run: func(_ context.Context, _ *stage.StageRequest) (*stage.StageResult, error) {
			acceptCalls++
			if acceptCalls <= 2 {
				return &stage.StageResult{
					Decision:  stage.DecisionFail,
					Artifacts: &gapArtifacts{gap: fmt.Sprintf("gap-%d", acceptCalls)},
				}, nil
			}
			return &stage.StageResult{Decision: stage.DecisionProceed}, nil
		},
	}

	decomposeStage := &testStage{
		name: "decompose",
		run: func(_ context.Context, _ *stage.StageRequest) (*stage.StageResult, error) {
			return &stage.StageResult{
				Decision:  stage.DecisionProceed,
				Artifacts: &stage.DecomposeArtifacts{Beads: []*bead.Bead{{ID: "fix"}}},
			}, nil
		},
	}
	planStage := &testStage{
		name: "plan",
		run: func(_ context.Context, _ *stage.StageRequest) (*stage.StageResult, error) {
			return &stage.StageResult{Decision: stage.DecisionProceed}, nil
		},
	}

	runner := NewRemediationRunner(RemediationRunnerConfig{
		AcceptStage:    acceptStage,
		PlanStage:      planStage,
		DecomposeStage: decomposeStage,
		BeadRunner:     &testBeadRunner{},
		GenerationCap:  3,
	})

	if err := runner.Run(context.Background(), "test-spec", worktree); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"remediation-1.md", "remediation-2.md"} {
		path := filepath.Join(gromitV2Dir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatalf("%s should exist", name)
		}
	}
}
```

**Step 2: Run and verify pass**

Run: `go test ./internal/v2/remediation/ -run TestRemediation_SecondRemediationCreatesRemediationPlan2 -v`
Expected: PASS (if Task 5 logic is correct with `generationCount+1` numbering).

**Step 3: Commit**

```bash
git add internal/v2/remediation/remediation_test.go
git commit -m "test: verify second remediation creates remediation-2.md"
```

---

### Task 7: Verify generation cap still respected

Regression test — existing `TestRemediation_GenerationCap*` tests should still pass.

**Files:**
- None (run existing tests)

**Step 1: Run existing remediation tests**

Run: `go test ./internal/v2/remediation/ -v`
Expected: All existing tests PASS.

**Step 2: Commit (only if tests needed adjustment)**

---

### Task 8: Wire PlanStage into RemediationRunner in run2_components.go

The remediation runner is constructed in `NewRun2LoopComponents` without a `PlanStage`. Wire it in.

**Files:**
- Modify: `internal/v2/loop/run2_components.go:203-210`

**Step 1: Write the failing test**

Add to `internal/v2/loop/run2_components_test.go`:

```go
func TestNewRun2LoopComponents_RemediationRunnerHasPlanStage(t *testing.T) {
	// This test verifies that the remediation runner receives a non-nil PlanStage.
	// The exact assertion depends on whether RemediationRunner exposes its config.
	// If not, verify indirectly: run the remediation runner and assert the plan stage
	// was called. Alternatively, add a PlanStage() accessor to RemediationRunner.
}
```

Note: The exact test shape depends on how `RemediationRunner` exposes its internals. If it doesn't, the integration test in Task 10 covers this path. Consider adding a simple accessor like `HasPlanStage() bool` or verifying via the integration test only.

**Step 2: Implement**

In `run2_components.go`, update the `RemediationRunner` construction:

```go
remediationRunner := v2remediation.NewRemediationRunner(v2remediation.RemediationRunnerConfig{
	AcceptStage:    acceptStage,
	PlanStage:      planStage,  // NEW: wire in the plan stage
	DecomposeStage: decomposeStage,
	BeadRunner:     &remediationBeadRunner{loop: beadLoop},
	GenerationCap:  v2remediation.DefaultGenerationCap,
	Emitter:        legacyEmitter,
	Presenter:      adapters.Presenter,
})
```

**Step 3: Run all tests**

Run: `go test ./internal/v2/loop/ -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/v2/loop/run2_components.go internal/v2/loop/run2_components_test.go
git commit -m "feat: wire plan stage into remediation runner"
```

---

### Task 9: Run full test suite — verify no regressions

**Files:**
- None

**Step 1: Run all v2 tests**

```bash
go test ./internal/v2/...
```

Expected: All PASS.

**Step 2: Run spec loop and remediation tests with race detector**

```bash
go test -race ./internal/v2/loop/ ./internal/v2/remediation/
```

Expected: No race conditions.

**Step 3: Commit (only if fixes needed)**

---

### Task 10: Integration test — cancel and resume skips decompose

End-to-end test confirming a cancelled-and-resumed spec does not replan or redecompose.

**Files:**
- Modify: `internal/v2/loop/integration_test.go`

**Step 1: Write the integration test**

```go
func TestIntegration_CancelAndResumeSkipsDecompose(t *testing.T) {
	t.Parallel()

	// First run: plan + decompose + cancel mid-bead-loop.
	git := newIntegrationGitAdapter(t)
	git.planContent = validPlanFixture
	tracker := newIntegrationTaskTrackerAdapter()
	llmAdapter := newIntegrationLLMAdapter()

	decomposeCalls := 0
	planCalls := 0

	planStage := &fakePlanStage{
		plan: validPlanFixture,
		onRun: func() { planCalls++ },
	}

	decomposeStage := &fakeDecomposeStage{
		producedBeads: []*bead.Bead{
			{ID: "bead-1", Title: "task 1", Labels: []string{"spec:resume-test"}},
			{ID: "bead-2", Title: "task 2", Labels: []string{"spec:resume-test"}},
		},
		onRun: func() { decomposeCalls++ },
	}

	// Bead runner: on first run, close bead-1 and cancel.
	ctx1, cancel1 := context.WithCancel(context.Background())
	firstRunBeadRunner := &fakeBeadRunner{
		onRun: func(beads []*bead.Bead) {
			// Simulate closing bead-1 and cancelling.
			tracker.CloseBead(ctx1, tasktracker.CloseBeadRequest{BeadID: "bead-1"})
			cancel1()
		},
	}

	// Build and run first time — should plan + decompose.
	// ... (construct SpecLoop with above fakes, run with ctx1) ...

	// Assert: plan + decompose both called once.
	if planCalls != 1 { t.Fatalf("plan calls = %d, want 1", planCalls) }
	if decomposeCalls != 1 { t.Fatalf("decompose calls = %d, want 1", decomposeCalls) }

	// Second run: should skip plan and decompose, run bead loop with bead-2 only.
	secondRunBeadRunner := &fakeBeadRunner{}
	// ... (construct new SpecLoop, run with fresh context) ...

	if planCalls != 1 { t.Fatal("plan should not run again on resume") }
	if decomposeCalls != 1 { t.Fatal("decompose should not run again on resume") }
	if len(secondRunBeadRunner.lastBeads) != 1 || secondRunBeadRunner.lastBeads[0].ID != "bead-2" {
		t.Fatalf("resume bead loop got %v, want [bead-2]", secondRunBeadRunner.lastBeads)
	}
}
```

Note: This test is a sketch. Adapt the fake constructors and assertions to match the actual helper signatures in `integration_test.go`. The key assertions: plan and decompose run exactly once across both runs; second run sees only the remaining open bead.

**Step 2: Run and verify pass**

Run: `go test ./internal/v2/loop/ -run TestIntegration_CancelAndResumeSkipsDecompose -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/v2/loop/integration_test.go
git commit -m "test: integration test for cancel-and-resume skipping replan/redecompose"
```

---

### Task 11: Integration test — acceptance failure adds beads, does not replan

**Files:**
- Modify: `internal/v2/loop/integration_test.go`

**Step 1: Write the integration test**

Test that when acceptance fails, remediation creates new beads tagged with the same spec label, without re-running the original plan or decompose stages.

The key assertions:
- Original plan stage: called once.
- Original decompose stage: called once.
- Remediation plan stage: called once (for the remediation plan).
- Remediation decompose stage: called once (for the remediation beads).
- New beads carry `spec:SPECID` label.
- Original `plan.md` unchanged.
- `remediation-1.md` exists.

**Step 2: Run and verify pass**

Run: `go test ./internal/v2/loop/ -run TestIntegration_AcceptFailureRemediationAddsBeads -v`

**Step 3: Commit**

```bash
git add internal/v2/loop/integration_test.go
git commit -m "test: integration test for additive remediation beads"
```

---

### Task 12: Final full test suite run

**Step 1: Run everything**

```bash
go test ./...
```

**Step 2: Run with race detector on changed packages**

```bash
go test -race ./internal/v2/loop/ ./internal/v2/remediation/ ./internal/v2/adapter/tasktracker/
```

**Step 3: Final commit if any cleanup needed**

```bash
git commit -m "chore: final cleanup for immutable plan implementation"
```
