# Scenario Test Writer Resilience Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the scenario test writer resilient to LLM format non-compliance by adding a fuzzy fallback parser and retrying parse failures.

**Architecture:** Two isolated changes: (1) `parseScenarioTestResponse` gains a markdown-fence fallback that fires only when strict marker parsing fails; (2) the retry loop in `WriteScenarioTestsStage.Run` treats parse errors the same as compile errors — passes them back as feedback and retries.

**Tech Stack:** Go, standard library only (`strings`, `os`, `path/filepath`)

---

### Task 1: Add unit tests for the fuzzy fallback parser

**Files:**
- Modify: `internal/next/contract/llm_scenario_test_writer_test.go`

The existing `TestLLMScenarioTestWriter_*` tests all use the strict `===` marker format. Add tests for the fallback cases directly against `parseScenarioTestResponse` (call it from the same package — it's unexported but tests are in `package contract`).

**Step 1: Write the failing tests**

Add these test functions after the existing tests in `llm_scenario_test_writer_test.go`:

```go
// TestParseScenarioTestResponse_StrictMarkers verifies existing strict parsing still works.
func TestParseScenarioTestResponse_StrictMarkers(t *testing.T) {
	response := "===TEST_FILE_PATH===\ninternal/pkg/foo_test.go\n===TEST_FILE_CONTENT===\npackage pkg\n===END_TEST_FILE==="
	path, content, err := parseScenarioTestResponse(response)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "internal/pkg/foo_test.go" {
		t.Errorf("expected path %q, got %q", "internal/pkg/foo_test.go", path)
	}
	if content != "package pkg" {
		t.Errorf("expected content %q, got %q", "package pkg", content)
	}
}

// TestParseScenarioTestResponse_FenceWithPathBefore verifies fallback extracts path
// from a line ending in .go immediately before the ```go fence.
func TestParseScenarioTestResponse_FenceWithPathBefore(t *testing.T) {
	response := "Here is the test file:\n\ninternal/pkg/foo_test.go\n```go\npackage pkg\n\nfunc TestFoo(t *testing.T) {}\n```\n"
	path, content, err := parseScenarioTestResponse(response)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "internal/pkg/foo_test.go" {
		t.Errorf("expected path %q, got %q", "internal/pkg/foo_test.go", path)
	}
	if !strings.Contains(content, "package pkg") {
		t.Errorf("expected content to contain 'package pkg', got %q", content)
	}
}

// TestParseScenarioTestResponse_FenceWithPathComment verifies fallback extracts path
// from a // path comment at the top of the ```go fence body.
func TestParseScenarioTestResponse_FenceWithPathComment(t *testing.T) {
	response := "```go\n// internal/pkg/foo_test.go\npackage pkg\n\nfunc TestFoo(t *testing.T) {}\n```\n"
	path, content, err := parseScenarioTestResponse(response)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "internal/pkg/foo_test.go" {
		t.Errorf("expected path %q, got %q", "internal/pkg/foo_test.go", path)
	}
	if !strings.Contains(content, "package pkg") {
		t.Errorf("expected content to contain 'package pkg', got %q", content)
	}
}

// TestParseScenarioTestResponse_FenceNoPath verifies fallback returns the original
// strict-parse error when no path can be found anywhere.
func TestParseScenarioTestResponse_FenceNoPath(t *testing.T) {
	response := "```go\npackage pkg\n\nfunc TestFoo(t *testing.T) {}\n```\n"
	_, _, err := parseScenarioTestResponse(response)
	if err == nil {
		t.Fatal("expected error when no path found, got nil")
	}
	if !strings.Contains(err.Error(), "===TEST_FILE_PATH===") {
		t.Errorf("expected original strict-parse error, got: %v", err)
	}
}

// TestParseScenarioTestResponse_NoFenceNoMarkers verifies original error is returned
// when neither markers nor a fence are present.
func TestParseScenarioTestResponse_NoFenceNoMarkers(t *testing.T) {
	response := "Here is some prose with no code block or markers."
	_, _, err := parseScenarioTestResponse(response)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "===TEST_FILE_PATH===") {
		t.Errorf("expected original strict-parse error, got: %v", err)
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/next/contract/ -run 'TestParseScenarioTestResponse' -count=1 -v
```

Expected: `FenceWithPathBefore`, `FenceWithPathComment` FAIL (fallback not implemented yet). `StrictMarkers` passes. `FenceNoPath` and `NoFenceNoMarkers` may pass or fail depending on exact error message.

**Step 3: Commit the failing tests**

```bash
git add internal/next/contract/llm_scenario_test_writer_test.go
git commit -m "test: red — fuzzy fallback parser for scenario test writer"
```

---

### Task 2: Implement the fuzzy fallback in `parseScenarioTestResponse`

**Files:**
- Modify: `internal/next/contract/llm_scenario_test_writer.go`

**Step 1: Add the fallback after the strict parse fails**

In `parseScenarioTestResponse`, after the existing strict-marker checks return the `pathStart == -1` error, add the fallback. The full updated function:

```go
func parseScenarioTestResponse(response string) (string, string, error) {
	pathStart := strings.Index(response, "===TEST_FILE_PATH===")
	if pathStart == -1 {
		// Strict parse failed — try fuzzy fallback before giving up.
		if path, content, ok := parseFenceResponse(response); ok {
			return path, content, nil
		}
		return "", "", fmt.Errorf("response missing ===TEST_FILE_PATH=== marker")
	}

	contentStart := strings.Index(response, "===TEST_FILE_CONTENT===")
	if contentStart == -1 {
		return "", "", fmt.Errorf("response missing ===TEST_FILE_CONTENT=== marker")
	}

	endMarker := strings.Index(response, "===END_TEST_FILE===")
	if endMarker == -1 {
		return "", "", fmt.Errorf("response missing ===END_TEST_FILE=== marker")
	}

	if pathStart >= contentStart || contentStart >= endMarker {
		return "", "", fmt.Errorf("invalid marker order in response")
	}

	pathContent := response[pathStart+len("===TEST_FILE_PATH===") : contentStart]
	testFilePath := strings.TrimSpace(pathContent)

	contentContent := response[contentStart+len("===TEST_FILE_CONTENT===") : endMarker]
	testContent := strings.TrimSpace(contentContent)

	if testFilePath == "" {
		return "", "", fmt.Errorf("test file path is empty")
	}
	if testContent == "" {
		return "", "", fmt.Errorf("test file content is empty")
	}

	return testFilePath, testContent, nil
}

// parseFenceResponse attempts to extract a test file path and content from a markdown
// ```go fence when the LLM did not use the === markers.
//
// Path extraction order:
//  1. The line immediately before the ```go fence, if it ends in ".go"
//  2. The first line of the fence body if it is a // comment ending in ".go"
//
// Returns ("", "", false) if no fence or no path is found.
func parseFenceResponse(response string) (path, content string, ok bool) {
	// Find the opening ```go fence.
	fenceOpen := strings.Index(response, "```go")
	if fenceOpen == -1 {
		return "", "", false
	}

	// Find the closing ``` after the opening fence.
	afterOpen := fenceOpen + len("```go")
	fenceClose := strings.Index(response[afterOpen:], "```")
	if fenceClose == -1 {
		return "", "", false
	}
	fenceClose += afterOpen

	// Extract fence body.
	body := strings.TrimSpace(response[afterOpen:fenceClose])
	if body == "" {
		return "", "", false
	}

	// Strategy 1: look for a .go path on the line immediately before the fence.
	beforeFence := strings.TrimRight(response[:fenceOpen], " \t")
	if lastNewline := strings.LastIndex(beforeFence, "\n"); lastNewline >= 0 {
		candidate := strings.TrimSpace(beforeFence[lastNewline+1:])
		if strings.HasSuffix(candidate, ".go") {
			return candidate, body, true
		}
	}

	// Strategy 2: look for a // comment path at the top of the fence body.
	firstLine := body
	if nl := strings.Index(body, "\n"); nl >= 0 {
		firstLine = body[:nl]
		body = strings.TrimSpace(body[nl+1:])
	}
	if strings.HasPrefix(firstLine, "//") {
		candidate := strings.TrimSpace(strings.TrimPrefix(firstLine, "//"))
		if strings.HasSuffix(candidate, ".go") {
			return candidate, body, true
		}
	}

	return "", "", false
}
```

**Step 2: Run the new tests**

```bash
go test ./internal/next/contract/ -run 'TestParseScenarioTestResponse' -count=1 -v
```

Expected: all 5 tests PASS.

**Step 3: Run all contract tests**

```bash
go test ./internal/next/contract/ -count=1
```

Expected: all PASS.

**Step 4: Commit**

```bash
git add internal/next/contract/llm_scenario_test_writer.go
git commit -m "feat: fuzzy fallback parser for scenario test writer

When LLM omits === markers, extract test file path and content from a
markdown code fence. Path is found on the line before the fence or as
a // comment at the top of the fence body."
```

---

### Task 3: Add a unit test for parse-error retry in the stage

**Files:**
- Modify: `internal/next/specloop/stages/write_scenario_tests_test.go`

The existing `fakeScenarioTestWriter` supports `failAttempt` (returns a generic error). We need a test where the first call returns a parse-format error and the second succeeds.

**Step 1: Update `fakeScenarioTestWriter` to support a configurable error message**

In `write_scenario_tests_test.go`, add a `failErr` field to `fakeScenarioTestWriter` so tests can control the exact error returned:

```go
type fakeScenarioTestWriter struct {
	calls               int
	failAttempt         int // -1 means never fail, N means fail on attempt N (0-indexed)
	failErr             error // error to return on failAttempt; defaults to a generic error if nil
	returnedPaths       []string
	returnedPathIndex   int
	compilableScenarios map[string]bool
}

func (m *fakeScenarioTestWriter) WriteScenarioTest(
	ctx context.Context,
	scenario contract.SpecScenario,
	implFiles []string,
	workDir string,
	compileErrors string,
) (testFilePath string, err error) {
	defer func() { m.calls++ }()

	if m.failAttempt >= 0 && m.calls == m.failAttempt {
		if m.failErr != nil {
			return "", m.failErr
		}
		return "", fmt.Errorf("mock writer simulated error on attempt %d", m.calls)
	}
	// ... rest unchanged
```

**Step 2: Write the failing test**

Add to `write_scenario_tests_test.go`:

```go
func TestWriteScenarioTests_ParseErrorRetried(t *testing.T) {
	// Parse error on attempt 0 is treated as retryable; attempt 1 succeeds.
	tmp := t.TempDir()
	rs := makeWriteScenarioTestsRunState(t)

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte(specWithTwoScenarios), 0o644); err != nil {
		t.Fatal(err)
	}

	evidenceDir := filepath.Join(tmp, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	testFile1 := filepath.Join(evidenceDir, "scenario_one_test.go")
	testFile2 := filepath.Join(evidenceDir, "scenario_two_test.go")

	parseErr := fmt.Errorf("parse scenario test response: response missing ===TEST_FILE_PATH=== marker")

	// capture compileErrors passed on retry
	var capturedCompileErrors []string
	writer := &fakeScenarioTestWriter{
		failAttempt:   0,
		failErr:       parseErr,
		returnedPaths: []string{testFile1, testFile2},
		compilableScenarios: map[string]bool{
			"scenario-one": true,
			"scenario-two": true,
		},
	}
	_ = capturedCompileErrors

	cfg := WriteScenarioTestsStageConfig{
		SpecPath:    specPath,
		EvidenceDir: evidenceDir,
		WorkDir:     tmp,
	}
	stage := NewWriteScenarioTestsStage(writer, cfg, nil, nil)

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind == specloop.Blocked {
		failures := ""
		if action.Context != nil {
			failures = strings.Join(action.Context.Failures, "; ")
		}
		t.Fatalf("expected Continue after parse-error retry, got Blocked: %s", failures)
	}
	// writer should have been called at least twice (fail then succeed for scenario-one)
	if writer.calls < 2 {
		t.Fatalf("expected at least 2 writer calls, got %d", writer.calls)
	}
}
```

Note: this test will need `strings` imported — it's already in the file's import block.

**Step 3: Run to verify it fails**

```bash
go test ./internal/next/specloop/stages/ -run 'TestWriteScenarioTests_ParseErrorRetried' -count=1 -v
```

Expected: FAIL — parse error causes immediate block, no retry.

**Step 4: Commit the failing test**

```bash
git add internal/next/specloop/stages/write_scenario_tests_test.go
git commit -m "test: red — parse error retry in write_scenario_tests stage"
```

---

### Task 4: Implement parse-error retry in the stage

**Files:**
- Modify: `internal/next/specloop/stages/write_scenario_tests.go`

**Step 1: Find the retry loop and update the error handling**

The retry loop is at line ~151. Currently when `writeErr != nil`, it sets `blockedReason` and breaks. Change it to check if the error is a parse error and if so treat it like a compile error.

Replace the block inside the `for attempt` loop that handles `writeErr != nil`:

```go
// Current (breaks immediately on any write error):
if writeErr != nil {
    blockedReason = fmt.Sprintf("scenario test writer error for %q: %v", scenario.Name, writeErr)
    break
}
```

With:

```go
if writeErr != nil {
    errMsg := writeErr.Error()
    // Parse errors are retryable — treat like compile errors so the LLM
    // gets feedback about the format and can correct itself.
    if strings.Contains(errMsg, "parse scenario test response:") && attempt < maxRetries {
        compileErrors = "Prior format error (fix your output format):\n" + errMsg + "\n\n" + compileErrors
        continue
    }
    blockedReason = fmt.Sprintf("scenario test writer error for %q: %v", scenario.Name, writeErr)
    break
}
```

Make sure `"strings"` is already in the import block — it is.

**Step 2: Run the new test**

```bash
go test ./internal/next/specloop/stages/ -run 'TestWriteScenarioTests_ParseErrorRetried' -count=1 -v
```

Expected: PASS.

**Step 3: Run all stage tests**

```bash
go test ./internal/next/specloop/stages/ -count=1
```

Expected: all PASS.

**Step 4: Run full test suite**

```bash
go test ./... -count=1
```

Expected: all PASS.

**Step 5: Commit**

```bash
git add internal/next/specloop/stages/write_scenario_tests.go
git commit -m "feat: retry parse errors in write_scenario_tests stage

Parse errors from the LLM (missing === markers) are now retried with
the error fed back into the prompt, matching compile-error retry behavior.
Fixes the root cause of run-94196d329c7dae1d blocking."
```

---

### Task 5: Verify and clean up

**Step 1: Run the full suite one more time**

```bash
go build ./... && go vet ./... && go test ./... -count=1
```

Expected: clean build, no vet warnings, all tests pass.

**Step 2: Done**

No further cleanup needed. The two changes are complete and independent.
