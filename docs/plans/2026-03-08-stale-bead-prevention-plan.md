# Stale Bead Prevention Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Prevent the run loop from rebuilding beads whose work is already done, by fixing cumulative diff tracking (P0), adding a pre-build satisfaction check in gate (P1), and enforcing behavioral acceptance criteria in decompose (P2).

**Architecture:** Three independent fixes that layer defensively. P0 fixes the accept stage's visibility into cumulative work. P1 adds early detection at gate time. P2 prevents the root cause at decomposition time. Each can be implemented and tested independently.

**Tech Stack:** Go, git CLI, existing LLM provider interface, existing validate package.

**Design doc:** `docs/plans/2026-03-08-stale-bead-prevention-design.md`

---

### Task 1: Add DiffFromBase to GitAdapter interface

**Files:**
- Modify: `internal/v2/adapter/adapter.go:18-27`
- Modify: `internal/v2/adapter/git/exec_git_adapter.go`
- Test: `internal/v2/adapter/git/exec_git_adapter_test.go`

**Step 1: Write the failing test for DiffFromBase**

```go
func TestDiffFromBase_ReturnsCumulativeDiff(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Init a repo, make a base commit, branch, make changes, commit them.
	runGit(t, tmp, "init")
	runGit(t, tmp, "commit", "--allow-empty", "-m", "base")
	baseSHA := strings.TrimSpace(runGitOutput(t, tmp, "rev-parse", "HEAD"))

	writeFile(t, filepath.Join(tmp, "a.go"), "package a\n")
	runGit(t, tmp, "add", "a.go")
	runGit(t, tmp, "commit", "-m", "add a")

	// Write the branch-base file
	baseDir := filepath.Join(tmp, ".gromit", "v2")
	os.MkdirAll(baseDir, 0o755)
	os.WriteFile(filepath.Join(baseDir, "branch-base"), []byte(baseSHA), 0o644)

	adapter := NewExecGitAdapter(tmp, filepath.Join(tmp, "worktrees"))
	diff, err := adapter.DiffFromBase(context.Background(), tmp)
	if err != nil {
		t.Fatalf("DiffFromBase: %v", err)
	}
	if !strings.Contains(diff, "a.go") {
		t.Fatalf("expected diff to contain a.go, got: %s", diff)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/v2/adapter/git/ -run TestDiffFromBase_ReturnsCumulativeDiff -v`
Expected: FAIL — `DiffFromBase` method does not exist.

**Step 3: Write the failing test for fallback**

```go
func TestDiffFromBase_FallsBackToHeadWhenNoBaseFile(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	runGit(t, tmp, "init")
	runGit(t, tmp, "commit", "--allow-empty", "-m", "initial")
	writeFile(t, filepath.Join(tmp, "b.go"), "package b\n")
	// Don't commit — leave as uncommitted change, no branch-base file.

	adapter := NewExecGitAdapter(tmp, filepath.Join(tmp, "worktrees"))
	diff, err := adapter.DiffFromBase(context.Background(), tmp)
	if err != nil {
		t.Fatalf("DiffFromBase fallback: %v", err)
	}
	// Fallback is git diff HEAD which shows uncommitted changes
	if !strings.Contains(diff, "b.go") {
		t.Fatalf("expected fallback diff to contain b.go, got: %s", diff)
	}
}
```

**Step 4: Implement DiffFromBase**

In `internal/v2/adapter/git/exec_git_adapter.go`, add:

```go
const branchBaseFileName = "branch-base"

// DiffFromBase returns the cumulative diff from the stored branch base commit
// to the current worktree state (committed + uncommitted). Falls back to
// git diff HEAD when no branch-base file exists.
func (a *ExecGitAdapter) DiffFromBase(ctx context.Context, worktree string) (string, error) {
	if strings.TrimSpace(worktree) == "" {
		return "", fmt.Errorf("worktree required")
	}

	basePath := filepath.Join(worktree, ".gromit", "v2", branchBaseFileName)
	baseData, err := os.ReadFile(basePath)
	if err != nil {
		// Fallback: no base file, use git diff HEAD
		return a.Diff(ctx, worktree)
	}

	baseSHA := strings.TrimSpace(string(baseData))
	if baseSHA == "" {
		return a.Diff(ctx, worktree)
	}

	// Diff from base to HEAD (committed changes) + uncommitted changes
	// Use git diff <base> to capture both
	cmd := exec.CommandContext(ctx, "git", "diff", baseSHA)
	cmd.Dir = worktree
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff from base: %s: %w", out, err)
	}
	return string(out), nil
}
```

**Step 5: Add DiffFromBase to the GitAdapter interface**

In `internal/v2/adapter/adapter.go`, add to the `GitAdapter` interface:

```go
DiffFromBase(ctx context.Context, worktree string) (string, error)
```

**Step 6: Run tests to verify they pass**

Run: `go test ./internal/v2/adapter/git/ -run TestDiffFromBase -v`
Expected: PASS

**Step 7: Commit**

