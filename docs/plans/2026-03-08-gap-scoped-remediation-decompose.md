# Gap-Scoped Remediation Decompose Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** When the remediation cycle re-decomposes after acceptance failure, create beads only for the unmet criteria — not the entire plan. Implements spec appendix A16 in `.gromit/specs/v2-run-loop.md`.

**Architecture:** Add a `GapAnalysis` field to `StageRequest` so the remediation runner can pass gap data structurally. The decompose stage uses the struct field when present, falls back to reading `gap-analysis.md` from disk when `Remediation=true`, and uses a gap-scoped prompt template that includes the plan for context but constrains bead creation to the failed criteria.

**Tech Stack:** Go, existing stage/remediation/decompose packages

---

### Task 1: Add GapAnalysis field to StageRequest

**Files:**
- Modify: `internal/v2/stage/stage.go:20-31`
- Test: `internal/v2/stage/decompose/decompose_test.go`

**Step 1: Add the field to StageRequest**

In `internal/v2/stage/stage.go`, add `GapAnalysis string` after the `Remediation bool` field:

```go
type StageRequest struct {
	Bead         BeadInfo
	Model        string
	Tier         string
	Provider     llmtypes.LLMProvider
	Iteration    int
	Config       *config.Config
	Worktree     string
	Remediation  bool
	GapAnalysis  string
	RetryContext *RetryContext
	Telemetry    *LLMCostSummary
}
```

**Step 2: Run existing tests to confirm no breakage**

Run: `go test ./internal/v2/...`
Expected: All existing tests pass (struct field addition is backwards-compatible)

**Step 3: Commit**

```bash
git add internal/v2/stage/stage.go
git commit -m "feat: add GapAnalysis field to StageRequest for remediation scoping"
```

---

### Task 2: Wire gap analysis from accept result into StageRequest in remediation runner

**Files:**
- Modify: `internal/v2/remediation/remediation.go:59-124`
- Test: `internal/v2/remediation/remediation_test.go`

**Step 1: Write the failing test**

Add to `internal/v2/remediation/remediation_test.go`:

```go
func TestRemediationRunnerPassesGapAnalysisToDecompose(t *testing.T) {
	t.Parallel()

	gapText := "Criterion 1 failed: commits not produced"

	acceptCalls := 0
	accept := &testStage{
		name: "accept",
		run: func(ctx context.Context, req *stage.Request) (*stage.Result, error) {
			acceptCalls++
			if acceptCalls == 1 {
				return &stage.Result{
					Decision: stage.DecisionFail,
					Artifacts: &accept_pkg.AcceptArtifacts{
						GapSummary: gapText,
					},
				}, nil
			}
			return &stage.Result{Decision: stage.DecisionProceed}, nil
		},
	}

	var capturedGap string
	decompose := &testStage{
		name: "decompose",
		run: func(ctx context.Context, req *stage.Request) (*stage.Result, error) {
			capturedGap = req.GapAnalysis
			return &stage.Result{
				Artifacts: &stage.DecomposeArtifacts{
					Beads: []*bead.Bead{{ID: "gap-bead"}},
				},
			}, nil
		},
	}

	runner := newRunnerForRemediationCycle(accept, decompose, &testBeadRunner{}, 1)
	if err := runner.Run(context.Background(), "spec-gap", ""); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if capturedGap != gapText {
		t.Fatalf("gap analysis = %q, want %q", capturedGap, gapText)
	}
}
```

