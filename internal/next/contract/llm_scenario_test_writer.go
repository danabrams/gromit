package contract

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/next/llmadapter"
)

// LLMScenarioTestWriter implements ScenarioTestWriter using an LLM invoker.
// It receives scenario details and generates test files following the Seed/Invoke/Assert pattern.
type LLMScenarioTestWriter struct {
	invoker              llmadapter.Invoker
	scenarioTestPatterns string
}

// NewLLMScenarioTestWriter creates a new LLMScenarioTestWriter.
// The scenarioTestPatterns parameter should contain the content of docs/scenario-tests.md,
// which serves as system prompt guidance for test writing.
func NewLLMScenarioTestWriter(invoker llmadapter.Invoker, scenarioTestPatterns string) *LLMScenarioTestWriter {
	return &LLMScenarioTestWriter{
		invoker:              invoker,
		scenarioTestPatterns: scenarioTestPatterns,
	}
}

// WriteScenarioTest generates a test file for the given scenario.
// It reads implementation files, builds a prompt, invokes the LLM,
// parses the response, and writes the test file to workDir.
// Returns the path to the written test file (relative to workDir).
func (w *LLMScenarioTestWriter) WriteScenarioTest(ctx context.Context, scenario SpecScenario, implFiles []string, workDir string, compileErrors string) (string, error) {
	// Read implementation file contents.
	implFilesContent := make(map[string]string)
	for _, implFile := range implFiles {
		absPath := filepath.Join(workDir, implFile)
		content, err := os.ReadFile(absPath)
		if err != nil {
			// Skip files that can't be read (e.g., deleted or inaccessible).
			continue
		}
		implFilesContent[implFile] = string(content)
	}

	// Build the prompt.
	prompt := w.buildPrompt(scenario, implFilesContent, compileErrors)

	// Invoke the LLM.
	result, err := w.invoker.Invoke(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("invoke llm: %w", err)
	}
	if result == nil {
		return "", fmt.Errorf("scenario test writer: provider returned nil result")
	}

	// Parse the response to extract test file path and content.
	testFilePath, testContent, err := parseScenarioTestResponse(result.Output)
	if err != nil {
		return "", fmt.Errorf("parse scenario test response: %w", err)
	}

	// Write the test file to workDir.
	absTestPath := filepath.Join(workDir, testFilePath)
	if err := os.MkdirAll(filepath.Dir(absTestPath), 0o755); err != nil {
		return "", fmt.Errorf("create test file directory: %w", err)
	}
	if err := os.WriteFile(absTestPath, []byte(testContent), 0o644); err != nil {
		return "", fmt.Errorf("write test file: %w", err)
	}

	return testFilePath, nil
}

// buildPrompt constructs the prompt for the LLM to write a scenario test.
func (w *LLMScenarioTestWriter) buildPrompt(scenario SpecScenario, implFilesContent map[string]string, compileErrors string) string {
	var sb strings.Builder

	sb.WriteString("You are writing a Go test file for a scenario in a CLI application.\n\n")

	sb.WriteString("# Scenario\n\n")
	sb.WriteString("**Name:** " + scenario.Name + "\n\n")
	sb.WriteString("**Given:** " + scenario.Given + "\n\n")
	sb.WriteString("**When:** " + scenario.When + "\n\n")
	sb.WriteString("**Then:** " + scenario.Then + "\n\n")
	if scenario.Notes != "" {
		sb.WriteString("**Notes:** " + scenario.Notes + "\n\n")
	}

	sb.WriteString("# Implementation Files\n\n")
	for path, content := range implFilesContent {
		sb.WriteString("## " + path + "\n\n")
		sb.WriteString("```go\n")
		sb.WriteString(content)
		sb.WriteString("\n```\n\n")
	}

	if compileErrors != "" {
		sb.WriteString("# Prior Compilation Errors\n\n")
		sb.WriteString("Please fix these compilation errors in your next attempt:\n\n")
		sb.WriteString(compileErrors)
		sb.WriteString("\n\n")
	}

	sb.WriteString("# Scenario Test Patterns\n\n")
	sb.WriteString(w.scenarioTestPatterns)
	sb.WriteString("\n\n")

	sb.WriteString("# Instructions\n\n")
	sb.WriteString("Write a single Go test file following the Seed/Invoke/Assert pattern:\n\n")
	sb.WriteString("1. **Seed**: Create a runstore.Store in t.TempDir() and populate it with RunState objects.\n")
	sb.WriteString("2. **Invoke**: Call the internal function or CLI command directly (preferred) or via cobra.\n")
	sb.WriteString("3. **Assert**: Use strings.Contains for presence checks and avoid asserting exact whitespace.\n\n")

	sb.WriteString("Requirements:\n")
	sb.WriteString("- Place the test file in the same package as the code under test.\n")
	sb.WriteString("- Use a dedicated file name like <pkg>_scenario_<name>_test.go.\n")
	sb.WriteString("- The test should compile and follow Go conventions.\n")
	sb.WriteString("- Output the result in the format below.\n\n")

	sb.WriteString("# Output Format\n\n")
	sb.WriteString("You MUST output your response in exactly this format:\n\n")
	sb.WriteString("===TEST_FILE_PATH===\n")
	sb.WriteString("path/to/package/package_scenario_name_test.go\n")
	sb.WriteString("===TEST_FILE_CONTENT===\n")
	sb.WriteString("package packagename\n\n")
	sb.WriteString("import (\n")
	sb.WriteString("    \"testing\"\n")
	sb.WriteString("    ...\n")
	sb.WriteString(")\n\n")
	sb.WriteString("func TestScenario_SomeName(t *testing.T) {\n")
	sb.WriteString("    // Seed\n")
	sb.WriteString("    // Invoke\n")
	sb.WriteString("    // Assert\n")
	sb.WriteString("}\n")
	sb.WriteString("===END_TEST_FILE===\n\n")

	sb.WriteString("Begin writing the test file now.\n")

	return sb.String()
}

// parseScenarioTestResponse extracts the test file path and content from the LLM response.
// Expected format:
//
// ===TEST_FILE_PATH===
// path/to/test_file.go
// ===TEST_FILE_CONTENT===
// <file content here>
// ===END_TEST_FILE===
func parseScenarioTestResponse(response string) (string, string, error) {
	pathStart := strings.Index(response, "===TEST_FILE_PATH===")
	if pathStart == -1 {
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

	// Extract test file path (between first and second marker).
	pathContent := response[pathStart+len("===TEST_FILE_PATH===") : contentStart]
	testFilePath := strings.TrimSpace(pathContent)

	// Extract test file content (between second and third marker).
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