```bash
git add internal/v2/adapter/adapter.go internal/v2/adapter/git/exec_git_adapter.go internal/v2/adapter/git/exec_git_adapter_test.go
git commit -m "feat: add DiffFromBase to GitAdapter for cumulative diff from branch base"
```

---

### Task 2: Store branch base SHA on worktree creation

**Files:**
- Modify: `internal/v2/adapter/git/exec_git_adapter.go:35-65`
- Test: `internal/v2/adapter/git/exec_git_adapter_test.go`

**Step 1: Write the failing test**

```go
func TestCheckout_WritesBranchBaseFile(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "repo")
	wtDir := filepath.Join(tmp, "worktrees")

	os.MkdirAll(repoDir, 0o755)
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial")
	expectedBase := strings.TrimSpace(runGitOutput(t, repoDir, "rev-parse", "HEAD"))

	adapter := NewExecGitAdapter(repoDir, wtDir)
	wtPath, err := adapter.Checkout(context.Background(), "test-spec")
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	basePath := filepath.Join(wtPath, ".gromit", "v2", "branch-base")
	data, err := os.ReadFile(basePath)
	if err != nil {
		t.Fatalf("read branch-base: %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != expectedBase {
		t.Fatalf("branch-base = %q, want %q", got, expectedBase)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/v2/adapter/git/ -run TestCheckout_WritesBranchBaseFile -v`
Expected: FAIL — branch-base file not written.

**Step 3: Implement — write base SHA after worktree creation**

In `internal/v2/adapter/git/exec_git_adapter.go`, in the `Checkout` method, after the `git worktree add` command succeeds and before the `filepath.Abs` call, add:

```go
	// Record the base commit SHA so DiffFromBase can compute cumulative diffs.
	baseOut, baseErr := runGitCommand(ctx, a.repoRoot, "rev-parse", "HEAD")
	if baseErr == nil {
		baseDir := filepath.Join(wtPath, ".gromit", "v2")
		if mkErr := os.MkdirAll(baseDir, 0o755); mkErr == nil {
			_ = os.WriteFile(filepath.Join(baseDir, branchBaseFileName), []byte(strings.TrimSpace(string(baseOut))), 0o644)
		}
	}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/v2/adapter/git/ -run TestCheckout_WritesBranchBaseFile -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/v2/adapter/git/exec_git_adapter.go internal/v2/adapter/git/exec_git_adapter_test.go
git commit -m "feat: store branch-base SHA on worktree creation for cumulative diffing"
```

---

### Task 3: Accept stage uses DiffFromBase

**Files:**
- Modify: `internal/v2/stage/accept/accept.go:53-55,154`
- Modify: `internal/v2/stage/accept/accept_test.go`

**Step 1: Update the GitDiffer interface in accept to include DiffFromBase**

In `internal/v2/stage/accept/accept.go`, update the `GitDiffer` interface:

```go
type GitDiffer interface {
	Diff(ctx context.Context, worktree string) (string, error)
	DiffFromBase(ctx context.Context, worktree string) (string, error)
}
```

**Step 2: Write the failing test**

```go
func TestRunUsesDiffFromBase(t *testing.T) {
	t.Parallel()

	specID := "spec-cumulative"
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, ".gromit", "specs")
	os.MkdirAll(specDir, 0o755)
	specPath := filepath.Join(specDir, specID+".md")
	content := "# Cumulative spec\n\n## Acceptance Criteria\n- feature works\n"
	os.WriteFile(specPath, []byte(content), 0o644)

	cfg := &config.Config{
		ProjectRoot: tmp,
		Paths:       config.PathsConfig{GromitDir: ".gromit", Specs: ".gromit/specs"},
	}

	git := &fakeGitAdapter{diff: "old uncommitted diff", diffFromBase: "cumulative diff with feature"}
	llmProvider := &fakeLLM{
		responses: []*llm.LLMResponse{
			{Success: true, Output: `{"pass": true, "summary": "feature present in cumulative diff"}`},
		},
	}

	stageInstance, _ := New(cfg, git, llmProvider, "", "", "")
	req := &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: specID}, Worktree: tmp}
	res, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("decision = %v, want proceed", res.Decision)
	}
	// Verify DiffFromBase was called, not Diff
	if !git.diffFromBaseCalled {
		t.Fatal("expected DiffFromBase to be called")
	}
	if git.diffCalled {
		t.Fatal("expected Diff NOT to be called")
	}
}
```

Update `fakeGitAdapter` to track which method was called:

```go
type fakeGitAdapter struct {
	diff               string
	diffFromBase       string
	diffCalled         bool
	diffFromBaseCalled bool
	lastWorktree       string
}

func (f *fakeGitAdapter) Diff(ctx context.Context, worktree string) (string, error) {
	f.diffCalled = true
	f.lastWorktree = worktree
	return f.diff, nil
}

func (f *fakeGitAdapter) DiffFromBase(ctx context.Context, worktree string) (string, error) {
	f.diffFromBaseCalled = true
	f.lastWorktree = worktree
	if f.diffFromBase != "" {
		return f.diffFromBase, nil
	}
	return f.diff, nil
}
```

**Step 3: Run test to verify it fails**