Note: You will need to import `accept_pkg "github.com/danabrams/gromit/internal/v2/stage/accept"` for `AcceptArtifacts`. If the import creates a cycle, use a local struct that matches the `GapSummary` field pattern, or extract the gap summary with a type assertion on `res.Artifacts`.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/v2/remediation/ -run TestRemediationRunnerPassesGapAnalysisToDecompose -v`
Expected: FAIL — `capturedGap` is empty because `executeRemediation` doesn't extract or forward the gap summary.

**Step 3: Implement gap extraction in remediation runner**

In `internal/v2/remediation/remediation.go`, modify the `Run` method to capture the accept result's gap summary and pass it to `executeRemediation`:

```go
func (r *RemediationRunner) Run(ctx context.Context, specID, worktree string) error {
	r.generationCount = 0
	if specID == "" {
		return ErrSpecIDRequired
	}
	if r.cfg.AcceptStage == nil {
		return ErrAcceptStageRequired
	}

	reqTemplate := stage.Request{Bead: stage.BeadInfo{ID: specID}, Worktree: worktree}
	for {
		req := reqTemplate
		res, err := r.cfg.AcceptStage.Run(ctx, &req)
		if err != nil {
			return err
		}
		if res != nil && res.Decision == stage.DecisionFail {
			gapAnalysis := extractGapSummary(res)
			if err := r.executeRemediation(ctx, &req, gapAnalysis); err != nil {
				return err
			}
			continue
		}
		if err := r.cleanup(ctx, specID); err != nil {
			return err
		}
		return nil
	}
}
```

Add the `extractGapSummary` helper and update `executeRemediation` signature:

```go
func extractGapSummary(res *stage.Result) string {
	if res == nil || res.Artifacts == nil {
		return ""
	}
	type gapHolder interface {
		GetGapSummary() string
	}
	if gh, ok := res.Artifacts.(gapHolder); ok {
		return gh.GetGapSummary()
	}
	return ""
}

func (r *RemediationRunner) executeRemediation(ctx context.Context, req *stage.Request, gapAnalysis string) error {
	// ... existing code ...
	req.Remediation = true
	req.GapAnalysis = gapAnalysis
	// ... rest unchanged ...
}
```

Since `AcceptArtifacts` doesn't implement an interface, use reflection or a direct type assertion. The simplest approach: define a small interface in the remediation package:

```go
// gapSummaryProvider is satisfied by accept.AcceptArtifacts.
type gapSummaryProvider interface {
	GetGapSummary() string
}
```

Then add a `GetGapSummary()` method to `AcceptArtifacts` in `internal/v2/stage/accept/accept.go`:

```go
func (a *AcceptArtifacts) GetGapSummary() string {
	if a == nil {
		return ""
	}
	return a.GapSummary
}
```

Alternatively, if importing `accept` creates a cycle, use a struct-field approach with `reflect` or define the interface in the `stage` package. The cleanest path is the interface approach since `remediation` already imports `stage` but not `accept`.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/v2/remediation/ -run TestRemediationRunnerPassesGapAnalysisToDecompose -v`
Expected: PASS

**Step 5: Run all remediation tests**

Run: `go test ./internal/v2/remediation/ -v`
Expected: All pass — existing tests don't set `GapSummary` on their accept results, so `req.GapAnalysis` will be empty for them, preserving behavior.

**Step 6: Commit**

```bash
git add internal/v2/remediation/remediation.go internal/v2/stage/accept/accept.go internal/v2/remediation/remediation_test.go
git commit -m "feat: remediation runner extracts gap summary from accept and passes to decompose"
```

---

### Task 3: Add remediation prompt template to decompose stage

**Files:**
- Modify: `internal/v2/stage/decompose/decompose.go:35-57`
- Test: `internal/v2/stage/decompose/decompose_test.go`

**Step 1: Write the failing test — decompose uses gap-scoped prompt when GapAnalysis is set**

Add to `internal/v2/stage/decompose/decompose_test.go`:

```go
func TestRunUsesGapScopedPromptWhenGapAnalysisProvided(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	planDir := filepath.Join(tmpDir, ".gromit", "v2")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	planContent := "# Full Plan\nTask 1: Build widget\nTask 2: Build gadget"
	if err := os.WriteFile(filepath.Join(planDir, "plan.md"), []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	gapContent := "Criterion 3 failed: widgets don't commit events"

	cfg := &config.Config{
		ProjectRoot: tmpDir,
		Paths:       config.PathsConfig{GromitDir: ".gromit"},
	}

	llm := &fakeLLM{
		responses: []*llm.LLMResponse{{Success: true, Output: `[
			{
				"title": "fix widget commits",
				"description": "add event commits to widgets",
				"priority": "P1",
				"acceptance_criteria": ["widgets commit events"],
				"expected_outputs": ["commit after widget stage"],
				"depends_on_index": []
			}
		]`}},
	}
	tracker := &fakeTracker{}
	stage, err := New(cfg, llm, tracker)
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}

	req := &stagepkg.Request{
		Bead:        stagepkg.BeadInfo{ID: "spec", Labels: []string{"gen:0"}},
		Config:      cfg,
		Remediation: true,
		GapAnalysis: gapContent,
	}

	_, err = stage.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}

	if len(llm.calls) == 0 {
		t.Fatal("expected llm invocation")
	}
	prompt := llm.calls[0].Prompt
	// Prompt must contain BOTH the plan (for context) and the gap analysis (for scoping)
	if !strings.Contains(prompt, planContent) {
		t.Fatal("prompt missing plan content for context")
	}
	if !strings.Contains(prompt, gapContent) {
		t.Fatal("prompt missing gap analysis content for scoping")
	}
	// Prompt must contain remediation-specific instruction
	if !strings.Contains(prompt, "ONLY create beads") {
		t.Fatal("prompt missing gap-scoping instruction")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/v2/stage/decompose/ -run TestRunUsesGapScopedPromptWhenGapAnalysisProvided -v`
