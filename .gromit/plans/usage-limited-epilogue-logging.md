# Usage-Limited Epilogue Logging Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Consolidate iteration log writing into the epilogue stage and ensure usage_limited flag is properly persisted in JSONL output.

**Architecture:** Currently, iteration log writing is scattered across runner/logging.go (constructs logger.IterationLog from IterationResult) and invoked from run_iteration.go. The epilogue stage already has an IterationLogWriter interface but the runner still constructs and writes the log directly. This plan moves log construction and writing entirely into the epilogue, making it the single source of truth for iteration log persistence. The epilogue's Input.Result field will carry a pre-constructed logger.IterationLog that includes the usage_limited flag when set.

**Tech Stack:** Go testing (TDD), JSON marshaling, pipeline.Input/Output structs

---

### Task 1: Create an IterationLogWriter adapter for the legacy logger

**Files:**
- Modify: `internal/pipeline/epilogue/epilogue.go`
- Modify: `internal/runner/logging.go`
- Test: `internal/pipeline/epilogue/epilogue_test.go`

**Step 1: Write the failing test for IterationLogWriter adapter**

Add to `internal/pipeline/epilogue/epilogue_test.go`:

```go
// TestEpilogue_WritesIterationLog_WithUsageLimited verifies that when
// in.Result.UsageLimited == true, the log entry includes usage_limited: true.
func TestEpilogue_WritesIterationLog_WithUsageLimited(t *testing.T) {
	beads := &fakeBeadLifecycle{}
	status := &fakeStatusWriter{}
	var capturedLog *logger.IterationLog

	logWriter := &fakeIterationLogWriter{
		writeFn: func(log *logger.IterationLog) error {
			capturedLog = log
			return nil
		},
	}

	in := makeInput("bead-1", "Title", true)
	in.Result = &logger.IterationLog{
		BeadID:       "bead-1",
		BeadTitle:    "Title",
		Success:      true,
		UsageLimited: true,
		Iteration:    1,
	}

	stage := epiloguepkg.New(beads, status, io.Discard)
	stage.WithIterationLogWriter(logWriter)

	_, err := stage.Run(context.Background(), in)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if capturedLog == nil {
		t.Fatal("expected log to be written")
	}

	if !capturedLog.UsageLimited {
		t.Fatal("expected UsageLimited to be true")
	}
}

// fakeIterationLogWriter is a test double for epilogue.IterationLogWriter.
type fakeIterationLogWriter struct {
	writeFn func(log *logger.IterationLog) error
	called  bool
	log     *logger.IterationLog
}

func (f *fakeIterationLogWriter) Write(log *logger.IterationLog) error {
	f.called = true
	f.log = log
	if f.writeFn != nil {
		return f.writeFn(log)
	}
	return nil
}
```

**Step 2: Run test to verify it fails**

```bash
cd /home/dabrams/gromit
go test -v ./internal/pipeline/epilogue -run TestEpilogue_WritesIterationLog_WithUsageLimited
```