Run: `go test ./internal/v2/stage/accept/ -run TestRunUsesDiffFromBase -v`
Expected: FAIL — accept still calls `Diff`, not `DiffFromBase`.

**Step 4: Implement — change accept.go line 154**

Replace:
```go
	diff, err := s.git.Diff(ctx, root)
```
With:
```go
	diff, err := s.git.DiffFromBase(ctx, root)
```

**Step 5: Run tests to verify all pass**

Run: `go test ./internal/v2/stage/accept/ -v`
Expected: PASS (all existing tests should pass since fakeGitAdapter now implements both methods; DiffFromBase falls back to diff field when diffFromBase is empty).

**Step 6: Commit**

```bash
git add internal/v2/stage/accept/accept.go internal/v2/stage/accept/accept_test.go
git commit -m "feat: accept stage uses DiffFromBase for cumulative diff visibility"
```

---

### Task 4: Fix all GitAdapter implementations to satisfy updated interface

**Files:**
- Find and update all types that implement `GitAdapter` or `GitDiffer` across the codebase

**Step 1: Find all implementations**

Run: `grep -rn "func.*Diff(ctx context.Context" internal/v2/ --include="*.go"` to find all types implementing `Diff`. Each needs a corresponding `DiffFromBase`.

For test fakes that only implement `Diff`, add a `DiffFromBase` that delegates to `Diff` (same fallback behavior as the real adapter).

**Step 2: Update each implementation**

For each fake/mock (e.g., in `review/review_test.go`, `accept/accept_test.go`, etc.), add:

```go
func (f *fakeGitAdapter) DiffFromBase(ctx context.Context, worktree string) (string, error) {
	return f.Diff(ctx, worktree)
}
```

**Step 3: Run full test suite**

Run: `go test ./internal/v2/... -v`
Expected: PASS — all tests compile and pass.

**Step 4: Commit**

```bash
git add -A
git commit -m "fix: update all GitAdapter/GitDiffer implementations for DiffFromBase"
```

---

### Task 5: Gate satisfaction check — generation-based tier selection

**Files:**
- Create: `internal/v2/stage/gate/satisfaction.go`
- Test: `internal/v2/stage/gate/satisfaction_test.go`

**Step 1: Write the failing tests**

```go
func TestSatisfactionTier_Gen0SkipsCheck(t *testing.T) {
	t.Parallel()
	tier := satisfactionTier(0)
	if tier != "" {
		t.Fatalf("gen 0 tier = %q, want empty (skip)", tier)
	}
}

func TestSatisfactionTier_Gen1ReturnsLow(t *testing.T) {
	t.Parallel()
	tier := satisfactionTier(1)
	if tier != "low" {
		t.Fatalf("gen 1 tier = %q, want low", tier)
	}
}

func TestSatisfactionTier_Gen2ReturnsMedium(t *testing.T) {
	t.Parallel()
	tier := satisfactionTier(2)
	if tier != "medium" {
		t.Fatalf("gen 2 tier = %q, want medium", tier)
	}
}

func TestSatisfactionTier_Gen3ReturnsHigh(t *testing.T) {
	t.Parallel()
	tier := satisfactionTier(3)
	if tier != "high" {
		t.Fatalf("gen 3 tier = %q, want high", tier)
	}
}

func TestSatisfactionTier_Gen5ReturnsHigh(t *testing.T) {
	t.Parallel()
	tier := satisfactionTier(5)
	if tier != "high" {
		t.Fatalf("gen 5 tier = %q, want high", tier)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/v2/stage/gate/ -run TestSatisfactionTier -v`
Expected: FAIL — function does not exist.

**Step 3: Implement satisfactionTier**

In `internal/v2/stage/gate/satisfaction.go`:

```go
package gate

// satisfactionTier returns the LLM tier for the pre-build satisfaction check
// based on bead generation. Gen 0 returns "" (skip check). Gen 1 uses low
// (haiku), gen 2 medium (sonnet), gen 3+ high (opus).
func satisfactionTier(generation int) string {
	switch {
	case generation <= 0:
		return ""
	case generation == 1:
		return "low"
	case generation == 2:
		return "medium"
	default:
		return "high"
	}
}
```

**Step 4: Run tests**

Run: `go test ./internal/v2/stage/gate/ -run TestSatisfactionTier -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/v2/stage/gate/satisfaction.go internal/v2/stage/gate/satisfaction_test.go
git commit -m "feat: generation-based tier selection for gate satisfaction check"
```

---

### Task 6: Gate satisfaction check — refactor/test bead guard

**Files:**
- Modify: `internal/v2/stage/gate/satisfaction.go`
- Modify: `internal/v2/stage/gate/satisfaction_test.go`

**Step 1: Write the failing tests**