Expected: FAIL — the prompt won't contain the gap content or the scoping instruction.

**Step 3: Add the remediation prompt template**

In `internal/v2/stage/decompose/decompose.go`, add a new prompt template constant:

```go
var remediationDecomposePromptTemplate = `# Remediation Decompose: %s

You are creating TARGETED beads to address specific unmet acceptance criteria. Do NOT re-implement work that already exists.

## Full Plan (for architectural context only)

%s

## Unmet Acceptance Criteria (create beads ONLY for these)

%s

## Skill Instructions

%s

## Output

ONLY create beads that directly address the unmet criteria listed above. Do not create beads for criteria that have already been satisfied.

Output ONLY a JSON array of bead definitions. No markdown, no explanations, no wrapper.
Each bead must include: title, description, priority, acceptance_criteria, expected_outputs, covers_tasks, depends_on_index.

expected_outputs: list each individual deliverable, function, or independently testable item as a separate entry.
covers_tasks: list the 1-based Task numbers from the plan that this bead covers (only tasks related to the unmet criteria).
depends_on_index: array of 0-based indices of prerequisite beads in THIS output array.

The spec label will be added automatically: spec:%s
`
```

**Step 4: Modify Run to use the remediation template when gap analysis is available**

In `decompose.go`, update the `Run` method. After reading `planBody`, add:

```go
	var promptText string
	gapAnalysis := s.resolveGapAnalysis(req)
	if req.Remediation && gapAnalysis != "" {
		promptText = fmt.Sprintf(remediationDecomposePromptTemplate, specID, string(planBody), gapAnalysis, skills.DecomposeSkill, specID)
	} else {
		promptText = fmt.Sprintf(s.promptTemplate, specID, string(planBody), skills.DecomposeSkill, specID)
	}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/v2/stage/decompose/ -run TestRunUsesGapScopedPromptWhenGapAnalysisProvided -v`
Expected: PASS

**Step 6: Run all decompose tests**

Run: `go test ./internal/v2/stage/decompose/ -v`
Expected: All pass. The existing `TestRunUsesGapAnalysisWhenRemediation` should still pass because it sets `Remediation: true` but does NOT set `GapAnalysis`, so the old code path is used.

**Step 7: Commit**

```bash
git add internal/v2/stage/decompose/decompose.go internal/v2/stage/decompose/decompose_test.go
git commit -m "feat: decompose uses gap-scoped prompt template during remediation"
```

---

### Task 4: Add disk fallback — decompose reads gap-analysis.md when GapAnalysis field is empty

**Files:**
- Modify: `internal/v2/stage/decompose/decompose.go`
- Test: `internal/v2/stage/decompose/decompose_test.go`

**Step 1: Write the failing test — decompose reads gap file from disk when field is empty but Remediation=true**

