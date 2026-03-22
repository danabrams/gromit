package contract

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

// mockInvoker implements llmadapter.Invoker and returns canned responses for testing.
type mockInvoker struct {
	response       string
	invokeCallFunc func(ctx context.Context, prompt string) // optional callback to inspect prompt
}

func (m *mockInvoker) Invoke(ctx context.Context, prompt string) (*provider.Result, error) {
	if m.invokeCallFunc != nil {
		m.invokeCallFunc(ctx, prompt)
	}
	return &provider.Result{
		Success: true,
		Output:  m.response,
	}, nil
}

func (m *mockInvoker) InvokeInDir(ctx context.Context, prompt string, dir string) (*provider.Result, error) {
	if m.invokeCallFunc != nil {
		m.invokeCallFunc(ctx, prompt)
	}
	return &provider.Result{
		Success: true,
		Output:  m.response,
	}, nil
}

// TestLLMScenarioTestWriter_HappyPath tests that a valid LLM response produces a correctly written test file.
func TestLLMScenarioTestWriter_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an implementation file to be read.
	implFile := "pkg/mypackage/impl.go"
	implPath := filepath.Join(tmpDir, implFile)
	if err := os.MkdirAll(filepath.Dir(implPath), 0o755); err != nil {
		t.Fatal(err)
	}
	implContent := "package mypackage\n\nfunc Add(a, b int) int { return a + b }\n"
	if err := os.WriteFile(implPath, []byte(implContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a scenario and test writer.
	scenario := SpecScenario{
		Name:  "add-works",
		Given: "two positive integers",
		When:  "Add is called",
		Then:  "the sum is returned",
		Notes: "basic arithmetic test",
	}

	// Mock response with properly formatted test file output.
	testFilePath := "pkg/mypackage/mypackage_scenario_add_works_test.go"
	testContent := `package mypackage

import "testing"

func TestScenario_AddWorks(t *testing.T) {
	// Seed
	a, b := 2, 3

	// Invoke
	result := Add(a, b)

	// Assert
	if result != 5 {
		t.Errorf("expected 5, got %d", result)
	}
}
`
	mockResponse := "===TEST_FILE_PATH===\n" + testFilePath + "\n===TEST_FILE_CONTENT===\n" + testContent + "\n===END_TEST_FILE===\n"

	invoker := &mockInvoker{response: mockResponse}
	writer := NewLLMScenarioTestWriter(invoker, "# Scenario Test Patterns\n\nUse the Seed/Invoke/Assert pattern.")

	// Invoke WriteScenarioTest.
	result, err := writer.WriteScenarioTest(context.Background(), scenario, []string{implFile}, tmpDir, "")
	if err != nil {
		t.Fatalf("WriteScenarioTest failed: %v", err)
	}

	// Verify returned path matches expected.
	if result != testFilePath {
		t.Errorf("expected path %q, got %q", testFilePath, result)
	}

	// Verify file was written to correct location.
	writtenPath := filepath.Join(tmpDir, testFilePath)
	writtenContent, err := os.ReadFile(writtenPath)
	if err != nil {
		t.Fatalf("could not read written file: %v", err)
	}
	if strings.TrimSpace(string(writtenContent)) != strings.TrimSpace(testContent) {
		t.Errorf("written content mismatch.\nexpected:\n%s\ngot:\n%s", testContent, string(writtenContent))
	}
}

// TestLLMScenarioTestWriter_WithCompileErrors verifies that compile errors are included in the prompt to the LLM.
func TestLLMScenarioTestWriter_WithCompileErrors(t *testing.T) {
	tmpDir := t.TempDir()

	scenario := SpecScenario{
		Name:  "test-scenario",
		Given: "some state",
		When:  "action is taken",
		Then:  "expected outcome",
	}

	// Track the prompt passed to the invoker.
	var capturedPrompt string
	invoker := &mockInvoker{
		response: "===TEST_FILE_PATH===\ntest_test.go\n===TEST_FILE_CONTENT===\npackage test\n===END_TEST_FILE===\n",
		invokeCallFunc: func(ctx context.Context, prompt string) {
			capturedPrompt = prompt
		},
	}

	writer := NewLLMScenarioTestWriter(invoker, "# Patterns\n")

	compileErrors := "undefined: SomeFunction\ntype mismatch: expected int, got string"
	_, err := writer.WriteScenarioTest(context.Background(), scenario, []string{}, tmpDir, compileErrors)
	if err != nil {
		t.Fatalf("WriteScenarioTest failed: %v", err)
	}

	// Verify compile errors are in the prompt.
	if !strings.Contains(capturedPrompt, "Prior Compilation Errors") {
		t.Error("prompt missing 'Prior Compilation Errors' section")
	}
	if !strings.Contains(capturedPrompt, compileErrors) {
		t.Errorf("compile errors not found in prompt.\nerrors: %s\nprompt: %s", compileErrors, capturedPrompt)
	}
}

// TestLLMScenarioTestWriter_ParsesFilePath tests that file paths are correctly extracted from the LLM response.
func TestLLMScenarioTestWriter_ParsesFilePath(t *testing.T) {
	tmpDir := t.TempDir()

	scenario := SpecScenario{Name: "test", Given: "", When: "", Then: ""}

	// Test with a deeply nested file path.
	testFilePath := "internal/pkg/nested/deep/package_scenario_test_test.go"
	mockResponse := "===TEST_FILE_PATH===\n" + testFilePath + "\n===TEST_FILE_CONTENT===\npackage test\nfunc Test(){}\n===END_TEST_FILE===\n"

	invoker := &mockInvoker{response: mockResponse}
	writer := NewLLMScenarioTestWriter(invoker, "")

	result, err := writer.WriteScenarioTest(context.Background(), scenario, []string{}, tmpDir, "")
	if err != nil {
		t.Fatalf("WriteScenarioTest failed: %v", err)
	}

	if result != testFilePath {
		t.Errorf("expected parsed path %q, got %q", testFilePath, result)
	}

	// Verify the file was created in the correct directory structure.
	fullPath := filepath.Join(tmpDir, testFilePath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		t.Errorf("file was not created at expected path: %s", fullPath)
	}
}

// TestLLMScenarioTestWriter_IncludesPatterns verifies that scenario test patterns content is included in the prompt.
func TestLLMScenarioTestWriter_IncludesPatterns(t *testing.T) {
	tmpDir := t.TempDir()

	scenario := SpecScenario{Name: "test", Given: "", When: "", Then: ""}

	// Capture the prompt.
	var capturedPrompt string
	invoker := &mockInvoker{
		response: "===TEST_FILE_PATH===\ntest_test.go\n===TEST_FILE_CONTENT===\npackage test\n===END_TEST_FILE===\n",
		invokeCallFunc: func(ctx context.Context, prompt string) {
			capturedPrompt = prompt
		},
	}

	patternsDoc := `# Scenario Test Patterns

## Seed
Create test fixtures and initial state.

## Invoke
Call the function under test.

## Assert
Verify the expected outcomes using assertions.
`

	writer := NewLLMScenarioTestWriter(invoker, patternsDoc)

	_, err := writer.WriteScenarioTest(context.Background(), scenario, []string{}, tmpDir, "")
	if err != nil {
		t.Fatalf("WriteScenarioTest failed: %v", err)
	}

	// Verify patterns content is in the prompt.
	if !strings.Contains(capturedPrompt, "Scenario Test Patterns") {
		t.Error("prompt missing 'Scenario Test Patterns' section header")
	}
	if !strings.Contains(capturedPrompt, "Seed") {
		t.Error("patterns content 'Seed' not found in prompt")
	}
	if !strings.Contains(capturedPrompt, "Invoke") {
		t.Error("patterns content 'Invoke' not found in prompt")
	}
	if !strings.Contains(capturedPrompt, "Assert") {
		t.Error("patterns content 'Assert' not found in prompt")
	}
}

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

func TestWriteScenarioTest_AbsolutePathMatchingWorkDir_ReturnsError(t *testing.T) {
	workDir := t.TempDir()
	relPath := "internal/foo/foo_scenario_bar_test.go"
	absPath := filepath.Join(workDir, relPath)

	invoker := &stubInvoker{output: fmt.Sprintf(
		"===TEST_FILE_PATH===\n%s\n===TEST_FILE_CONTENT===\npackage foo\n===END_TEST_FILE===\n",
		absPath,
	)}
	w := NewLLMScenarioTestWriter(invoker, "")

	_, err := w.WriteScenarioTest(context.Background(), SpecScenario{Name: "bar"}, nil, workDir, "")
	if err == nil {
		t.Fatal("expected error for absolute path, got nil")
	}
}

func TestWriteScenarioTest_AbsolutePathOutsideWorkDir_ReturnsError(t *testing.T) {
	workDir := t.TempDir()
	outsidePath := "/etc/passwd"

	invoker := &stubInvoker{output: fmt.Sprintf(
		"===TEST_FILE_PATH===\n%s\n===TEST_FILE_CONTENT===\npackage foo\n===END_TEST_FILE===\n",
		outsidePath,
	)}
	w := NewLLMScenarioTestWriter(invoker, "")

	_, err := w.WriteScenarioTest(context.Background(), SpecScenario{Name: "bar"}, nil, workDir, "")
	if err == nil {
		t.Fatal("expected error for absolute path outside workDir, got nil")
	}
}

func TestWriteScenarioTest_RelativePath_Unchanged(t *testing.T) {
	workDir := t.TempDir()
	relPath := "internal/foo/foo_scenario_bar_test.go"

	invoker := &stubInvoker{output: fmt.Sprintf(
		"===TEST_FILE_PATH===\n%s\n===TEST_FILE_CONTENT===\npackage foo\n===END_TEST_FILE===\n",
		relPath,
	)}
	w := NewLLMScenarioTestWriter(invoker, "")

	got, err := w.WriteScenarioTest(context.Background(), SpecScenario{Name: "bar"}, nil, workDir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != relPath {
		t.Errorf("got %q, want %q", got, relPath)
	}
}

func TestWriteScenarioTest_RelativeTraversalPath_ReturnsError(t *testing.T) {
	workDir := t.TempDir()
	traversalPath := "../../etc/passwd"

	invoker := &stubInvoker{output: fmt.Sprintf(
		"===TEST_FILE_PATH===\n%s\n===TEST_FILE_CONTENT===\npackage foo\n===END_TEST_FILE===\n",
		traversalPath,
	)}
	w := NewLLMScenarioTestWriter(invoker, "")

	_, err := w.WriteScenarioTest(context.Background(), SpecScenario{Name: "bar"}, nil, workDir, "")
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
}