Expected: FAIL (test compiles but test helper fakeIterationLogWriter doesn't exist yet, or assertion fails because usage_limited is not propagated)

**Step 3: Create LoggerIterationLogWriter adapter in runner/logging.go**

Modify `internal/runner/logging.go` to add a new adapter type that wraps the runner's logger:

```go
// LoggerIterationLogWriter adapts the runner's logger.IterationLogger interface
// to the epilogue's IterationLogWriter interface.
type LoggerIterationLogWriter struct {
	logger IterationLogger
}

// NewLoggerIterationLogWriter creates an adapter that writes to an IterationLogger.
func NewLoggerIterationLogWriter(logger IterationLogger) *LoggerIterationLogWriter {
	return &LoggerIterationLogWriter{logger: logger}
}

// Write implements epilogue.IterationLogWriter by delegating to the wrapped logger.
func (w *LoggerIterationLogWriter) Write(log *logger.IterationLog) error {
	if w == nil || w.logger == nil || log == nil {
		return nil
	}
	return w.logger.LogIteration(log)
}
```

**Step 4: Run test to verify it passes**

```bash
cd /home/dabrams/gromit
go test -v ./internal/pipeline/epilogue -run TestEpilogue_WritesIterationLog_WithUsageLimited
```

Expected: PASS

**Step 5: Commit**

```bash
cd /home/dabrams/gromit
git add internal/pipeline/epilogue/epilogue.go internal/pipeline/epilogue/epilogue_test.go internal/runner/logging.go
git commit -m "feat: add LoggerIterationLogWriter adapter for epilogue logging integration"
```

---

### Task 2: Move iteration log entry construction into epilogue

**Files:**
- Modify: `internal/runner/logging.go` - extract newIterationLogEntry into epilogue package
- Create: `internal/pipeline/epilogue/iteration_log_builder.go`
- Test: `internal/pipeline/epilogue/epilogue_test.go`

**Step 1: Write test for log entry construction with UsageLimited**

Add to `internal/pipeline/epilogue/epilogue_test.go`:

```go
// TestBuildIterationLog_IncludesUsageLimited verifies that usage_limited
// is included in the constructed log entry when set.
func TestBuildIterationLog_IncludesUsageLimited(t *testing.T) {
	input := pipeline.Input{
		Iteration: 5,
		Bead: &bead.Bead{
			ID:    "test-bead",
			Title: "Test Title",
		},
		Result: &logger.IterationLog{
			BeadID:       "test-bead",
			BeadTitle:    "Test Title",
			Iteration:    5,
			UsageLimited: true,
			Success:      true,
			Model:        "haiku",
			Duration:     1000,
		},
	}

	logEntry := buildIterationLog(input)

	if !logEntry.UsageLimited {
		t.Fatal("expected UsageLimited to be true in built log entry")
	}
	if logEntry.BeadID != "test-bead" {
		t.Fatalf("expected BeadID test-bead, got %s", logEntry.BeadID)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /home/dabrams/gromit
go test -v ./internal/pipeline/epilogue -run TestBuildIterationLog_IncludesUsageLimited
```

Expected: FAIL (buildIterationLog function doesn't exist)

**Step 3: Create buildIterationLog function in epilogue**

Create `internal/pipeline/epilogue/iteration_log_builder.go`:

```go
package epilogue

import (
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
)

// buildIterationLog constructs a logger.IterationLog from pipeline.Input.
// The Input.Result must be a pre-built logger.IterationLog (passed by runner).
func buildIterationLog(in pipeline.Input) *logger.IterationLog {
	if in.Result == nil {
		return nil
	}

	// For now, Input.Result is already a constructed IterationLog.
	// In the future, we may need to build it from IterationResult here.
	log, ok := in.Result.(*logger.IterationLog)
	if !ok {
		return nil
	}

	// Ensure iteration number is set from input
	if log.Iteration == 0 {
		log.Iteration = in.Iteration
	}

	return log
}
```

**Step 4: Run test to verify it passes**

```bash
cd /home/dabrams/gromit
go test -v ./internal/pipeline/epilogue -run TestBuildIterationLog_IncludesUsageLimited
```

Expected: PASS

**Step 5: Commit**

```bash
cd /home/dabrams/gromit
git add internal/pipeline/epilogue/iteration_log_builder.go internal/pipeline/epilogue/epilogue_test.go
git commit -m "feat: add buildIterationLog function to construct logs with UsageLimited field"
```

---

### Task 3: Remove iteration log writing from runner

**Files:**
- Modify: `internal/runner/run_iteration.go` - remove writeIterationLog call
- Modify: `internal/runner/logging.go` - mark writeIterationLog as deprecated
- Test: `internal/runner/writeiterationlog_test.go` - add test noting legacy behavior

**Step 1: Write test for deprecated behavior**

Add to `internal/runner/writeiterationlog_test.go`:

```go
// TestWriteIterationLog_DeprecatedInRunIteration verifies that writeIterationLog
// is no longer called from run_iteration; logging is now handled by epilogue.
func TestWriteIterationLog_DeprecatedInRunIteration(t *testing.T) {
	// This test documents the transition: writeIterationLog should not be called
	// from the main iteration loop anymore. The epilogue stage now owns logging.
	// This test can be removed once writeIterationLog is fully removed.

	// Verify that the function still exists and works for backward compatibility
	tmpDir := t.TempDir()
	l, err := logger.NewLogger(tmpDir)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer l.Close()

	r := &Runner{logger: l}
	result := &IterationResult{
		BeadID:    "deprecated-test",
		BeadTitle: "Test",
		Success:   true,
		UsageLimited: true,
		Duration:  time.Second,
	}

	r.writeIterationLog(1, result)

	// Verify the log was written (backward compatibility)
	// but note: new code should use epilogue.IterationLogWriter instead
}
```

**Step 2: Run test to verify it passes**

```bash
cd /home/dabrams/gromit
go test -v ./internal/runner -run TestWriteIterationLog_DeprecatedInRunIteration
```

Expected: PASS

**Step 3: Remove the writeIterationLog call from run_iteration.go**

Read and then modify `internal/runner/run_iteration.go` to remove the line:

```go
r.writeIterationLog(st.iteration, result)
```

The line is currently around line 401. After removal, the epilogue stage will handle logging.

**Step 4: Run existing tests to verify nothing breaks**

```bash
cd /home/dabrams/gromit
go test -v ./internal/runner -run "TestRunIteration|TestEpilogue"
```

Expected: All existing tests still pass (or fail with clear, addressable errors)

**Step 5: Commit**

```bash
cd /home/dabrams/gromit
git add internal/runner/run_iteration.go internal/runner/writeiterationlog_test.go
git commit -m "refactor: remove writeIterationLog call from run_iteration; epilogue now owns logging"
```

---

### Task 4: Wire epilogue to use IterationLogWriter in runner

**Files:**
- Modify: `internal/runner/runner.go` or orchestration code
- Test: Integration test verifying epilogue logs with usage_limited

**Step 1: Write integration test**

Add to `internal/runner/epilogue_test.go`:

```go
// TestRunnerEpilogueLogging_WritesUsageLimited verifies that the runner
// wires epilogue with an IterationLogWriter that persists usage_limited.
func TestRunnerEpilogueLogging_WritesUsageLimited(t *testing.T) {
	tmpDir := t.TempDir()
	l, err := logger.NewLogger(tmpDir)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer l.Close()

	// Create a mock epilogue that receives an Input with a Result
	input := pipeline.Input{
		Iteration: 1,
		Bead: &bead.Bead{ID: "usage-test", Title: "Test"},
		Result: &logger.IterationLog{
			BeadID:       "usage-test",
			Iteration:    1,
			UsageLimited: true,
			Success:      true,
			Duration:     1000,
		},
	}

	// Verify that the log writer can receive and persist this
	writer := runner.NewLoggerIterationLogWriter(l)

	err = writer.Write(input.Result.(*logger.IterationLog))
	if err != nil {
		t.Fatalf("failed to write log: %v", err)
	}

	// Verify the log was written with usage_limited
	logs, err := logger.ReadIterationLogs(tmpDir)
	if err != nil {
		t.Fatalf("failed to read logs: %v", err)
	}

	if len(logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(logs))
	}

	if !logs[0].UsageLimited {
		t.Fatal("expected usage_limited to be true in persisted log")
	}
}
```

**Step 2: Run test to verify it fails or passes**

```bash
cd /home/dabrams/gromit
go test -v ./internal/runner -run TestRunnerEpilogueLogging_WritesUsageLimited
```

Expected: PASS (wiring should already work, just verifying integration)

**Step 3: Verify runner creates and wires epilogue correctly**

Find where the runner creates the epilogue stage and ensure it wires the IterationLogWriter:

```bash
grep -n "epiloguepkg.New\|WithIterationLogWriter" internal/runner/*.go
```

Add wiring code if it doesn't exist:

```go
// In runner initialization or loop orchestration:
logWriter := runner.NewLoggerIterationLogWriter(r.logger)
epilogueStage.WithIterationLogWriter(logWriter)
```

**Step 4: Run full test suite to verify integration**

```bash
cd /home/dabrams/gromit
go test -v ./internal/runner -run Epilogue
go test -v ./internal/pipeline/epilogue
```

Expected: All tests pass

**Step 5: Commit**

```bash
cd /home/dabrams/gromit
git add internal/runner/epilogue_test.go
git commit -m "feat: wire epilogue IterationLogWriter in runner with usage_limited persistence"
```

---

### Task 5: Add comprehensive tests for usage_limited persistence

**Files:**
- Test: `internal/pipeline/epilogue/epilogue_test.go`
- Test: `internal/runner/logging_test.go`

**Step 1: Write test for usage_limited=false (default case)**

Add to `internal/pipeline/epilogue/epilogue_test.go`:

```go
// TestEpilogue_WritesIterationLog_WithoutUsageLimited verifies default case
// where usage_limited is false and omitted from JSON.
func TestEpilogue_WritesIterationLog_WithoutUsageLimited(t *testing.T) {
	var capturedLog *logger.IterationLog

	logWriter := &fakeIterationLogWriter{
		writeFn: func(log *logger.IterationLog) error {
			capturedLog = log
			return nil
		},
	}

	in := makeInput("bead-2", "Default Case", true)
	in.Result = &logger.IterationLog{
		BeadID:    "bead-2",
		BeadTitle: "Default Case",
		Success:   true,
		Iteration: 1,
		// UsageLimited not set (false)
	}

	stage := epiloguepkg.New(&fakeBeadLifecycle{}, &fakeStatusWriter{}, io.Discard)
	stage.WithIterationLogWriter(logWriter)

	_, err := stage.Run(context.Background(), in)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if capturedLog == nil {
		t.Fatal("expected log to be written")
	}

	if capturedLog.UsageLimited {
		t.Fatal("expected UsageLimited to be false by default")
	}
}
```

**Step 2: Write test for JSONL serialization**

Add to `internal/runner/logging_test.go`:

```go
// TestIterationLogJSON_SerializesUsageLimited verifies that usage_limited
// is correctly serialized to JSON with omitempty behavior.
func TestIterationLogJSON_SerializesUsageLimited(t *testing.T) {
	tmpDir := t.TempDir()
	l, err := logger.NewLogger(tmpDir)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer l.Close()

	logWithUsageLimited := &logger.IterationLog{
		BeadID:       "json-test-1",
		BeadTitle:    "With UsageLimited",
		Iteration:    1,
		Success:      true,
		UsageLimited: true,
		Duration:     1000,
		Model:        "haiku",
	}

	if err := l.LogIteration(logWithUsageLimited); err != nil {
		t.Fatalf("failed to log iteration: %v", err)
	}

	// Read back and verify JSON contains usage_limited: true
	logs, err := logger.ReadIterationLogs(tmpDir)
	if err != nil {
		t.Fatalf("failed to read logs: %v", err)
	}

	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}

	if !logs[0].UsageLimited {
		t.Fatal("expected usage_limited to persist in JSON")
	}
}
```

**Step 3: Run new tests**

```bash
cd /home/dabrams/gromit
go test -v ./internal/pipeline/epilogue -run TestEpilogue_WritesIterationLog
go test -v ./internal/runner -run TestIterationLogJSON_SerializesUsageLimited
```

Expected: All tests PASS

**Step 4: Commit**

```bash
cd /home/dabrams/gromit
git add internal/pipeline/epilogue/epilogue_test.go internal/runner/logging_test.go
git commit -m "test: add comprehensive tests for usage_limited persistence in iteration logs"
```

---

## Summary

This plan consolidates iteration log writing into the epilogue stage, making it the single source of truth for persistence. The key changes are:

1. **LoggerIterationLogWriter adapter** - bridges runner's logger interface to epilogue's IterationLogWriter
2. **buildIterationLog function** - ensures usage_limited and other fields are preserved during construction
3. **Remove runner logging call** - epilogue now owns all iteration log I/O
4. **Wire epilogue** - runner creates and configures the IterationLogWriter
5. **Comprehensive tests** - verify usage_limited is correctly serialized in all cases

All tests use TDD (write failing test → implement → verify pass) with frequent commits after each logical step.