```go
func TestIsStructuralBead_RefactorTitle(t *testing.T) {
	t.Parallel()
	if !isStructuralBead("Refactor debug command into separate modules", "") {
		t.Fatal("expected refactor bead to be detected")
	}
}

func TestIsStructuralBead_TestTitle(t *testing.T) {
	t.Parallel()
	if !isStructuralBead("Add test coverage for router", "") {
		t.Fatal("expected test bead to be detected")
	}
}

func TestIsStructuralBead_ReorganizeDescription(t *testing.T) {
	t.Parallel()
	if !isStructuralBead("Clean up modules", "reorganize the debug package") {
		t.Fatal("expected reorganize bead to be detected")
	}
}

func TestIsStructuralBead_NormalBead(t *testing.T) {
	t.Parallel()
	if isStructuralBead("Implement debug command entry point", "diagnose root cause from event log") {
		t.Fatal("expected normal bead to NOT be detected as structural")
	}
}

func TestIsStructuralBead_ExtractTitle(t *testing.T) {
	t.Parallel()
	if !isStructuralBead("Extract validation logic into helper", "") {
		t.Fatal("expected extract bead to be detected")
	}
}

func TestIsStructuralBead_MoveTitle(t *testing.T) {
	t.Parallel()
	if !isStructuralBead("Move types to shared package", "") {
		t.Fatal("expected move bead to be detected")
	}
}

func TestIsStructuralBead_RenameTitle(t *testing.T) {
	t.Parallel()
	if !isStructuralBead("Rename adapter methods for consistency", "") {
		t.Fatal("expected rename bead to be detected")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/v2/stage/gate/ -run TestIsStructuralBead -v`
Expected: FAIL

**Step 3: Implement isStructuralBead**

In `internal/v2/stage/gate/satisfaction.go`, add:

```go
import "strings"

// structuralKeywords identifies beads that change code structure without
// changing observable behavior. These beads skip satisfaction checks because
// a "does this behavior exist?" check would false-positive.
var structuralKeywords = []string{
	"refactor",
	"reorganize",
	"extract",
	"move",
	"rename",
	"add test",
	"test coverage",
	"integration test",
}

// isStructuralBead returns true if the bead's title or description indicates
// a refactoring or test-only change that should bypass satisfaction checks.
func isStructuralBead(title, description string) bool {
	combined := strings.ToLower(title + " " + description)
	for _, kw := range structuralKeywords {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}
```

**Step 4: Run tests**

Run: `go test ./internal/v2/stage/gate/ -run TestIsStructuralBead -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/v2/stage/gate/satisfaction.go internal/v2/stage/gate/satisfaction_test.go
git commit -m "feat: structural bead guard for gate satisfaction check"
```

---

### Task 7: Gate satisfaction check — LLM evaluation logic

**Files:**
- Modify: `internal/v2/stage/gate/satisfaction.go`
- Modify: `internal/v2/stage/gate/satisfaction_test.go`

**Step 1: Write the failing tests**

```go
func TestCheckSatisfaction_AllCriteriaPass_ReturnsTrue(t *testing.T) {
	t.Parallel()
	llm := &fakeLLM{
		responses: []string{
			`{"pass": true, "summary": "already implemented"}`,
			`{"pass": true, "summary": "present in diff"}`,
		},
	}
	satisfied, err := checkSatisfaction(context.Background(), llm, "low", "cumulative diff here", "bead-1", []string{"feature A works", "feature B works"})
	if err != nil {
		t.Fatalf("checkSatisfaction: %v", err)
	}
	if !satisfied {
		t.Fatal("expected satisfied=true when all criteria pass")
	}
}

func TestCheckSatisfaction_AnyCriterionFails_ReturnsFalse(t *testing.T) {
	t.Parallel()
	llm := &fakeLLM{
		responses: []string{
			`{"pass": true, "summary": "ok"}`,
			`{"pass": false, "summary": "not found"}`,
		},
	}
	satisfied, err := checkSatisfaction(context.Background(), llm, "low", "cumulative diff", "bead-1", []string{"A works", "B works"})
	if err != nil {
		t.Fatalf("checkSatisfaction: %v", err)
	}
	if satisfied {
		t.Fatal("expected satisfied=false when any criterion fails")
	}
}

func TestCheckSatisfaction_NoCriteria_ReturnsFalse(t *testing.T) {
	t.Parallel()
	satisfied, err := checkSatisfaction(context.Background(), nil, "low", "diff", "bead-1", nil)
	if err != nil {
		t.Fatalf("checkSatisfaction: %v", err)
	}
	if satisfied {
		t.Fatal("expected satisfied=false when no criteria provided")
	}
}
```