```go
func TestRunReadsGapAnalysisFromDiskWhenFieldEmpty(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	planDir := filepath.Join(tmpDir, ".gromit", "v2")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	planContent := "# Plan\nTask 1: Build thing"
	if err := os.WriteFile(filepath.Join(planDir, "plan.md"), []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	gapContent := "Criterion 5 failed: retries not preserved"
	if err := os.WriteFile(filepath.Join(planDir, "gap-analysis.md"), []byte(gapContent), 0o644); err != nil {
		t.Fatalf("write gap file: %v", err)
	}

	cfg := &config.Config{
		ProjectRoot: tmpDir,
		Paths:       config.PathsConfig{GromitDir: ".gromit"},
	}

	llm := &fakeLLM{
		responses: []*llm.LLMResponse{{Success: true, Output: `[
			{
				"title": "preserve retries",
				"description": "keep retry commits",
				"priority": "P1",
				"acceptance_criteria": ["retries preserved"],
				"expected_outputs": ["separate retry commits"],
				"depends_on_index": []
			}
		]`}},
	}
	tracker := &fakeTracker{}
	stg, err := New(cfg, llm, tracker)
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}

	req := &stagepkg.Request{
		Bead:        stagepkg.BeadInfo{ID: "spec", Labels: []string{"gen:0"}},
		Config:      cfg,
		Remediation: true,
		// GapAnalysis intentionally empty — should fall back to disk
	}

	_, err = stg.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	prompt := llm.calls[0].Prompt
	if !strings.Contains(prompt, gapContent) {
		t.Fatal("prompt missing gap content from disk fallback")
	}
	if !strings.Contains(prompt, "ONLY create beads") {
		t.Fatal("prompt missing gap-scoping instruction from disk fallback")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/v2/stage/decompose/ -run TestRunReadsGapAnalysisFromDiskWhenFieldEmpty -v`
Expected: FAIL — `resolveGapAnalysis` doesn't read from disk yet.

**Step 3: Implement resolveGapAnalysis with disk fallback**

Add to `internal/v2/stage/decompose/decompose.go`:

```go
func (s *Stage) resolveGapAnalysis(req *stagepkg.Request) string {
	if ga := strings.TrimSpace(req.GapAnalysis); ga != "" {
		return ga
	}
	if !req.Remediation {
		return ""
	}
	path := s.gapAnalysisPath(req)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (s *Stage) gapAnalysisPath(req *stagepkg.Request) string {
	cfg := req.Config
	if cfg == nil {
		cfg = s.cfg
	}
	root := strings.TrimSpace(req.Worktree)
	if root == "" && cfg != nil {
		root = cfg.ProjectRoot
	}
	if root == "" {
		root = "."
	}
	gromitDir := defaultGromitDir
	if cfg != nil && cfg.Paths.GromitDir != "" {
		gromitDir = cfg.Paths.GromitDir
	}
	return filepath.Join(root, gromitDir, v2DirName, gapFileName)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/v2/stage/decompose/ -run TestRunReadsGapAnalysisFromDiskWhenFieldEmpty -v`
Expected: PASS

**Step 5: Run all decompose tests**

Run: `go test ./internal/v2/stage/decompose/ -v`
Expected: All pass

**Step 6: Commit**

```bash
git add internal/v2/stage/decompose/decompose.go internal/v2/stage/decompose/decompose_test.go
git commit -m "feat: decompose falls back to gap-analysis.md on disk when GapAnalysis field empty"
```

---

### Task 5: Fix the misleading existing test

**Files:**
- Modify: `internal/v2/stage/decompose/decompose_test.go`

**Step 1: Rename and update `TestRunUsesGapAnalysisWhenRemediation`**

The existing test is misleadingly named — it tests that `Remediation=true` bumps the generation label, not that gap analysis is used. Rename it and add a clarifying comment:

```go
func TestRunIncrementsGenerationWhenRemediation(t *testing.T) {
	// This test verifies that Remediation=true increments the gen label.
	// Gap-scoped prompt behavior is tested in TestRunUsesGapScopedPromptWhenGapAnalysisProvided
	// and TestRunReadsGapAnalysisFromDiskWhenFieldEmpty.
	// ... existing test body unchanged ...
}
```

**Step 2: Run all decompose tests**

Run: `go test ./internal/v2/stage/decompose/ -v`
Expected: All pass

**Step 3: Commit**

```bash
git add internal/v2/stage/decompose/decompose_test.go
git commit -m "fix: rename misleading test to reflect actual behavior (generation increment, not gap analysis)"
```

---

### Task 6: Integration test — remediation runner passes gap analysis end-to-end through decompose

**Files:**
- Modify: `internal/v2/remediation/remediation_test.go`

**Step 1: Write the integration test**

This test verifies the full flow: accept fails with gap summary → remediation runner extracts it → passes to decompose via `req.GapAnalysis` → decompose receives it.

