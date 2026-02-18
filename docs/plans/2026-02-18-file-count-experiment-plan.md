# File Count Experiment Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Capture `files_touched` per iteration so the retro can analyze first-pass success rate by file-count bucket (1-3, 4-6, 7+).

**Architecture:** Add a `FilesTouched` field to the iteration result, log, metric, and efficiency structs. Compute it via `ParseDiffFiles()` (already exists) after each bead completes. Surface it in the retro template's per-iteration efficiency table.

**Tech Stack:** Go, existing `methodology.ParseDiffFiles()`, existing `Runner.getDiff()`.

---

### Task 1: Add FilesTouched to IterationResult

**Files:**
- Modify: `internal/runner/runtypes/types.go:98` (after `FirstPassSuccess`)
- Test: `internal/runner/runtypes/types_test.go`

**Step 1: Write the failing test**

Add a test that constructs a `BeadContext` with `FilesTouched` set on the result and asserts it round-trips.

```go
func TestIterationResult_FilesTouched(t *testing.T) {
	result := IterationResult{
		BeadID:       "test-1",
		FilesTouched: 3,
	}
	if result.FilesTouched != 3 {
		t.Errorf("FilesTouched = %d, want 3", result.FilesTouched)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/runner/runtypes/ -run TestIterationResult_FilesTouched -v`
Expected: FAIL — `FilesTouched` field does not exist

**Step 3: Write minimal implementation**

Add to `IterationResult` in `internal/runner/runtypes/types.go`, after line 98 (`FirstPassSuccess bool`):

```go
	FilesTouched int
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/runner/runtypes/ -run TestIterationResult_FilesTouched -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/runner/runtypes/types.go internal/runner/runtypes/types_test.go
git commit -m "Add FilesTouched field to IterationResult"
```

---

### Task 2: Add FilesTouched to IterationLog and propagate in writeIterationLog

**Files:**
- Modify: `internal/logger/logger.go:55` (after `FirstPassSuccess`)
- Modify: `internal/runner/logging.go:84` (after `FirstPassSuccess` propagation)
- Test: `internal/runner/writeiterationlog_test.go`

**Step 1: Write the failing test**

Add a test in `writeiterationlog_test.go` that sets `FilesTouched` on the result and verifies it appears in the logged JSONL.

```go
func TestWriteIterationLog_FilesTouched(t *testing.T) {
	dir := t.TempDir()
	l, err := logger.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	r := &Runner{logger: l}
	result := &IterationResult{
		BeadID:       "ft-test",
		Model:        "sonnet",
		Success:      true,
		Duration:     time.Second,
		FilesTouched: 5,
	}

	r.writeIterationLog(1, result)
	_ = l.Close()

	// Read back the log and verify FilesTouched
	entries, err := logger.ReadAllLogs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].FilesTouched != 5 {
		t.Errorf("FilesTouched = %d, want 5", entries[0].FilesTouched)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/runner/ -run TestWriteIterationLog_FilesTouched -v`
Expected: FAIL — `FilesTouched` not a field on `IterationLog`

**Step 3: Write minimal implementation**

In `internal/logger/logger.go`, add after `FirstPassSuccess` (line 55):
```go
	FilesTouched     int    `json:"files_touched,omitempty"`
```

In `internal/runner/logging.go`, add after the `FirstPassSuccess` line (line 84):
```go
		FilesTouched:              result.FilesTouched,
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/runner/ -run TestWriteIterationLog_FilesTouched -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/logger/logger.go internal/runner/logging.go internal/runner/writeiterationlog_test.go
git commit -m "Add FilesTouched to IterationLog and propagate from runner"
```

---

### Task 3: Add FilesTouched to IterationMetric and propagate in buildIterationMetrics

**Files:**
- Modify: `internal/logger/process_trend.go:38` (after `RollingAvgMTTRProxyMs`)
- Modify: `internal/logger/process_trend.go:262` (in `buildIterationMetrics`, propagate field)
- Test: `internal/logger/process_trend_test.go`

**Step 1: Write the failing test**

```go
func TestBuildContinuousMetrics_FilesTouched(t *testing.T) {
	dir := t.TempDir()
	metricsDir := t.TempDir()

	l, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.LogIteration(&IterationLog{
		Timestamp:    time.Now(),
		Iteration:    1,
		BeadID:       "ft-1",
		Model:        "sonnet",
		Success:      true,
		FilesTouched: 3,
	}); err != nil {
		t.Fatal(err)
	}
	l.Close()

	_, err = BuildContinuousMetrics(dir, metricsDir, 30)
	if err != nil {
		t.Fatal(err)
	}

	// Read back metrics JSONL and check FilesTouched
	data, err := os.ReadFile(filepath.Join(metricsDir, "iteration_metrics.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	var metric IterationMetric
	if err := json.Unmarshal(bytes.TrimSpace(data), &metric); err != nil {
		t.Fatal(err)
	}
	if metric.FilesTouched != 3 {
		t.Errorf("FilesTouched = %d, want 3", metric.FilesTouched)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/logger/ -run TestBuildContinuousMetrics_FilesTouched -v`
Expected: FAIL — `FilesTouched` not a field on `IterationMetric`

**Step 3: Write minimal implementation**

In `internal/logger/process_trend.go`, add to `IterationMetric` (after line 37):
```go
	FilesTouched            int       `json:"files_touched,omitempty"`
```

In `buildIterationMetrics` (around line 262), add to the `IterationMetric` literal:
```go
			FilesTouched:            entry.FilesTouched,
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/logger/ -run TestBuildContinuousMetrics_FilesTouched -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/logger/process_trend.go internal/logger/process_trend_test.go
git commit -m "Add FilesTouched to IterationMetric pipeline"
```

---

### Task 4: Add FilesTouched to IterationEfficiency and retro template