Add `fakeLLM` to gate tests (similar to accept's):

```go
type fakeLLM struct {
	responses []string
	calls     int
}

func (f *fakeLLM) Invoke(ctx context.Context, req llmtypes.LLMInvokeRequest) (*llmtypes.LLMResponse, error) {
	if f.calls >= len(f.responses) {
		return nil, fmt.Errorf("no more responses")
	}
	resp := f.responses[f.calls]
	f.calls++
	return &llmtypes.LLMResponse{Success: true, Output: resp}, nil
}

func (f *fakeLLM) StreamInvoke(ctx context.Context, req llmtypes.StreamInvokeRequest) (*llmtypes.LLMResponse, error) {
	return nil, fmt.Errorf("not supported")
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/v2/stage/gate/ -run TestCheckSatisfaction -v`
Expected: FAIL

**Step 3: Implement checkSatisfaction**

In `internal/v2/stage/gate/satisfaction.go`, add:

```go
import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/danabrams/gromit/internal/v2/llmtypes"
)

const satisfactionPromptTemplate = `You are evaluating whether a bead's acceptance criteria are ALREADY satisfied by existing code changes.

## Bead ID: %s

## Criterion to evaluate:
%s

## Cumulative diff (all changes on this branch):
%s

## Instructions
Evaluate whether this criterion is ALREADY satisfied by the code in the diff.
Output ONLY a JSON object: {"pass": true/false, "summary": "brief reason"}
`

// checkSatisfaction evaluates each acceptance criterion against the cumulative
// diff. Returns true only if ALL criteria pass (bead is fully satisfied).
// Returns false with nil error when no criteria are provided.
func checkSatisfaction(ctx context.Context, llm llmtypes.LLMProvider, tier, diff, beadID string, criteria []string) (bool, error) {
	if len(criteria) == 0 {
		return false, nil
	}

	for _, criterion := range criteria {
		prompt := fmt.Sprintf(satisfactionPromptTemplate, beadID, criterion, diff)
		resp, err := llm.Invoke(ctx, llmtypes.LLMInvokeRequest{
			Prompt: prompt,
			Model:  tier,
		})
		if err != nil {
			return false, fmt.Errorf("satisfaction check for %s: %w", beadID, err)
		}

		var eval struct {
			Pass    bool   `json:"pass"`
			Summary string `json:"summary"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Output)), &eval); err != nil {
			// Try extracting JSON from wrapped output
			trimmed := strings.TrimSpace(resp.Output)
			start := strings.Index(trimmed, "{")
			end := strings.LastIndex(trimmed, "}")
			if start >= 0 && end > start {
				if err2 := json.Unmarshal([]byte(trimmed[start:end+1]), &eval); err2 != nil {
					return false, fmt.Errorf("parse satisfaction response: %w", err2)
				}
			} else {
				return false, fmt.Errorf("parse satisfaction response: %w", err)
			}
		}

		if !eval.Pass {
			return false, nil
		}
	}

	return true, nil
}
```

**Step 4: Run tests**

Run: `go test ./internal/v2/stage/gate/ -run TestCheckSatisfaction -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/v2/stage/gate/satisfaction.go internal/v2/stage/gate/satisfaction_test.go
git commit -m "feat: LLM-based satisfaction evaluation for gate stage"
```

---

### Task 8: Wire satisfaction check into Gate.Run

**Files:**
- Modify: `internal/v2/stage/gate/gate.go`
- Modify: `internal/v2/stage/gate/gate_test.go`

**Step 1: Write the failing tests**

```go
func TestGateSatisfactionCheck_Gen0_SkipsCheck(t *testing.T) {
	t.Parallel()
	tracker := &fakeTaskTracker{
		beads: map[string]*tasktracker.Bead{
			"bead-gen0": {ID: "bead-gen0", Status: "open", Labels: []string{"gen:0"}},
		},
	}
	llm := &fakeLLM{responses: []string{`{"pass": true, "summary": "done"}`}}
	git := &fakeGitDiffer{diff: "some diff"}

	stageInstance, _ := New(&config.Config{}, tracker, WithSatisfactionCheck(llm, git))
	res, err := stageInstance.Run(context.Background(), &stagepkg.Request{
		Bead:     stagepkg.BeadInfo{ID: "bead-gen0", Labels: []string{"gen:0"}},
		Worktree: "/tmp/wt",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("gen 0 should proceed, got %v", res.Decision)
	}
	if llm.calls > 0 {
		t.Fatal("gen 0 should NOT invoke LLM")
	}
}

func TestGateSatisfactionCheck_Gen1_SatisfiedBeadSkipped(t *testing.T) {
	t.Parallel()
	tracker := &fakeTaskTracker{
		beads: map[string]*tasktracker.Bead{
			"bead-sat": {
				ID:     "bead-sat",
				Status: "open",
				Labels: []string{"gen:1"},
				// Description contains acceptance criteria
			},
		},
	}
	llm := &fakeLLM{responses: []string{`{"pass": true, "summary": "already done"}`}}
	git := &fakeGitDiffer{diff: "cumulative diff with feature"}

	stageInstance, _ := New(&config.Config{}, tracker, WithSatisfactionCheck(llm, git))
	res, err := stageInstance.Run(context.Background(), &stagepkg.Request{
		Bead: stagepkg.BeadInfo{
			ID:          "bead-sat",
			Labels:      []string{"gen:1"},
			Description: "## Acceptance Criteria\n- feature exists",
		},
		Worktree: "/tmp/wt",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Decision != stagepkg.DecisionSkip {
		t.Fatalf("satisfied bead should be skipped, got %v", res.Decision)
	}
}

func TestGateSatisfactionCheck_NilLLM_SkipsCheck(t *testing.T) {
	t.Parallel()
	tracker := &fakeTaskTracker{
		beads: map[string]*tasktracker.Bead{
			"bead-no-llm": {ID: "bead-no-llm", Status: "open", Labels: []string{"gen:1"}},
		},
	}
	// No WithSatisfactionCheck — backwards compat
	stageInstance, _ := New(&config.Config{}, tracker)
	res, err := stageInstance.Run(context.Background(), &stagepkg.Request{
		Bead: stagepkg.BeadInfo{ID: "bead-no-llm", Labels: []string{"gen:1"}},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("no LLM should proceed, got %v", res.Decision)
	}
}

func TestGateSatisfactionCheck_RefactorBead_SkipsCheck(t *testing.T) {
	t.Parallel()
	tracker := &fakeTaskTracker{
		beads: map[string]*tasktracker.Bead{
			"bead-refactor": {ID: "bead-refactor", Status: "open", Labels: []string{"gen:1"}},
		},
	}
	llm := &fakeLLM{responses: []string{`{"pass": true, "summary": "done"}`}}
	git := &fakeGitDiffer{diff: "some diff"}

	stageInstance, _ := New(&config.Config{}, tracker, WithSatisfactionCheck(llm, git))
	res, err := stageInstance.Run(context.Background(), &stagepkg.Request{
		Bead: stagepkg.BeadInfo{
			ID:     "bead-refactor",
			Title:  "Refactor debug module",
			Labels: []string{"gen:1"},
		},
		Worktree: "/tmp/wt",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("refactor bead should proceed without check, got %v", res.Decision)
	}
	if llm.calls > 0 {
		t.Fatal("refactor bead should NOT invoke LLM")
	}
}
```

Add `fakeGitDiffer`:

```go
type fakeGitDiffer struct {
	diff string
}

func (f *fakeGitDiffer) DiffFromBase(ctx context.Context, worktree string) (string, error) {
	return f.diff, nil
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/v2/stage/gate/ -run TestGateSatisfactionCheck -v`
Expected: FAIL — `WithSatisfactionCheck` option doesn't exist.

**Step 3: Implement — update Gate struct and constructor**

In `internal/v2/stage/gate/gate.go`:

```go
import (
	"github.com/danabrams/gromit/internal/v2/generation"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
)

// SatisfactionDiffer provides cumulative diffs for satisfaction checks.
type SatisfactionDiffer interface {
	DiffFromBase(ctx context.Context, worktree string) (string, error)
}

type Stage struct {
	name    string
	tracker trackertypes.TaskTracker
	llm     llmtypes.LLMProvider    // optional: enables satisfaction check
	git     SatisfactionDiffer       // optional: provides cumulative diff
}

// Option configures optional gate stage parameters.
type Option func(*Stage)

// WithSatisfactionCheck enables pre-build satisfaction checking using the
// provided LLM and git adapter.
func WithSatisfactionCheck(llm llmtypes.LLMProvider, git SatisfactionDiffer) Option {
	return func(s *Stage) {
		s.llm = llm
		s.git = git
	}
}

func New(cfg *config.Config, tracker trackertypes.TaskTracker, opts ...Option) (*Stage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	if tracker == nil {
		return nil, fmt.Errorf("task tracker required")
	}
	s := &Stage{name: stagedesc.Describe("gate", cfg), tracker: tracker}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}
```

Update `Run` to add satisfaction check after dependency check passes:

```go
func (s *Stage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	if req == nil {
		return nil, fmt.Errorf("request required")
	}

	beadID := strings.TrimSpace(req.Bead.ID)
	if beadID == "" {
		return nil, fmt.Errorf("bead ID required")
	}

	b, err := s.tracker.ShowBead(ctx, beadID)
	if err != nil {
		return nil, fmt.Errorf("gate: show bead: %w", err)
	}
	if b == nil {
		return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
	}

	if isClosed(b.Status) {
		return &stagepkg.Result{Decision: stagepkg.DecisionSkip}, nil
	}

	if pending, err := s.hasPendingDependencies(ctx, b); err != nil {
		return nil, err
	} else if pending {
		return &stagepkg.Result{Decision: stagepkg.DecisionBlock}, nil
	}

	// Pre-build satisfaction check (P1)
	if satisfied, err := s.trySatisfactionCheck(ctx, req); err != nil {
		return nil, fmt.Errorf("gate: satisfaction check: %w", err)
	} else if satisfied {
		// Close the bead since its work is already done
		_, _ = s.tracker.CloseBead(ctx, trackertypes.TaskTrackerCloseBeadRequest{BeadID: beadID})
		return &stagepkg.Result{Decision: stagepkg.DecisionSkip}, nil
	}

	return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
}

// trySatisfactionCheck runs the pre-build satisfaction check if conditions
// are met: LLM and git are configured, generation > 0, bead is not structural.
func (s *Stage) trySatisfactionCheck(ctx context.Context, req *stagepkg.Request) (bool, error) {
	if s.llm == nil || s.git == nil {
		return false, nil
	}

	gen := generation.Current(req.Bead.Labels)
	tier := satisfactionTier(gen)
	if tier == "" {
		return false, nil
	}

	if isStructuralBead(req.Bead.Title, req.Bead.Description) {
		return false, nil
	}

	worktree := strings.TrimSpace(req.Worktree)
	if worktree == "" {
		return false, nil
	}

	diff, err := s.git.DiffFromBase(ctx, worktree)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(diff) == "" {
		return false, nil
	}

	criteria := extractCriteria(req.Bead.Description)
	return checkSatisfaction(ctx, s.llm, tier, diff, req.Bead.ID, criteria)
}

// extractCriteria parses acceptance criteria from a bead description.
// Looks for lines starting with "- " after an "Acceptance Criteria" header.
func extractCriteria(description string) []string {
	var criteria []string
	inCriteria := false
	for _, line := range strings.Split(description, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(strings.ToLower(trimmed), "acceptance criteria") {
			inCriteria = true
			continue
		}
		if inCriteria {
			if strings.HasPrefix(trimmed, "- ") {
				criteria = append(criteria, strings.TrimPrefix(trimmed, "- "))
			} else if trimmed == "" {
				continue
			} else if strings.HasPrefix(trimmed, "#") {
				break // next section
			}
		}
	}
	return criteria
}
```

**Step 4: Run tests**

Run: `go test ./internal/v2/stage/gate/ -v`
Expected: PASS (all old tests and new tests).

**Step 5: Commit**

```bash
git add internal/v2/stage/gate/gate.go internal/v2/stage/gate/gate_test.go internal/v2/stage/gate/satisfaction.go internal/v2/stage/gate/satisfaction_test.go
git commit -m "feat: wire satisfaction check into gate with generation-based tiers and structural guards"
```

---

### Task 9: Update decompose prompt for behavioral criteria (P2)

**Files:**
- Modify: `internal/v2/stage/decompose/decompose.go:35-87`
- Test: `internal/v2/stage/decompose/decompose_test.go`

**Step 1: Write the failing test**

```go
func TestDecomposePromptContainsBehavioralCriteriaInstruction(t *testing.T) {
	t.Parallel()
	if !strings.Contains(defaultDecomposePromptTemplate, "observable behavior") {
		t.Fatal("default decompose prompt must instruct behavioral acceptance criteria")
	}
	if !strings.Contains(defaultDecomposePromptTemplate, "NOT a file path") {
		t.Fatal("default decompose prompt must warn against file-path criteria")
	}
}

func TestRemediationDecomposePromptContainsBehavioralCriteriaInstruction(t *testing.T) {
	t.Parallel()
	if !strings.Contains(remediationDecomposePromptTemplate, "observable behavior") {
		t.Fatal("remediation decompose prompt must instruct behavioral acceptance criteria")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/v2/stage/decompose/ -run TestDecomposePrompt.*Behavioral -v`
Expected: FAIL

**Step 3: Update both prompt templates**

In `defaultDecomposePromptTemplate`, add before the `## Output` section:

```
## Acceptance Criteria Rules

acceptance_criteria: each criterion MUST describe an observable behavior or capability, NOT a file path, function name, or code structure.

Good: "debug command identifies root cause category from event log"
Bad: "create internal/v2/debug/diagnose.go with Diagnose() function"

The implementation may consolidate or restructure deliverables — criteria must remain valid regardless of how the code is organized.
```

Add the same block to `remediationDecomposePromptTemplate` before its `## Output` section.

**Step 4: Run tests**

Run: `go test ./internal/v2/stage/decompose/ -run TestDecomposePrompt.*Behavioral -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/v2/stage/decompose/decompose.go internal/v2/stage/decompose/decompose_test.go
git commit -m "feat: require behavioral acceptance criteria in decompose prompts"
```

---

### Task 10: Add validation rule for file-path criteria

**Files:**
- Modify: `internal/validate/validate.go`
- Modify: `internal/validate/validate_test.go`

**Step 1: Write the failing tests**

```go
func TestCheckBeads_FlagsFilePathInCriteria(t *testing.T) {
	t.Parallel()
	beads := []BeadCandidate{
		{
			Title:              "Build feature",
			AcceptanceCriteria: []string{"create internal/v2/debug/diagnose.go with Diagnose() function"},
			ExpectedOutputs:    []string{"Diagnose function"},
		},
	}
	violations := CheckBeads(beads)
	found := false
	for _, v := range violations {
		if v.Rule == "criteria_structural" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected criteria_structural violation for file-path criterion")
	}
}

func TestCheckBeads_AllowsBehavioralCriteria(t *testing.T) {
	t.Parallel()
	beads := []BeadCandidate{
		{
			Title:              "Build feature",
			AcceptanceCriteria: []string{"debug command identifies root cause category from event log"},
			ExpectedOutputs:    []string{"Diagnose function"},
		},
	}
	violations := CheckBeads(beads)
	for _, v := range violations {
		if v.Rule == "criteria_structural" {
			t.Fatalf("unexpected criteria_structural violation for behavioral criterion")
		}
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/validate/ -run TestCheckBeads_FlagsFilePath -v`
Expected: FAIL

**Step 3: Implement — add file-path detection to CheckBeads**

In `internal/validate/validate.go`, add after the sibling overlap check in `CheckBeads`:

```go
	// Check for structural/file-path acceptance criteria
	filePathExtensions := []string{".go", ".ts", ".js", ".py", ".rs", ".java", ".yaml", ".yml", ".json", ".toml"}

	for i, bead := range beads {
		for _, criterion := range bead.AcceptanceCriteria {
			if containsFilePath(criterion, filePathExtensions) {
				violations = append(violations, Violation{
					BeadIndex: i,
					Rule:      "criteria_structural",
					Message:   "Acceptance criterion references file paths or code structure; use observable behavior instead",
				})
				break
			}
		}
	}
```

Add the helper:

```go
// containsFilePath checks if text contains file path indicators.
func containsFilePath(text string, extensions []string) bool {
	lower := strings.ToLower(text)
	// Check for path separators with extensions
	for _, ext := range extensions {
		if strings.Contains(lower, "/") && strings.Contains(lower, ext) {
			return true
		}
	}
	return false
}
```

**Step 4: Run tests**

Run: `go test ./internal/validate/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/validate/validate.go internal/validate/validate_test.go
git commit -m "feat: validate rule to flag structural file-path acceptance criteria"
```

---

### Task 11: Update v2 run loop appendix

**Files:**
- Find the v2 run loop design doc and add appendix entry

**Step 1: Add appendix to `docs/plans/2026-03-04-v2-run-loop-clean-implementation-design.md`**

Append:

```markdown

## Appendix: Stale Bead Prevention (2026-03-08)

Three fixes prevent the run loop from rebuilding beads whose work is already done:

1. **Cumulative diff (P0):** Accept stage uses `DiffFromBase` instead of `git diff HEAD`. Branch base SHA stored in `.gromit/v2/branch-base` at worktree creation. Diffs capture all committed + uncommitted changes since the branch point.

2. **Gate satisfaction check (P1):** Before proceeding, gate evaluates bead acceptance criteria against the cumulative diff via LLM. Tier escalates by generation: gen0=skip, gen1=haiku, gen2=sonnet, gen3+=opus. Structural beads (refactor, test, rename, etc.) bypass the check to avoid false positives.

3. **Behavioral criteria (P2):** Decompose prompts require acceptance criteria to describe observable behavior, not file paths or code structure. Validation rule flags criteria containing file paths as `criteria_structural` violations.

Design doc: `docs/plans/2026-03-08-stale-bead-prevention-design.md`
```

**Step 2: Commit**

```bash
git add docs/plans/2026-03-04-v2-run-loop-clean-implementation-design.md
git commit -m "docs: add stale bead prevention appendix to v2 run loop design"
```

---

### Task 12: Integration test — full stale bead scenario

**Files:**
- Create: `internal/v2/stage/gate/gate_integration_test.go`

**Step 1: Write the integration test**

```go
//go:build integration

func TestIntegration_GateClosesAlreadySatisfiedBead(t *testing.T) {
	t.Parallel()

	// Simulate: bead at gen:1 whose acceptance criteria are satisfied
	// by existing cumulative diff.
	tracker := &fakeTaskTracker{
		beads: map[string]*tasktracker.Bead{
			"stale-bead": {
				ID:     "stale-bead",
				Status: "open",
				Labels: []string{"gen:1", "spec:test-spec"},
			},
		},
		closedBeads: make(map[string]bool),
	}

	llm := &fakeLLM{
		responses: []string{`{"pass": true, "summary": "debug command already exists"}`},
	}
	git := &fakeGitDiffer{diff: "diff showing debug2.go implementation"}

	stageInstance, _ := New(&config.Config{}, tracker, WithSatisfactionCheck(llm, git))

	req := &stagepkg.Request{
		Bead: stagepkg.BeadInfo{
			ID:          "stale-bead",
			Title:       "Implement debug command entry point",
			Description: "## Acceptance Criteria\n- debug command can diagnose failures from event log",
			Labels:      []string{"gen:1"},
		},
		Worktree: "/tmp/wt",
	}

	res, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if res.Decision != stagepkg.DecisionSkip {
		t.Fatalf("stale bead should be skipped, got %v", res.Decision)
	}

	if !tracker.closedBeads["stale-bead"] {
		t.Fatal("expected stale bead to be closed via tracker")
	}
}
```

Update `fakeTaskTracker` to track closes:

```go
type fakeTaskTracker struct {
	beads       map[string]*tasktracker.Bead
	closedBeads map[string]bool
}

func (f *fakeTaskTracker) CloseBead(_ context.Context, req tasktracker.CloseBeadRequest) (*tasktracker.CloseBeadResponse, error) {
	if f.closedBeads != nil {
		f.closedBeads[req.BeadID] = true
	}
	return &tasktracker.CloseBeadResponse{Closed: true}, nil
}
```

**Step 2: Run integration test**

Run: `go test ./internal/v2/stage/gate/ -run TestIntegration_GateClosesAlreadySatisfiedBead -tags=integration -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/v2/stage/gate/gate_integration_test.go
git commit -m "test: integration test for gate closing already-satisfied beads"
```

---

Plan complete and saved to `docs/plans/2026-03-08-stale-bead-prevention-plan.md`. Two execution options:

**1. Subagent-Driven (this session)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** — Open new session with executing-plans, batch execution with checkpoints

Which approach?