```go
func TestRemediationRunnerGapAnalysisFlowsToDecompose(t *testing.T) {
	t.Parallel()

	gapText := "Criterion 1 failed: no stage commits\nCriterion 3 failed: no event log"

	acceptCalls := 0
	accept := &testStage{
		name: "accept",
		run: func(ctx context.Context, req *stage.Request) (*stage.Result, error) {
			acceptCalls++
			if acceptCalls == 1 {
				artifacts := &acceptArtifactsWithGap{gapSummary: gapText}
				return &stage.Result{Decision: stage.DecisionFail, Artifacts: artifacts}, nil
			}
			return &stage.Result{Decision: stage.DecisionProceed}, nil
		},
	}

	var capturedReq *stage.Request
	decompose := &testStage{
		name: "decompose",
		run: func(ctx context.Context, req *stage.Request) (*stage.Result, error) {
			capturedReq = req
			return &stage.Result{
				Artifacts: &stage.DecomposeArtifacts{
					Beads: []*bead.Bead{{ID: "targeted-bead"}},
				},
			}, nil
		},
	}

	runner := newRunnerForRemediationCycle(accept, decompose, &testBeadRunner{}, 1)
	if err := runner.Run(context.Background(), "spec-gap-flow", ""); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if capturedReq == nil {
		t.Fatal("decompose was not called")
	}
	if capturedReq.GapAnalysis != gapText {
		t.Fatalf("GapAnalysis = %q, want %q", capturedReq.GapAnalysis, gapText)
	}
	if !capturedReq.Remediation {
		t.Fatal("Remediation flag not set")
	}
}

// acceptArtifactsWithGap implements the gapSummaryProvider interface.
type acceptArtifactsWithGap struct {
	gapSummary string
}

func (a *acceptArtifactsWithGap) GetGapSummary() string {
	return a.gapSummary
}
```

**Step 2: Run test to verify it passes (should pass after Task 2)**

Run: `go test ./internal/v2/remediation/ -run TestRemediationRunnerGapAnalysisFlowsToDecompose -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/v2/remediation/remediation_test.go
git commit -m "test: integration test verifying gap analysis flows from accept through remediation to decompose"
```

---

### Task 7: Verification — full test suite and manual inspection

**Step 1: Run all v2 tests**

Run: `go test ./internal/v2/... -v`
Expected: All pass

**Step 2: Run acceptance tests**

Run: `go test ./internal/v2/... -tags acceptance -v`
Expected: All pass

**Step 3: Run full project tests**

Run: `go test ./... 2>&1 | tail -30`
Expected: No regressions

**Step 4: Verify decompose works with gap-analysis and not just a plan**

Manual verification checklist:
- [ ] `TestRunUsesGapScopedPromptWhenGapAnalysisProvided` — proves decompose uses gap-scoped prompt when `GapAnalysis` field is set
- [ ] `TestRunReadsGapAnalysisFromDiskWhenFieldEmpty` — proves decompose falls back to `gap-analysis.md` on disk
- [ ] `TestRunCreatesBeadsFromPlan` — proves normal (non-remediation) decompose still works with plan only
- [ ] `TestRunIncrementsGenerationWhenRemediation` — proves gen label still increments
- [ ] `TestRemediationRunnerGapAnalysisFlowsToDecompose` — proves end-to-end flow from accept → remediation runner → decompose
- [ ] `TestRemediationRunnerPassesGapAnalysisToDecompose` — proves remediation runner extracts gap from accept artifacts
- [ ] Remediation prompt contains plan content (architectural context)
- [ ] Remediation prompt contains gap analysis (scoping)
- [ ] Remediation prompt contains "ONLY create beads" instruction
- [ ] Non-remediation decompose is completely unaffected

**Step 5: Commit (if any fixes needed)**

---

## Summary of Changes

| File | Change |
|------|--------|
| `internal/v2/stage/stage.go` | Add `GapAnalysis string` field to `StageRequest` |
| `internal/v2/stage/accept/accept.go` | Add `GetGapSummary()` method to `AcceptArtifacts` |
| `internal/v2/remediation/remediation.go` | Extract gap summary from accept result, pass to decompose via `req.GapAnalysis` |
| `internal/v2/stage/decompose/decompose.go` | Add `remediationDecomposePromptTemplate`, `resolveGapAnalysis` (struct field + disk fallback), use gap-scoped prompt when available |
| `internal/v2/stage/decompose/decompose_test.go` | Rename misleading test, add gap-scoped prompt test, add disk fallback test |
| `internal/v2/remediation/remediation_test.go` | Add gap extraction test, add end-to-end flow test |