**Files:**
- Modify: `internal/logger/efficiency.go:22` (after `ExceededThreshold`)
- Modify: `internal/logger/efficiency.go:135` (where `IterationEfficiency` is built)
- Modify: `.gromit/templates/PROMPT_retro.md:55-58` (per-iteration table)
- Test: `internal/logger/efficiency_test.go`

**Step 1: Write the failing test**

Add a test in `efficiency_test.go` that creates a log entry with `FilesTouched` and verifies it appears in the efficiency report's `CurrentIterations`.

```go
func TestReadEfficiencyReport_FilesTouched(t *testing.T) {
	dir := t.TempDir()
	runID := "20260218-120000"
	logContent := `{"timestamp":"2026-02-18T12:00:00Z","iteration":1,"bead_id":"ft-1","bead_title":"Test","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000,"cost_usd":0.1,"input_tokens":1000,"output_tokens":200,"files_touched":4}
`
	if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("run-%s.jsonl", runID)), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := ReadEfficiencyReport(dir, runID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.CurrentIterations) != 1 {
		t.Fatalf("expected 1 iteration, got %d", len(report.CurrentIterations))
	}
	if report.CurrentIterations[0].FilesTouched != 4 {
		t.Errorf("FilesTouched = %d, want 4", report.CurrentIterations[0].FilesTouched)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/logger/ -run TestReadEfficiencyReport_FilesTouched -v`
Expected: FAIL — `FilesTouched` not a field on `IterationEfficiency`

**Step 3: Write minimal implementation**

In `internal/logger/efficiency.go`, add to `IterationEfficiency` (after `ExceededThreshold`):
```go
	FilesTouched      int
```

In the efficiency report builder (around line 135), where `IterationEfficiency` is constructed, add:
```go
					FilesTouched:      entry.FilesTouched,
```

In `.gromit/templates/PROMPT_retro.md`, update the per-iteration table header (line 55-58):
```
| Bead ID | Model | Duration | Cost (USD) | Input Tokens | Output Tokens | Files Touched |
|---------|-------|----------|------------|--------------|---------------|---------------|
{{- range .Efficiency.CurrentIterations }}
| {{ .BeadID }} | {{ .Model }} | {{ .Duration }} | ${{ printf "%.4f" .CostUSD }} | {{ .InputTokens }} | {{ .OutputTokens }} | {{ .FilesTouched }} |
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/logger/ -run TestReadEfficiencyReport_FilesTouched -v`
Expected: PASS

**Step 5: Verify template renders**

Run: `go test ./internal/retro/ -run TestRealTemplateExecution -v`
Expected: PASS (template still parses correctly)

**Step 6: Commit**

```bash
git add internal/logger/efficiency.go internal/logger/efficiency_test.go .gromit/templates/PROMPT_retro.md
git commit -m "Add FilesTouched to efficiency report and retro template"
```

---

### Task 5: Compute file count in processBead

**Files:**
- Modify: `internal/runner/runner.go:204` (add defer after `setupBeadContext`)
- Test: `internal/runner/process_test.go`

**Step 1: Write the failing test**

Add a test that runs `processBead` with a mock `gitDiffFn` that returns a multi-file diff, and verifies `result.FilesTouched` is set correctly.

```go
func TestProcessBead_FilesTouched(t *testing.T) {
	r := newTestRunner(t)
	r.gitDiffFn = func(fromCommit string) (string, error) {
		return "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n" +
			"diff --git a/bar.go b/bar.go\n--- a/bar.go\n+++ b/bar.go\n" +
			"diff --git a/baz_test.go b/baz_test.go\n--- a/baz_test.go\n+++ b/baz_test.go\n", nil
	}
	// Setup mock invocation that succeeds
	r.invoker = &mockInvoker{result: &claude.Result{Success: true}}

	b := &bead.Bead{ID: "ft-bead", Title: "File Touch Test", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}
	result := r.processBead(context.Background(), b, 1, time.Now().Add(time.Minute), nil)

	if result.FilesTouched != 3 {
		t.Errorf("FilesTouched = %d, want 3", result.FilesTouched)
	}
}
```

Note: The exact test setup depends on the existing `newTestRunner` helper and mock patterns in `process_test.go`. Match the existing test style.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/runner/ -run TestProcessBead_FilesTouched -v`
Expected: FAIL — `FilesTouched` is 0

**Step 3: Write minimal implementation**

In `internal/runner/runner.go`, add after line 204 (`defer beadCancel()`), before the existing duration defer:

```go
	defer func() {
		if bc.StartCommit != "" {
			if diff, err := r.getDiff(bc.StartCommit); err == nil {
				bc.Result.FilesTouched = len(methodology.ParseDiffFiles(diff))
			}
		}
	}()
```

Add the import for `methodology` if not already present:
```go
	"github.com/danabrams/gromit/internal/runner/methodology"
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/runner/ -run TestProcessBead_FilesTouched -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `go test ./internal/runner/... -count=1`
Expected: All existing tests pass

**Step 6: Commit**

```bash
git add internal/runner/runner.go internal/runner/process_test.go
git commit -m "Compute FilesTouched via git diff in processBead"
```

---

### Task 6: End-to-end verification

**Step 1: Run full test suite**

Run: `go test ./... -count=1`
Expected: All tests pass

**Step 2: Verify JSONL schema**

Manually inspect `internal/logger/process_trend.go` IterationMetric JSON tags to confirm `files_touched` appears in output.

**Step 3: Verify retro template**

Run: `go test ./internal/retro/ -count=1`
Expected: All retro tests pass, template renders correctly with new column.

**Step 4: Final commit if any fixups needed**

```bash
git add -A
git commit -m "Fix any remaining issues from file count experiment instrumentation"
```
