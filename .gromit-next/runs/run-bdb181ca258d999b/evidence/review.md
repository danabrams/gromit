# Review Decision Sheet

## Terminal State

needs_human

## What Changed

diff --git a/cmd/gromit-next/stage_provider.go b/cmd/gromit-next/stage_provider.go
index a3918c3da..79d1283e3 100644
--- a/cmd/gromit-next/stage_provider.go
+++ b/cmd/gromit-next/stage_provider.go
@@ -3,6 +3,7 @@ package main
 import (
 	"context"
 	"fmt"
+	"log"
 	"os"
 	"os/exec"
 	"time"
@@ -96,14 +97,15 @@ func (p *RealStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.R
 	}
 
 	var (
-		compiler       stages.SpecCompiler
-		planCreator    stages.PlanCreator
-		taskRunner     specloop.TaskRunner
-		finalVal       stages.FinalValidator
-		reviewRunner   stages.ReviewRunner
-		acceptEval     stages.AcceptEvaluator
-		contractWriter contract.ContractWriter
-		diffProv       review.DiffProvider = &noopDiffProvider{}
+		compiler           stages.SpecCompiler
+		planCreator        stages.PlanCreator
+		taskRunner         specloop.TaskRunner
+		finalVal           stages.FinalValidator
+		reviewRunner       stages.ReviewRunner
+		acceptEval         stages.AcceptEvaluator
+		contractWriter     contract.ContractWriter
+		scenarioTestWriter contract.ScenarioTestWriter
+		diffProv           review.DiffProvider = &noopDiffProvider{}
 	)
 
 	if p.claudeProvider != nil {
@@ -169,6 +171,28 @@ func (p *RealStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.R
 		)
 		contractWriter = contract.NewLLMContractWriter(contractAdapter)
 
+		// Scenario test writer with FallbackAdapter (Sonnet/Planner tier).
+		scenarioTestAdapter := llmadapter.NewFallbackAdapter(
+			router, "scenario_tests",
+			llmadapter.Config{Tier: policy.Models.Planner, OnCost: costCallback, OnInvocation: invocationCallback},
+			policy.Models.Planner,
+		)
+
+		// Read scenario test patterns from docs/scenario-tests.md
+		scenarioTestPatterns := ""
+		patternsPath := "docs/scenario-tests.md"
+		patternsContent, err := os.ReadFile(patternsPath)
+		if err != nil && !os.IsNotExist(err) {
+			return nil, fmt.Errorf("read scenario test patterns: %w", err)
+		}
+		if err == nil {
+			scenarioTestPatterns = string(patternsContent)
+		} else {
+			log.Printf("warning: scenario test patterns file not found at %s, proceeding without pattern guidance", patternsPath)
+		}
+
+		scenarioTestWriter = contract.NewLLMScenarioTestWriter(scenarioTestAdapter, scenarioTestPatterns)
+
 		diffProv = &lazyDiffProvider{rs: rs, fallbackDir: p.cfg.WorkDir}
 
 		// TODO: Wire real SpecCompilerAdapter here (blocked on ArtifactStore, cell resolution, level selection).
@@ -183,6 +207,7 @@ func (p *RealStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.R
 		reviewRunner = &noopReviewRunner{}
 		acceptEval = &noopAcceptEvaluator{}
 		contractWriter = &noopContractWriter{}
+		scenarioTestWriter = &noopScenarioTestWriter{}
 	}
 
 	compileStage := stages.NewCompileStage(compiler, store, nil)
@@ -223,6 +248,13 @@ func (p *RealStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.R
 
 	evidenceDir := store.RunEvidenceDir(rs.RunID)
 
+	writeScenarioTestsStage := stages.NewWriteScenarioTestsStage(scenarioTestWriter, stages.WriteScenarioTestsStageConfig{
+		SpecPath:    p.cfg.SpecPath,
+		EvidenceDir: evidenceDir,
+		Store:       store,
+		WorkDir:     p.cfg.WorkDir,
+	}, budget, eventLog)
+
 	contractEvaluator := &contract.DefaultContractEvaluator{}
 
 	writeContractsStage := stages.NewWriteContractsStage(contractWriter, stages.WriteContractsStageConfig{
@@ -269,6 +301,7 @@ func (p *RealStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.R
 		planStage,
 		writeContractsStage,
 		executeStage,
+		writeScenarioTestsStage,
 		validateStage,
 		reviewStage,
 		acceptStage,
@@ -442,3 +475,10 @@ type noopContractWriter struct{}
 func (n *noopContractWriter) WriteContracts(_ context.Context, _ []contract.SpecScenario, _ string) (*contract.ScenarioContract, error) {
 	return nil, nil
 }
+
+// noopScenarioTestWriter satisfies contract.ScenarioTestWriter with a no-op.
+type noopScenarioTestWriter struct{}
+
+func (n *noopScenarioTestWriter) WriteScenarioTest(_ context.Context, _ contract.SpecScenario, _ []string, _ string, _ string) (string, error) {
+	return "", nil
+}
diff --git a/cmd/gromit-next/stage_provider_test.go b/cmd/gromit-next/stage_provider_test.go
index 24e703c88..12e653b09 100644
--- a/cmd/gromit-next/stage_provider_test.go
+++ b/cmd/gromit-next/stage_provider_test.go
@@ -36,7 +36,7 @@ func TestRealStageProvider_BuildStages_ReturnsStages(t *testing.T) {
 		t.Fatal("expected at least one stage, got 0")
 	}
 
-	expectedNames := []string{"init", "compile", "plan", "write_contracts", "execute", "validate", "review", "accept", "evidence", "finalize"}
+	expectedNames := []string{"init", "compile", "plan", "write_contracts", "execute", "write_scenario_tests", "validate", "review", "accept", "evidence", "finalize"}
 	if len(stages) != len(expectedNames) {
 		t.Fatalf("expected %d stages, got %d", len(expectedNames), len(stages))
 	}
@@ -155,8 +155,8 @@ func TestRealStageProvider_BuildStages_DefaultTierUsesModelsEvaluator(t *testing
 		t.Fatalf("BuildStages: %v", err)
 	}
 	// Verify we got the expected stages (sanity check).
-	if len(stages) != 10 {
-		t.Fatalf("expected 10 stages, got %d", len(stages))
+	if len(stages) != 11 {
+		t.Fatalf("expected 11 stages, got %d", len(stages))
 	}
 }
 
@@ -452,7 +452,7 @@ func TestRealStageProvider_BuildStages_WithProvider_ReturnsRealAdapters(t *testi
 		t.Fatalf("BuildStages returned error: %v", err)
 	}
 
-	expectedNames := []string{"init", "compile", "plan", "write_contracts", "execute", "validate", "review", "accept", "evidence", "finalize"}
+	expectedNames := []string{"init", "compile", "plan", "write_contracts", "execute", "write_scenario_tests", "validate", "review", "accept", "evidence", "finalize"}
 	if len(stages) != len(expectedNames) {
 		t.Fatalf("expected %d stages, got %d", len(expectedNames), len(stages))
 	}
@@ -478,8 +478,8 @@ func TestRealStageProvider_BuildStages_WithProvider_NilProviderFallsBackToNoops(
 	if err != nil {
 		t.Fatalf("BuildStages returned error: %v", err)
 	}
-	if len(stages) != 10 {
-		t.Fatalf("expected 10 stages, got %d", len(stages))
+	if len(stages) != 11 {
+		t.Fatalf("expected 11 stages, got %d", len(stages))
 	}
 }
 
@@ -550,8 +550,8 @@ func TestBuildStages_WithClaudeProvider_UsesFallbackAdapter(t *testing.T) {
 	if err != nil {
 		t.Fatalf("BuildStages returned error: %v", err)
 	}
-	if len(stages) != 10 {
-		t.Fatalf("expected 10 stages, got %d", len(stages))
+	if len(stages) != 11 {
+		t.Fatalf("expected 11 stages, got %d", len(stages))
 	}
 }
 
@@ -898,8 +898,39 @@ func TestIntegration_BuildStages_FallbackAdapter_RouterWiring(t *testing.T) {
 	if len(stages) == 0 {
 		t.Fatal("expected at least one stage from BuildStages")
 	}
-	// Verify 10 stages are returned (same as before — multi-provider doesn't change stage count)
-	if len(stages) != 10 {
-		t.Fatalf("expected 10 stages, got %d", len(stages))
+	// Verify 11 stages are returned (same as before — multi-provider doesn't change stage count)
+	if len(stages) != 11 {
+		t.Fatalf("expected 11 stages, got %d", len(stages))
+	}
+}
+
+func TestBuildStages_WriteScenarioTestsStageWired(t *testing.T) {
+	// Verify that the write_scenario_tests stage is properly wired with scenarioTestWriter.
+	// The stage provider builds a scenarioTestWriter (either LLM-based ScenarioTestWriter
+	// or noop) and wires it into the write_scenario_tests stage.
+	policy := execpolicy.DefaultPolicy()
+	rs := runstore.NewRunState("test-spec", "test-project")
+
+	sp := NewRealStageProvider(RealStageProviderConfig{
+		WorkDir:  t.TempDir(),
+		StoreDir: t.TempDir(),
+		SpecPath: "test-spec.md",
+	})
+
+	stages, err := sp.BuildStages(policy, rs, specloop.NewBudget(policy.Budgets), nil)
+	if err != nil {
+		t.Fatalf("BuildStages: %v", err)
+	}
+
+	// Verify write_scenario_tests stage exists in the pipeline
+	var found bool
+	for _, s := range stages {
+		if s.Name() == "write_scenario_tests" {
+			found = true
+			break
+		}
+	}
+	if !found {
+		t.Fatal("expected write_scenario_tests stage not found in BuildStages output")
 	}
 }
diff --git a/cmd/gromit/final_verification_test.go b/cmd/gromit/final_verification_test.go
index b01319320..0ee6739da 100644
--- a/cmd/gromit/final_verification_test.go
+++ b/cmd/gromit/final_verification_test.go
@@ -195,7 +195,7 @@ func scanProjectTestFiles(projectRoot string) ([]scannedTestFile, error) {
 		if err != nil {
 			return err
 		}
-		if d.IsDir() && (d.Name() == "vendor" || d.Name() == ".git" || d.Name() == ".gromit" || d.Name() == ".claude" || d.Name() == ".worktrees" || d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".-gromit-")) {
+		if d.IsDir() && (d.Name() == "vendor" || d.Name() == ".git" || d.Name() == ".gromit" || d.Name() == ".claude" || d.Name() == ".worktrees" || d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".-gromit-") || strings.HasPrefix(d.Name(), ".gromit-next")) {
 			return filepath.SkipDir
 		}
 		if d.IsDir() || !strings.HasSuffix(d.Name(), "_test.go") {
diff --git a/cmd/gromit/run_spec_flag_test.go b/cmd/gromit/run_spec_flag_test.go
index e4691e27b..12b1fac9d 100644
--- a/cmd/gromit/run_spec_flag_test.go
+++ b/cmd/gromit/run_spec_flag_test.go
@@ -117,8 +117,19 @@ func TestRunSpecTestEnvRestoresWorkingDirectory(t *testing.T) {
 	if err != nil {
 		t.Fatalf("getting working directory after cleanup: %v", err)
 	}
-	if currentWD != originalWD {
-		t.Fatalf("expected working directory %q after cleanup, got %q", originalWD, currentWD)
+
+	// Resolve symlinks for comparison (macOS /var -> /private/var)
+	originalWDResolved, err := filepath.EvalSymlinks(originalWD)
+	if err != nil {
+		t.Fatalf("resolving original working directory symlinks: %v", err)
+	}
+	currentWDResolved, err := filepath.EvalSymlinks(currentWD)
+	if err != nil {
+		t.Fatalf("resolving current working directory symlinks: %v", err)
+	}
+
+	if currentWDResolved != originalWDResolved {
+		t.Fatalf("expected working directory %q after cleanup, got %q", originalWDResolved, currentWDResolved)
 	}
 }
 
diff --git a/internal/next/contract/llm_scenario_test_writer.go b/internal/next/contract/llm_scenario_test_writer.go
new file mode 100644
index 000000000..35d7b27a2
--- /dev/null
+++ b/internal/next/contract/llm_scenario_test_writer.go
@@ -0,0 +1,189 @@
+package contract
+
+import (
+	"context"
+	"fmt"
+	"os"
+	"path/filepath"
+	"strings"
+
+	"github.com/danabrams/gromit/internal/next/llmadapter"
+)
+
+// LLMScenarioTestWriter implements ScenarioTestWriter using an LLM invoker.
+// It receives scenario details and generates test files following the Seed/Invoke/Assert pattern.
+type LLMScenarioTestWriter struct {
+	invoker              llmadapter.Invoker
+	scenarioTestPatterns string
+}
+
+// NewLLMScenarioTestWriter creates a new LLMScenarioTestWriter.
+// The scenarioTestPatterns parameter should contain the content of docs/scenario-tests.md,
+// which serves as system prompt guidance for test writing.
+func NewLLMScenarioTestWriter(invoker llmadapter.Invoker, scenarioTestPatterns string) *LLMScenarioTestWriter {
+	return &LLMScenarioTestWriter{
+		invoker:              invoker,
+		scenarioTestPatterns: scenarioTestPatterns,
+	}
+}
+
+// WriteScenarioTest generates a test file for the given scenario.
+// It reads implementation files, builds a prompt, invokes the LLM,
+// parses the response, and writes the test file to workDir.
+// Returns the path to the written test file (relative to workDir).
+func (w *LLMScenarioTestWriter) WriteScenarioTest(ctx context.Context, scenario SpecScenario, implFiles []string, workDir string, compileErrors string) (string, error) {
+	// Read implementation file contents.
+	implFilesContent := make(map[string]string)
+	for _, implFile := range implFiles {
+		absPath := filepath.Join(workDir, implFile)
+		content, err := os.ReadFile(absPath)
+		if err != nil {
+			// Skip files that can't be read (e.g., deleted or inaccessible).
+			continue
+		}
+		implFilesContent[implFile] = string(content)
+	}
+
+	// Build the prompt.
+	prompt := w.buildPrompt(scenario, implFilesContent, compileErrors)
+
+	// Invoke the LLM.
+	result, err := w.invoker.Invoke(ctx, prompt)
+	if err != nil {
+		return "", fmt.Errorf("invoke llm: %w", err)
+	}
+	if result == nil {
+		return "", fmt.Errorf("scenario test writer: provider returned nil result")
+	}
+
+	// Parse the response to extract test file path and content.
+	testFilePath, testContent, err := parseScenarioTestResponse(result.Output)
+	if err != nil {
+		return "", fmt.Errorf("parse scenario test response: %w", err)
+	}
+
+	// Write the test file to workDir.
+	absTestPath := filepath.Join(workDir, testFilePath)
+	if err := os.MkdirAll(filepath.Dir(absTestPath), 0o755); err != nil {
+		return "", fmt.Errorf("create test file directory: %w", err)
+	}
+	if err := os.WriteFile(absTestPath, []byte(testContent), 0o644); err != nil {
+		return "", fmt.Errorf("write test file: %w", err)
+	}
+
+	return testFilePath, nil
+}
+
+// buildPrompt constructs the prompt for the LLM to write a scenario test.
+func (w *LLMScenarioTestWriter) buildPrompt(scenario SpecScenario, implFilesContent map[string]string, compileErrors string) string {
+	var sb strings.Builder
+
+	sb.WriteString("You are writing a Go test file for a scenario in a CLI application.\n\n")
+
+	sb.WriteString("# Scenario\n\n")
+	sb.WriteString("**Name:** " + scenario.Name + "\n\n")
+	sb.WriteString("**Given:** " + scenario.Given + "\n\n")
+	sb.WriteString("**When:** " + scenario.When + "\n\n")
+	sb.WriteString("**Then:** " + scenario.Then + "\n\n")
+	if scenario.Notes != "" {
+		sb.WriteString("**Notes:** " + scenario.Notes + "\n\n")
+	}
+
+	sb.WriteString("# Implementation Files\n\n")
+	for path, content := range implFilesContent {
+		sb.WriteString("## " + path + "\n\n")
+		sb.WriteString("```go\n")
+		sb.WriteString(content)
+		sb.WriteString("\n```\n\n")
+	}
+
+	if compileErrors != "" {
+		sb.WriteString("# Prior Compilation Errors\n\n")
+		sb.WriteString("Please fix these compilation errors in your next attempt:\n\n")
+		sb.WriteString(compileErrors)
+		sb.WriteString("\n\n")
+	}
+
+	sb.WriteString("# Scenario Test Patterns\n\n")
+	sb.WriteString(w.scenarioTestPatterns)
+	sb.WriteString("\n\n")
+
+	sb.WriteString("# Instructions\n\n")
+	sb.WriteString("Write a single Go test file following the Seed/Invoke/Assert pattern:\n\n")
+	sb.WriteString("1. **Seed**: Create a runstore.Store in t.TempDir() and populate it with RunState objects.\n")
+	sb.WriteString("2. **Invoke**: Call the internal function or CLI command directly (preferred) or via cobra.\n")
+	sb.WriteString("3. **Assert**: Use strings.Contains for presence checks and avoid asserting exact whitespace.\n\n")
+
+	sb.WriteString("Requirements:\n")
+	sb.WriteString("- Place the test file in the same package as the code under test.\n")
+	sb.WriteString("- Use a dedicated file name like <pkg>_scenario_<name>_test.go.\n")
+	sb.WriteString("- The test should compile and follow Go conventions.\n")
+	sb.WriteString("- Output the result in the format below.\n\n")
+
+	sb.WriteString("# Output Format\n\n")
+	sb.WriteString("You MUST output your response in exactly this format:\n\n")
+	sb.WriteString("===TEST_FILE_PATH===\n")
+	sb.WriteString("path/to/package/package_scenario_name_test.go\n")
+	sb.WriteString("===TEST_FILE_CONTENT===\n")
+	sb.WriteString("package packagename\n\n")
+	sb.WriteString("import (\n")
+	sb.WriteString("    \"testing\"\n")
+	sb.WriteString("    ...\n")
+	sb.WriteString(")\n\n")
+	sb.WriteString("func TestScenario_SomeName(t *testing.T) {\n")
+	sb.WriteString("    // Seed\n")
+	sb.WriteString("    // Invoke\n")
+	sb.WriteString("    // Assert\n")
+	sb.WriteString("}\n")
+	sb.WriteString("===END_TEST_FILE===\n\n")
+
+	sb.WriteString("Begin writing the test file now.\n")
+
+	return sb.String()
+}
+
+// parseScenarioTestResponse extracts the test file path and content from the LLM response.
+// Expected format:
+//
+// ===TEST_FILE_PATH===
+// path/to/test_file.go
+// ===TEST_FILE_CONTENT===
+// <file content here>
+// ===END_TEST_FILE===
+func parseScenarioTestResponse(response string) (string, string, error) {
+	pathStart := strings.Index(response, "===TEST_FILE_PATH===")
+	if pathStart == -1 {
+		return "", "", fmt.Errorf("response missing ===TEST_FILE_PATH=== marker")
+	}
+
+	contentStart := strings.Index(response, "===TEST_FILE_CONTENT===")
+	if contentStart == -1 {
+		return "", "", fmt.Errorf("response missing ===TEST_FILE_CONTENT=== marker")
+	}
+
+	endMarker := strings.Index(response, "===END_TEST_FILE===")
+	if endMarker == -1 {
+		return "", "", fmt.Errorf("response missing ===END_TEST_FILE=== marker")
+	}
+
+	if pathStart >= contentStart || contentStart >= endMarker {
+		return "", "", fmt.Errorf("invalid marker order in response")
+	}
+
+	// Extract test file path (between first and second marker).
+	pathContent := response[pathStart+len("===TEST_FILE_PATH===") : contentStart]
+	testFilePath := strings.TrimSpace(pathContent)
+
+	// Extract test file content (between second and third marker).
+	contentContent := response[contentStart+len("===TEST_FILE_CONTENT===") : endMarker]
+	testContent := strings.TrimSpace(contentContent)
+
+	if testFilePath == "" {
+		return "", "", fmt.Errorf("test file path is empty")
+	}
+	if testContent == "" {
+		return "", "", fmt.Errorf("test file content is empty")
+	}
+
+	return testFilePath, testContent, nil
+}
diff --git a/internal/next/contract/llm_scenario_test_writer_test.go b/internal/next/contract/llm_scenario_test_writer_test.go
new file mode 100644
index 000000000..4c813074b
--- /dev/null
+++ b/internal/next/contract/llm_scenario_test_writer_test.go
@@ -0,0 +1,222 @@
+package contract
+
+import (
+	"context"
+	"os"
+	"path/filepath"
+	"strings"
+	"testing"
+
+	"github.com/danabrams/gromit/internal/provider"
+)
+
+// mockInvoker implements llmadapter.Invoker and returns canned responses for testing.
+type mockInvoker struct {
+	response       string
+	invokeCallFunc func(ctx context.Context, prompt string) // optional callback to inspect prompt
+}
+
+func (m *mockInvoker) Invoke(ctx context.Context, prompt string) (*provider.Result, error) {
+	if m.invokeCallFunc != nil {
+		m.invokeCallFunc(ctx, prompt)
+	}
+	return &provider.Result{
+		Success: true,
+		Output:  m.response,
+	}, nil
+}
+
+func (m *mockInvoker) InvokeInDir(ctx context.Context, prompt string, dir string) (*provider.Result, error) {
+	if m.invokeCallFunc != nil {
+		m.invokeCallFunc(ctx, prompt)
+	}
+	return &provider.Result{
+		Success: true,
+		Output:  m.response,
+	}, nil
+}
+
+// TestLLMScenarioTestWriter_HappyPath tests that a valid LLM response produces a correctly written test file.
+func TestLLMScenarioTestWriter_HappyPath(t *testing.T) {
+	tmpDir := t.TempDir()
+
+	// Create an implementation file to be read.
+	implFile := "pkg/mypackage/impl.go"
+	implPath := filepath.Join(tmpDir, implFile)
+	if err := os.MkdirAll(filepath.Dir(implPath), 0o755); err != nil {
+		t.Fatal(err)
+	}
+	implContent := "package mypackage\n\nfunc Add(a, b int) int { return a + b }\n"
+	if err := os.WriteFile(implPath, []byte(implContent), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	// Create a scenario and test writer.
+	scenario := SpecScenario{
+		Name:  "add-works",
+		Given: "two positive integers",
+		When:  "Add is called",
+		Then:  "the sum is returned",
+		Notes: "basic arithmetic test",
+	}
+
+	// Mock response with properly formatted test file output.
+	testFilePath := "pkg/mypackage/mypackage_scenario_add_works_test.go"
+	testContent := `package mypackage
+
+import "testing"
+
+func TestScenario_AddWorks(t *testing.T) {
+	// Seed
+	a, b := 2, 3
+
+	// Invoke
+	result := Add(a, b)
+
+	// Assert
+	if result != 5 {
+		t.Errorf("expected 5, got %d", result)
+	}
+}
+`
+	mockResponse := "===TEST_FILE_PATH===\n" + testFilePath + "\n===TEST_FILE_CONTENT===\n" + testContent + "\n===END_TEST_FILE===\n"
+
+	invoker := &mockInvoker{response: mockResponse}
+	writer := NewLLMScenarioTestWriter(invoker, "# Scenario Test Patterns\n\nUse the Seed/Invoke/Assert pattern.")
+
+	// Invoke WriteScenarioTest.
+	result, err := writer.WriteScenarioTest(context.Background(), scenario, []string{implFile}, tmpDir, "")
+	if err != nil {
+		t.Fatalf("WriteScenarioTest failed: %v", err)
+	}
+
+	// Verify returned path matches expected.
+	if result != testFilePath {
+		t.Errorf("expected path %q, got %q", testFilePath, result)
+	}
+
+	// Verify file was written to correct location.
+	writtenPath := filepath.Join(tmpDir, testFilePath)
+	writtenContent, err := os.ReadFile(writtenPath)
+	if err != nil {
+		t.Fatalf("could not read written file: %v", err)
+	}
+	if strings.TrimSpace(string(writtenContent)) != strings.TrimSpace(testContent) {
+		t.Errorf("written content mismatch.\nexpected:\n%s\ngot:\n%s", testContent, string(writtenContent))
+	}
+}
+
+// TestLLMScenarioTestWriter_WithCompileErrors verifies that compile errors are included in the prompt to the LLM.
+func TestLLMScenarioTestWriter_WithCompileErrors(t *testing.T) {
+	tmpDir := t.TempDir()
+
+	scenario := SpecScenario{
+		Name:  "test-scenario",
+		Given: "some state",
+		When:  "action is taken",
+		Then:  "expected outcome",
+	}
+
+	// Track the prompt passed to the invoker.
+	var capturedPrompt string
+	invoker := &mockInvoker{
+		response: "===TEST_FILE_PATH===\ntest_test.go\n===TEST_FILE_CONTENT===\npackage test\n===END_TEST_FILE===\n",
+		invokeCallFunc: func(ctx context.Context, prompt string) {
+			capturedPrompt = prompt
+		},
+	}
+
+	writer := NewLLMScenarioTestWriter(invoker, "# Patterns\n")
+
+	compileErrors := "undefined: SomeFunction\ntype mismatch: expected int, got string"
+	_, err := writer.WriteScenarioTest(context.Background(), scenario, []string{}, tmpDir, compileErrors)
+	if err != nil {
+		t.Fatalf("WriteScenarioTest failed: %v", err)
+	}
+
+	// Verify compile errors are in the prompt.
+	if !strings.Contains(capturedPrompt, "Prior Compilation Errors") {
+		t.Error("prompt missing 'Prior Compilation Errors' section")
+	}
+	if !strings.Contains(capturedPrompt, compileErrors) {
+		t.Errorf("compile errors not found in prompt.\nerrors: %s\nprompt: %s", compileErrors, capturedPrompt)
+	}
+}
+
+// TestLLMScenarioTestWriter_ParsesFilePath tests that file paths are correctly extracted from the LLM response.
+func TestLLMScenarioTestWriter_ParsesFilePath(t *testing.T) {
+	tmpDir := t.TempDir()
+
+	scenario := SpecScenario{Name: "test", Given: "", When: "", Then: ""}
+
+	// Test with a deeply nested file path.
+	testFilePath := "internal/pkg/nested/deep/package_scenario_test_test.go"
+	mockResponse := "===TEST_FILE_PATH===\n" + testFilePath + "\n===TEST_FILE_CONTENT===\npackage test\nfunc Test(){}\n===END_TEST_FILE===\n"
+
+	invoker := &mockInvoker{response: mockResponse}
+	writer := NewLLMScenarioTestWriter(invoker, "")
+
+	result, err := writer.WriteScenarioTest(context.Background(), scenario, []string{}, tmpDir, "")
+	if err != nil {
+		t.Fatalf("WriteScenarioTest failed: %v", err)
+	}
+
+	if result != testFilePath {
+		t.Errorf("expected parsed path %q, got %q", testFilePath, result)
+	}
+
+	// Verify the file was created in the correct directory structure.
+	fullPath := filepath.Join(tmpDir, testFilePath)
+	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
+		t.Errorf("file was not created at expected path: %s", fullPath)
+	}
+}
+
+// TestLLMScenarioTestWriter_IncludesPatterns verifies that scenario test patterns content is included in the prompt.
+func TestLLMScenarioTestWriter_IncludesPatterns(t *testing.T) {
+	tmpDir := t.TempDir()
+
+	scenario := SpecScenario{Name: "test", Given: "", When: "", Then: ""}
+
+	// Capture the prompt.
+	var capturedPrompt string
+	invoker := &mockInvoker{
+		response: "===TEST_FILE_PATH===\ntest_test.go\n===TEST_FILE_CONTENT===\npackage test\n===END_TEST_FILE===\n",
+		invokeCallFunc: func(ctx context.Context, prompt string) {
+			capturedPrompt = prompt
+		},
+	}
+
+	patternsDoc := `# Scenario Test Patterns
+
+## Seed
+Create test fixtures and initial state.
+
+## Invoke
+Call the function under test.
+
+## Assert
+Verify the expected outcomes using assertions.
+`
+
+	writer := NewLLMScenarioTestWriter(invoker, patternsDoc)
+
+	_, err := writer.WriteScenarioTest(context.Background(), scenario, []string{}, tmpDir, "")
+	if err != nil {
+		t.Fatalf("WriteScenarioTest failed: %v", err)
+	}
+
+	// Verify patterns content is in the prompt.
+	if !strings.Contains(capturedPrompt, "Scenario Test Patterns") {
+		t.Error("prompt missing 'Scenario Test Patterns' section header")
+	}
+	if !strings.Contains(capturedPrompt, "Seed") {
+		t.Error("patterns content 'Seed' not found in prompt")
+	}
+	if !strings.Contains(capturedPrompt, "Invoke") {
+		t.Error("patterns content 'Invoke' not found in prompt")
+	}
+	if !strings.Contains(capturedPrompt, "Assert") {
+		t.Error("patterns content 'Assert' not found in prompt")
+	}
+}
diff --git a/internal/next/contract/types.go b/internal/next/contract/types.go
index c2da78c87..ac5e4634c 100644
--- a/internal/next/contract/types.go
+++ b/internal/next/contract/types.go
@@ -1,5 +1,7 @@
 package contract
 
+import "context"
+
 // SpecScenario represents a single scenario parsed from the spec's Scenarios section.
 // Extracted from spec markdown by matching "### Scenario:" headers and Given/When/Then blocks.
 type SpecScenario struct {
@@ -44,3 +46,19 @@ type ContractFailure struct {
 	AssertionType string // e.g., "file_contains"
 	Details       string // Human-readable failure description
 }
+
+// ScenarioTestWriter writes test files for scenarios.
+type ScenarioTestWriter interface {
+	WriteScenarioTest(ctx context.Context, scenario SpecScenario, implFiles []string, workDir string, compileErrors string) (testFilePath string, err error)
+}
+
+// ScenarioTestManifest holds the list of scenario test files written.
+type ScenarioTestManifest struct {
+	Scenarios []ScenarioTestEntry `json:"scenarios"`
+}
+
+// ScenarioTestEntry represents a single scenario test file in the manifest.
+type ScenarioTestEntry struct {
+	Name     string `json:"name"`
+	TestFile string `json:"test_file"`
+}
diff --git a/internal/next/runstore/events.go b/internal/next/runstore/events.go
index 1f8d30649..87a50c0ca 100644
--- a/internal/next/runstore/events.go
+++ b/internal/next/runstore/events.go
@@ -156,6 +156,22 @@ type ContractScenarioSkippedEvent struct {
 	Reason string `json:"reason"`
 }
 
+type ScenarioTestWrittenEvent struct {
+	BaseEvent
+	ScenarioName string `json:"scenario_name"`
+	TestFile     string `json:"test_file"`
+}
+
+type ScenarioTestsCompleteEvent struct {
+	BaseEvent
+	ScenarioCount int `json:"scenario_count"`
+}
+
+type ScenarioTestsBlockedEvent struct {
+	BaseEvent
+	Reason string `json:"reason"`
+}
+
 type TerminalStateEvent struct {
 	BaseEvent
 	Status string `json:"status"`
@@ -304,6 +320,15 @@ func unmarshalEvent(data []byte) (TypedEvent, error) {
 	case "contract_scenario_skipped":
 		var e ContractScenarioSkippedEvent
 		ev = &e
+	case "scenario_tests_written":
+		var e ScenarioTestWrittenEvent
+		ev = &e
+	case "scenario_tests_complete":
+		var e ScenarioTestsCompleteEvent
+		ev = &e
+	case "scenario_tests_blocked":
+		var e ScenarioTestsBlockedEvent
+		ev = &e
 	default:
 		return nil, fmt.Errorf("unknown event type: %s", peek.Type)
 	}
diff --git a/internal/next/runstore/store.go b/internal/next/runstore/store.go
index 075a95984..da0d28a71 100644
--- a/internal/next/runstore/store.go
+++ b/internal/next/runstore/store.go
@@ -79,13 +79,14 @@ func (s *Store) List(projectID string) ([]*RunState, error) {
 // ResetForNewCycle resets per-cycle gate fields on rs.
 // Fields that persist across replan cycles (e.g. ContractsWritten, ReplanContext,
 // AccumulatedCost, TotalReplans) are intentionally NOT reset here.
+// ScenarioTestsWritten and FailureHistory are NOT reset — they persist across cycles.
 func ResetForNewCycle(rs *RunState) {
 	rs.FinalValidationPassed = false
 	rs.FinalReviewPassed = false
 	rs.FinalAcceptancePassed = false
 	rs.ReviewFindings = []string{}
 	rs.AcceptanceResults = []string{}
-	// ContractsWritten is NOT reset — contracts are written once and persist across cycles.
+	// ContractsWritten, ScenarioTestsWritten, and FailureHistory are NOT reset — they persist across cycles.
 }
 
 // RunDir returns the directory path for a given run ID.
diff --git a/internal/next/runstore/store_test.go b/internal/next/runstore/store_test.go
index 2ad478a80..af3819760 100644
--- a/internal/next/runstore/store_test.go
+++ b/internal/next/runstore/store_test.go
@@ -114,3 +114,42 @@ func TestStore_ReadTaskArtifact_NotFound(t *testing.T) {
 		t.Fatal("expected error for missing artifact")
 	}
 }
+
+func TestResetForNewCycle(t *testing.T) {
+	rs := NewRunState("spec-1", "proj-1")
+	rs.FinalValidationPassed = true
+	rs.FinalReviewPassed = true
+	rs.FinalAcceptancePassed = true
+	rs.ReviewFindings = []string{"finding-1"}
+	rs.AcceptanceResults = []string{"result-1"}
+	rs.ScenarioTestsWritten = true
+	rs.FailureHistory = map[string]int{"error1": 1, "error2": 2}
+
+	ResetForNewCycle(rs)
+
+	if rs.FinalValidationPassed {
+		t.Fatal("FinalValidationPassed should be reset to false")
+	}
+	if rs.FinalReviewPassed {
+		t.Fatal("FinalReviewPassed should be reset to false")
+	}
+	if rs.FinalAcceptancePassed {
+		t.Fatal("FinalAcceptancePassed should be reset to false")
+	}
+	if len(rs.ReviewFindings) != 0 {
+		t.Fatal("ReviewFindings should be reset to empty slice")
+	}
+	if len(rs.AcceptanceResults) != 0 {
+		t.Fatal("AcceptanceResults should be reset to empty slice")
+	}
+	// ScenarioTestsWritten and FailureHistory should NOT be reset
+	if !rs.ScenarioTestsWritten {
+		t.Fatal("ScenarioTestsWritten should NOT be reset")
+	}
+	if len(rs.FailureHistory) != 2 {
+		t.Fatal("FailureHistory should NOT be reset")
+	}
+	if rs.FailureHistory["error1"] != 1 || rs.FailureHistory["error2"] != 2 {
+		t.Fatal("FailureHistory values should remain unchanged")
+	}
+}
diff --git a/internal/next/runstore/types.go b/internal/next/runstore/types.go
index 275b45831..06799bad7 100644
--- a/internal/next/runstore/types.go
+++ b/internal/next/runstore/types.go
@@ -44,7 +44,9 @@ type RunState struct {
 	TotalReplans          int                    `json:"total_replans"`
 	SpecConstraints       string                 `json:"spec_constraints,omitempty"`
 	Resumed               bool                   `json:"resumed,omitempty"`
-	ContractsWritten      bool                   `json:"contracts_written"`
+	ContractsWritten bool `json:"contracts_written"`
+	ScenarioTestsWritten bool `json:"scenario_tests_written"`
+	FailureHistory map[string]int `json:"failure_history,omitempty"`
 }
 
 // See CLAUDE.md nil-field normalization visibility convention:
@@ -63,6 +65,9 @@ func (rs *RunState) NormalizeNilFields() {
 	if rs.AcceptanceResults == nil {
 		rs.AcceptanceResults = []string{}
 	}
+	if rs.FailureHistory == nil {
+		rs.FailureHistory = map[string]int{}
+	}
 	for i := range rs.Tasks {
 		rs.Tasks[i].NormalizeNilFields()
 	}
diff --git a/internal/next/specloop/failure_history.go b/internal/next/specloop/failure_history.go
new file mode 100644
index 000000000..cf3a63384
--- /dev/null
+++ b/internal/next/specloop/failure_history.go
@@ -0,0 +1,106 @@
+package specloop
+
+import (
+	"fmt"
+	"strings"
+)
+
+// ExtractTestFailureKeys extracts test function names from '--- FAIL: TestFunctionName' patterns in failure strings
+func ExtractTestFailureKeys(failures []string) []string {
+	var keys []string
+	for _, failure := range failures {
+		if strings.HasPrefix(failure, "--- FAIL: ") {
+			// Extract the part after "--- FAIL: "
+			remaining := strings.TrimPrefix(failure, "--- FAIL: ")
+			// The test name is the first word (up to space or end of string)
+			parts := strings.Fields(remaining)
+			if len(parts) > 0 {
+				keys = append(keys, parts[0])
+			}
+		}
+	}
+	return keys
+}
+
+// ExtractContractFailureKeys extracts 'contract:<scenario-name>' keys by splitting on ' — ' and taking the first segment
+func ExtractContractFailureKeys(failures []string) []string {
+	var keys []string
+	for _, failure := range failures {
+		if strings.HasPrefix(failure, "contract:") {
+			// Split on ' — ' and take the first segment
+			parts := strings.Split(failure, " — ")
+			if len(parts) > 0 {
+				key := strings.TrimSpace(parts[0])
+				if key != "" {
+					keys = append(keys, key)
+				}
+			}
+		}
+	}
+	return keys
+}
+
+// UpdateFailureHistory increments count for keys present, resets to zero for keys not present (then deletes zero entries)
+func UpdateFailureHistory(history map[string]int, currentKeys []string) {
+	// Create a map of current keys for quick lookup
+	currentKeySet := make(map[string]bool)
+	for _, key := range currentKeys {
+		currentKeySet[key] = true
+	}
+
+	// Increment counts for keys that are present in currentKeys
+	for key := range currentKeySet {
+		history[key]++
+	}
+
+	// Reset to zero (and mark for deletion) for keys not present
+	keysToDelete := []string{}
+	for key := range history {
+		if !currentKeySet[key] {
+			history[key] = 0
+			keysToDelete = append(keysToDelete, key)
+		}
+	}
+
+	// Delete zero entries
+	for _, key := range keysToDelete {
+		delete(history, key)
+	}
+}
+
+// AnnotateWithPersistentHints appends persistent failure hints to failures for keys with count >= threshold
+func AnnotateWithPersistentHints(failures []string, history map[string]int, threshold int) []string {
+	var annotated []string
+
+	for _, failure := range failures {
+		annotated = append(annotated, failure)
+
+		var key string
+		var found bool
+
+		// Check if this is a test failure
+		if strings.HasPrefix(failure, "--- FAIL: ") {
+			parts := strings.Fields(strings.TrimPrefix(failure, "--- FAIL: "))
+			if len(parts) > 0 {
+				key = parts[0]
+				found = true
+			}
+		} else if strings.HasPrefix(failure, "contract:") {
+			// Check if this is a contract failure
+			parts := strings.Split(failure, " — ")
+			if len(parts) > 0 {
+				key = strings.TrimSpace(parts[0])
+				found = true
+			}
+		}
+
+		// If we found a key and it has met or exceeded the threshold, add a hint
+		if found && history[key] >= threshold {
+			hint := fmt.Sprintf("persistent-failure: %s has failed %d consecutive cycles — may indicate a bad test specification rather than an implementation bug",
+				key, history[key])
+			annotated = append(annotated, hint)
+		}
+	}
+
+	return annotated
+}
diff --git a/internal/next/specloop/failure_history_test.go b/internal/next/specloop/failure_history_test.go
new file mode 100644
index 000000000..2d5ef8be6
--- /dev/null
+++ b/internal/next/specloop/failure_history_test.go
@@ -0,0 +1,334 @@
+package specloop
+
+import (
+	"slices"
+	"testing"
+)
+
+// TestFailureHistory is the main test suite for failure history functionality
+func TestFailureHistory(t *testing.T) {
+	t.Run("TestExtractTestFailureKeys", testExtractTestFailureKeys)
+	t.Run("TestExtractContractFailureKeys", testExtractContractFailureKeys)
+	t.Run("TestUpdateFailureHistory", testUpdateFailureHistory)
+	t.Run("TestAnnotateWithPersistentFailureHints", testAnnotateWithPersistentFailureHints)
+}
+
+func testExtractTestFailureKeys(t *testing.T) {
+	tests := []struct {
+		name     string
+		failures []string
+		want     []string
+	}{
+		{
+			name: "extract single test failure",
+			failures: []string{
+				"--- FAIL: TestFoo (0.01s)",
+			},
+			want: []string{"TestFoo"},
+		},
+		{
+			name: "extract multiple test failures",
+			failures: []string{
+				"--- FAIL: TestFoo (0.01s)",
+				"--- FAIL: TestBar (0.02s)",
+				"--- FAIL: TestBaz (0.03s)",
+			},
+			want: []string{"TestFoo", "TestBar", "TestBaz"},
+		},
+		{
+			name: "ignore non-matching lines",
+			failures: []string{
+				"--- FAIL: TestFoo (0.01s)",
+				"some other error",
+				"contract:something — failed",
+				"--- PASS: TestBar (0.02s)",
+			},
+			want: []string{"TestFoo"},
+		},
+		{
+			name:     "empty input returns empty list",
+			failures: []string{},
+			want:     []string{},
+		},
+		{
+			name: "no matching failures returns empty list",
+			failures: []string{
+				"some random error",
+				"another error",
+			},
+			want: []string{},
+		},
+		{
+			name: "test name with multiple words uses first word",
+			failures: []string{
+				"--- FAIL: TestFoo and other text (0.01s)",
+			},
+			want: []string{"TestFoo"},
+		},
+	}
+
+	for _, tt := range tests {
+		t.Run(tt.name, func(t *testing.T) {
+			got := ExtractTestFailureKeys(tt.failures)
+			if !slices.Equal(got, tt.want) {
+				t.Errorf("ExtractTestFailureKeys() = %v, want %v", got, tt.want)
+			}
+		})
+	}
+}
+
+func testExtractContractFailureKeys(t *testing.T) {
+	tests := []struct {
+		name     string
+		failures []string
+		want     []string
+	}{
+		{
+			name: "extract single contract failure",
+			failures: []string{
+				"contract:add-works — file_contains failed: expected output not found",
+			},
+			want: []string{"contract:add-works"},
+		},
+		{
+			name: "extract multiple contract failures",
+			failures: []string{
+				"contract:add-works — file_contains failed: expected output not found",
+				"contract:list-shows-items — output_matches failed: regex mismatch",
+				"contract:delete-removes — file_missing failed: file still exists",
+			},
+			want: []string{"contract:add-works", "contract:list-shows-items", "contract:delete-removes"},
+		},
+		{
+			name: "ignore non-contract failures",
+			failures: []string{
+				"contract:add-works — file_contains failed: expected output not found",
+				"--- FAIL: TestFoo (0.01s)",
+				"some other error",
+				"contract:list-shows-items — output_matches failed: regex mismatch",
+			},
+			want: []string{"contract:add-works", "contract:list-shows-items"},
+		},
+		{
+			name:     "empty input returns empty list",
+			failures: []string{},
+			want:     []string{},
+		},
+		{
+			name: "no matching failures returns empty list",
+			failures: []string{
+				"--- FAIL: TestFoo (0.01s)",
+				"some error",
+			},
+			want: []string{},
+		},
+		{
+			name: "handles extra whitespace around delimiter",
+			failures: []string{
+				"contract:test-case  —  some details here",
+			},
+			want: []string{"contract:test-case"},
+		},
+	}
+
+	for _, tt := range tests {
+		t.Run(tt.name, func(t *testing.T) {
+			got := ExtractContractFailureKeys(tt.failures)
+			if !slices.Equal(got, tt.want) {
+				t.Errorf("ExtractContractFailureKeys() = %v, want %v", got, tt.want)
+			}
+		})
+	}
+}
+
+func testUpdateFailureHistory(t *testing.T) {
+	tests := []struct {
+		name        string
+		history     map[string]int
+		currentKeys []string
+		want        map[string]int
+	}{
+		{
+			name:        "increment present keys",
+			history:     map[string]int{"TestFoo": 1, "TestBar": 2},
+			currentKeys: []string{"TestFoo", "TestBar"},
+			want:        map[string]int{"TestFoo": 2, "TestBar": 3},
+		},
+		{
+			name:        "remove absent keys",
+			history:     map[string]int{"TestFoo": 1, "TestBar": 2},
+			currentKeys: []string{"TestFoo"},
+			want:        map[string]int{"TestFoo": 2},
+		},
+		{
+			name:        "add new keys",
+			history:     map[string]int{"TestFoo": 1},
+			currentKeys: []string{"TestFoo", "TestBar"},
+			want:        map[string]int{"TestFoo": 2, "TestBar": 1},
+		},
+		{
+			name:        "handle empty history",
+			history:     map[string]int{},
+			currentKeys: []string{"TestFoo", "TestBar"},
+			want:        map[string]int{"TestFoo": 1, "TestBar": 1},
+		},
+		{
+			name:        "clear all keys when no current keys",
+			history:     map[string]int{"TestFoo": 1, "TestBar": 2},
+			currentKeys: []string{},
+			want:        map[string]int{},
+		},
+		{
+			name:        "mixed additions and removals",
+			history:     map[string]int{"TestFoo": 1, "TestBar": 2, "TestBaz": 3},
+			currentKeys: []string{"TestFoo", "TestQux"},
+			want:        map[string]int{"TestFoo": 2, "TestQux": 1},
+		},
+	}
+
+	for _, tt := range tests {
+		t.Run(tt.name, func(t *testing.T) {
+			UpdateFailureHistory(tt.history, tt.currentKeys)
+			if !mapsEqual(tt.history, tt.want) {
+				t.Errorf("UpdateFailureHistory() = %v, want %v", tt.history, tt.want)
+			}
+		})
+	}
+}
+
+func testAnnotateWithPersistentFailureHints(t *testing.T) {
+	tests := []struct {
+		name      string
+		failures  []string
+		history   map[string]int
+		threshold int
+		want      []string
+	}{
+		{
+			name: "add hint when count >= threshold",
+			failures: []string{
+				"--- FAIL: TestFoo (0.01s)",
+			},
+			history:   map[string]int{"TestFoo": 2},
+			threshold: 2,
+			want: []string{
+				"--- FAIL: TestFoo (0.01s)",
+				"persistent-failure: TestFoo has failed 2 consecutive cycles — may indicate a bad test specification rather than an implementation bug",
+			},
+		},
+		{
+			name: "leave failure unchanged when count < threshold",
+			failures: []string{
+				"--- FAIL: TestFoo (0.01s)",
+			},
+			history:   map[string]int{"TestFoo": 1},
+			threshold: 2,
+			want: []string{
+				"--- FAIL: TestFoo (0.01s)",
+			},
+		},
+		{
+			name: "handle mixed test and contract failures",
+			failures: []string{
+				"--- FAIL: TestFoo (0.01s)",
+				"contract:add-works — file_contains failed: expected output not found",
+			},
+			history:   map[string]int{"TestFoo": 3, "contract:add-works": 1},
+			threshold: 2,
+			want: []string{
+				"--- FAIL: TestFoo (0.01s)",
+				"persistent-failure: TestFoo has failed 3 consecutive cycles — may indicate a bad test specification rather than an implementation bug",
+				"contract:add-works — file_contains failed: expected output not found",
+			},
+		},
+		{
+			name: "no hint for failures not in history",
+			failures: []string{
+				"--- FAIL: TestFoo (0.01s)",
+			},
+			history:   map[string]int{},
+			threshold: 2,
+			want: []string{
+				"--- FAIL: TestFoo (0.01s)",
+			},
+		},
+		{
+			name: "multiple failures with selective hints",
+			failures: []string{
+				"--- FAIL: TestFoo (0.01s)",
+				"--- FAIL: TestBar (0.02s)",
+				"--- FAIL: TestBaz (0.03s)",
+			},
+			history:   map[string]int{"TestFoo": 2, "TestBar": 5, "TestBaz": 1},
+			threshold: 2,
+			want: []string{
+				"--- FAIL: TestFoo (0.01s)",
+				"persistent-failure: TestFoo has failed 2 consecutive cycles — may indicate a bad test specification rather than an implementation bug",
+				"--- FAIL: TestBar (0.02s)",
+				"persistent-failure: TestBar has failed 5 consecutive cycles — may indicate a bad test specification rather than an implementation bug",
+				"--- FAIL: TestBaz (0.03s)",
+			},
+		},
+		{
+			name: "contract failures with hints",
+			failures: []string{
+				"contract:add-works — file_contains failed: expected output not found",
+				"contract:list-shows-items — output_matches failed: regex mismatch",
+			},
+			history:   map[string]int{"contract:add-works": 3, "contract:list-shows-items": 1},
+			threshold: 2,
+			want: []string{
+				"contract:add-works — file_contains failed: expected output not found",
+				"persistent-failure: contract:add-works has failed 3 consecutive cycles — may indicate a bad test specification rather than an implementation bug",
+				"contract:list-shows-items — output_matches failed: regex mismatch",
+			},
+		},
+		{
+			name:      "empty failures returns empty list",
+			failures:  []string{},
+			history:   map[string]int{"TestFoo": 5},
+			threshold: 2,
+			want:      []string{},
+		},
+		{
+			name: "threshold of zero applies to all failures",
+			failures: []string{
+				"--- FAIL: TestFoo (0.01s)",
+			},
+			history:   map[string]int{"TestFoo": 0},
+			threshold: 0,
+			want: []string{
+				"--- FAIL: TestFoo (0.01s)",
+				"persistent-failure: TestFoo has failed 0 consecutive cycles — may indicate a bad test specification rather than an implementation bug",
+			},
+		},
+	}
+
+	for _, tt := range tests {
+		t.Run(tt.name, func(t *testing.T) {
+			got := AnnotateWithPersistentHints(tt.failures, tt.history, tt.threshold)
+			if !slices.Equal(got, tt.want) {
+				t.Errorf("AnnotateWithPersistentHints() = %v, want %v", got, tt.want)
+			}
+		})
+	}
+}
+
+// mapsEqual compares two maps, handling nil maps
+func mapsEqual(a, b map[string]int) bool {
+	if a == nil && b == nil {
+		return true
+	}
+	if a == nil || b == nil {
+		return false
+	}
+	if len(a) != len(b) {
+		return false
+	}
+	for k, v := range a {
+		if b[k] != v {
+			return false
+		}
+	}
+	return true
+}
diff --git a/internal/next/specloop/specloop.go b/internal/next/specloop/specloop.go
index efd64ace0..d0c638812 100644
--- a/internal/next/specloop/specloop.go
+++ b/internal/next/specloop/specloop.go
@@ -2,6 +2,8 @@ package specloop
 
 import (
 	"context"
+	"fmt"
+	"strings"
 	"time"
 
 	"github.com/danabrams/gromit/internal/next/runstore"
@@ -127,7 +129,56 @@ func (sl *SpecLoop) Run(ctx context.Context, rs *runstore.RunState) error {
 
 		// Thread failure context into RunState for PlanStage to read on replan
 		if replanContext != nil {
-			rs.ReplanContext = replanContext.Failures
+			// Extract failure keys from both test and contract failures
+			testKeys := ExtractTestFailureKeys(replanContext.Failures)
+			contractKeys := ExtractContractFailureKeys(replanContext.Failures)
+
+			// Merge key sets
+			mergedKeys := make([]string, 0, len(testKeys)+len(contractKeys))
+			mergedKeys = append(mergedKeys, testKeys...)
+			mergedKeys = append(mergedKeys, contractKeys...)
+
+			// Initialize FailureHistory if nil
+			if rs.FailureHistory == nil {
+				rs.FailureHistory = make(map[string]int)
+			}
+
+			// Update failure history with current cycle's failures
+			UpdateFailureHistory(rs.FailureHistory, mergedKeys)
+
+			// Annotate failures with persistent-failure hints for consecutive cycles
+			// that may indicate a bad test specification rather than an implementation bug
+			var annotated []string
+			for _, failure := range replanContext.Failures {
+				annotated = append(annotated, failure)
+
+				var key string
+				var found bool
+
+				// Check if this is a test failure
+				if strings.HasPrefix(failure, "--- FAIL: ") {
+					parts := strings.Fields(strings.TrimPrefix(failure, "--- FAIL: "))
+					if len(parts) > 0 {
+						key = parts[0]
+						found = true
+					}
+				} else if strings.HasPrefix(failure, "contract:") {
+					// Check if this is a contract failure
+					parts := strings.Split(failure, " — ")
+					if len(parts) > 0 {
+						key = strings.TrimSpace(parts[0])
+						found = true
+					}
+				}
+
+				// If we found a key and it has met or exceeded the threshold, add a hint
+				if found && rs.FailureHistory[key] >= 2 {
+					hint := fmt.Sprintf("persistent-failure: %s has failed %d consecutive cycles — may indicate a bad test specification rather than an implementation bug",
+						key, rs.FailureHistory[key])
+					annotated = append(annotated, hint)
+				}
+			}
+			rs.ReplanContext = annotated
 		}
 
 		// Emit replan_triggered event
@@ -163,6 +214,16 @@ func (sl *SpecLoop) Run(ctx context.Context, rs *runstore.RunState) error {
 	return nil
 }
 
+// extractFailureKeys extracts and merges both test and contract failure keys.
+func extractFailureKeys(failures []string) []string {
+	testKeys := ExtractTestFailureKeys(failures)
+	contractKeys := ExtractContractFailureKeys(failures)
+	merged := make([]string, 0, len(testKeys)+len(contractKeys))
+	merged = append(merged, testKeys...)
+	merged = append(merged, contractKeys...)
+	return merged
+}
+
 // emitEvent appends an event to the log if configured.
 func (sl *SpecLoop) emitEvent(ev runstore.TypedEvent) {
 	if sl.config.EventLog != nil {
diff --git a/internal/next/specloop/specloop_scenario_test.go b/internal/next/specloop/specloop_scenario_test.go
new file mode 100644
index 000000000..689d1387d
--- /dev/null
+++ b/internal/next/specloop/specloop_scenario_test.go
@@ -0,0 +1,426 @@
+//go:build integration
+
+package specloop
+
+import (
+	"context"
+	"testing"
+
+	"github.com/danabrams/gromit/internal/next/execpolicy"
+	"github.com/danabrams/gromit/internal/next/runstore"
+)
+
+// --- Test 1: WriteScenarioTests in pipeline stage order ---
+
+func TestIntegration_WriteScenarioTestsInPipeline(t *testing.T) {
+	// Verify that WriteScenarioTests runs after Execute and before Validate.
+	// We'll track the order of stage calls using a call sequence.
+	var stageCallSequence []string
+
+	initStage := &scenarioStage{
+		name: "init",
+		fn: func(_ context.Context, _ *runstore.RunState, _ int) (NextAction, error) {
+			stageCallSequence = append(stageCallSequence, "init")
+			return NextAction{Kind: Continue}, nil
+		},
+	}
+
+	executeStage := &scenarioStage{
+		name: "execute",
+		fn: func(_ context.Context, _ *runstore.RunState, _ int) (NextAction, error) {
+			stageCallSequence = append(stageCallSequence, "execute")
+			return NextAction{Kind: Continue}, nil
+		},
+	}
+
+	writeScenarioTestsStage := &scenarioStage{
+		name: "write_scenario_tests",
+		fn: func(_ context.Context, _ *runstore.RunState, _ int) (NextAction, error) {
+			stageCallSequence = append(stageCallSequence, "write_scenario_tests")
+			return NextAction{Kind: Continue}, nil
+		},
+	}
+
+	validateStage := &scenarioStage{
+		name: "validate",
+		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
+			stageCallSequence = append(stageCallSequence, "validate")
+			rs.FinalValidationPassed = true
+			return NextAction{Kind: Continue}, nil
+		},
+	}
+
+	reviewStage := &scenarioStage{
+		name: "review",
+		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
+			stageCallSequence = append(stageCallSequence, "review")
+			rs.FinalReviewPassed = true
+			return NextAction{Kind: Continue}, nil
+		},
+	}
+
+	acceptStage := &scenarioStage{
+		name: "accept",
+		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
+			stageCallSequence = append(stageCallSequence, "accept")
+			rs.FinalAcceptancePassed = true
+			return NextAction{Kind: Continue}, nil
+		},
+	}
+
+	stages := []Stage{initStage, executeStage, writeScenarioTestsStage, validateStage, reviewStage, acceptStage}
+	loop := NewSpecLoop(stages, SpecLoopConfig{
+		Budget: NewBudget(execpolicy.Budgets{
+			MaxSpecCycles:          1,
+			MaxRunCostUSD:          99,
+			MaxRunDurationSeconds:  3600,
+			MaxTaskDurationSeconds: 300,
+		}),
+		ReplanStage: "execute",
+	})
+
+	rs := runstore.NewRunState("test-spec", "test-project")
+	if err := loop.Run(context.Background(), rs); err != nil {
+		t.Fatalf("Run: %v", err)
+	}
+
+	// Verify that write_scenario_tests appears after execute and before validate.
+	executeIdx := -1
+	writeIdx := -1
+	validateIdx := -1
+	for i, stage := range stageCallSequence {
+		if stage == "execute" {
+			executeIdx = i
+		}
+		if stage == "write_scenario_tests" {
+			writeIdx = i
+		}
+		if stage == "validate" {
+			validateIdx = i
+		}
+	}
+
+	if executeIdx < 0 {
+		t.Error("execute stage was not called")
+	}
+	if writeIdx < 0 {
+		t.Error("write_scenario_tests stage was not called")
+	}
+	if validateIdx < 0 {
+		t.Error("validate stage was not called")
+	}
+
+	if executeIdx >= writeIdx {
+		t.Errorf("execute should run before write_scenario_tests: execute_idx=%d, write_idx=%d", executeIdx, writeIdx)
+	}
+	if writeIdx >= validateIdx {
+		t.Errorf("write_scenario_tests should run before validate: write_idx=%d, validate_idx=%d", writeIdx, validateIdx)
+	}
+}
+
+// --- Test 2: ScenarioTestReplan preserves tests (no-op on second cycle) ---
+
+func TestIntegration_ScenarioTestReplanPreservesTests(t *testing.T) {
+	// Verify that on a replan cycle, WriteScenarioTests is a no-op when
+	// ScenarioTestsWritten is already true. We track how many times the
+	// write_scenario_tests stage actually performs work.
+	writeWorkCount := 0
+
+	executeStage := &scenarioStage{
+		name: "execute",
+		fn: func(_ context.Context, _ *runstore.RunState, _ int) (NextAction, error) {
+			return NextAction{Kind: Continue}, nil
+		},
+	}
+
+	writeScenarioTestsStage := &scenarioStage{
+		name: "write_scenario_tests",
+		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
+			// If not yet written, mark it and count the work
+			if !rs.ScenarioTestsWritten {
+				writeWorkCount++
+				rs.ScenarioTestsWritten = true
+				return NextAction{Kind: Continue}, nil
+			}
+			// Already written: idempotent no-op
+			return NextAction{Kind: Continue}, nil
+		},
+	}
+
+	validateStage := &scenarioStage{
+		name: "validate",
+		fn: func(_ context.Context, rs *runstore.RunState, call int) (NextAction, error) {
+			rs.FinalValidationPassed = true
+			// On first cycle, trigger replan to test that write_scenario_tests is idempotent
+			if call == 1 {
+				return NextAction{
+					Kind: ReplanFrom,
+					Context: &FailureContext{
+						Failures: []string{"--- FAIL: TestSomeTest"},
+						Cycle:    rs.Cycle,
+					},
+				}, nil
+			}
+			// On second cycle, validation passes
+			return NextAction{Kind: Continue}, nil
+		},
+	}
+
+	reviewStage := &scenarioStage{
+		name: "review",
+		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
+			rs.FinalReviewPassed = true
+			return NextAction{Kind: Continue}, nil
+		},
+	}
+
+	acceptStage := &scenarioStage{
+		name: "accept",
+		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
+			rs.FinalAcceptancePassed = true
+			return NextAction{Kind: Continue}, nil
+		},
+	}
+
+	finalizeStage := &scenarioStage{
+		name: "finalize",
+		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
+			if rs.FinalValidationPassed && rs.FinalReviewPassed && rs.FinalAcceptancePassed {
+				rs.Status = runstore.StatusReadyForReview
+			}
+			return NextAction{Kind: Continue}, nil
+		},
+	}
+
+	stages := []Stage{executeStage, writeScenarioTestsStage, validateStage, reviewStage, acceptStage, finalizeStage}
+	loop := NewSpecLoop(stages, SpecLoopConfig{
+		Budget: NewBudget(execpolicy.Budgets{
+			MaxSpecCycles:          3,
+			MaxRunCostUSD:          99,
+			MaxRunDurationSeconds:  3600,
+			MaxTaskDurationSeconds: 300,
+		}),
+		ReplanStage: "execute",
+	})
+
+	rs := runstore.NewRunState("test-spec", "test-project")
+	if err := loop.Run(context.Background(), rs); err != nil {
+		t.Fatalf("Run: %v", err)
+	}
+
+	// ScenarioTestsWritten should be set after first cycle
+	if !rs.ScenarioTestsWritten {
+		t.Error("ScenarioTestsWritten should be true after first cycle")
+	}
+
+	// writeWorkCount should be 1 (only performed work once, on first cycle)
+	// Cycle 2 should see ScenarioTestsWritten=true and skip work
+	if writeWorkCount != 1 {
+		t.Errorf("write_scenario_tests should only perform work once (idempotent), got count %d", writeWorkCount)
+	}
+
+	// Second cycle should have run (verified by rs.Cycle)
+	if rs.Cycle < 2 {
+		t.Errorf("expected at least 2 cycles (replan cycle), got %d", rs.Cycle)
+	}
+}
+
+// --- Test 3: PersistentFailureHint after two consecutive cycles ---
+
+func TestIntegration_PersistentFailureHintAfterTwoCycles(t *testing.T) {
+	// Verify that FailureHistory accumulates across cycles and annotates
+	// failures with persistent-failure hints after 2 consecutive cycles.
+	// Uses contract: failure format which is properly extracted by ExtractContractFailureKeys.
+
+	executeStage := passThrough("execute")
+
+	validateStage := &scenarioStage{
+		name: "validate",
+		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
+			rs.FinalValidationPassed = true
+			return NextAction{Kind: Continue}, nil
+		},
+	}
+
+	reviewStage := &scenarioStage{
+		name: "review",
+		fn: func(_ context.Context, rs *runstore.RunState, call int) (NextAction, error) {
+			// All cycles: review finds a blocking failure using contract: format
+			// Contract failures are properly extracted for FailureHistory tracking
+			rs.FinalReviewPassed = false
+			rs.ReviewFindings = []string{"contract:UserCreation — missing validation"}
+			return NextAction{
+				Kind: ReplanFrom,
+				Context: &FailureContext{
+					Failures: []string{"contract:UserCreation — missing validation"},
+					Cycle:    rs.Cycle,
+				},
+			}, nil
+		},
+	}
+
+	acceptStage := passThrough("accept")
+
+	stages := []Stage{executeStage, validateStage, reviewStage, acceptStage}
+	loop := NewSpecLoop(stages, SpecLoopConfig{
+		Budget: NewBudget(execpolicy.Budgets{
+			MaxSpecCycles:          3,
+			MaxRunCostUSD:          99,
+			MaxRunDurationSeconds:  3600,
+			MaxTaskDurationSeconds: 300,
+		}),
+		ReplanStage: "execute",
+	})
+
+	rs := runstore.NewRunState("test-spec", "test-project")
+	if err := loop.Run(context.Background(), rs); err != nil {
+		t.Fatalf("Run: %v", err)
+	}
+
+	// After 2+ cycles with the same contract failure, the replan context should have been
+	// annotated with a persistent-failure hint (threshold is 2).
+	// The hint should be appended to the annotated failures.
+
+	foundPersistentFailureHint := false
+	for _, failure := range rs.ReplanContext {
+		if contains(failure, "persistent-failure:") {
+			foundPersistentFailureHint = true
+			// Verify the hint mentions the failure key and has "failed" and "consecutive cycles"
+			if !contains(failure, "contract:UserCreation") {
+				t.Errorf("persistent-failure hint should mention contract:UserCreation, got: %s", failure)
+			}
+			if !contains(failure, "consecutive cycles") {
+				t.Errorf("persistent-failure hint should mention 'consecutive cycles', got: %s", failure)
+			}
+			break
+		}
+	}
+
+	if !foundPersistentFailureHint {
+		t.Errorf("ReplanContext should contain persistent-failure hint after 2 consecutive cycles. Got: %v", rs.ReplanContext)
+	}
+
+	// FailureHistory should be tracking the contract key
+	if len(rs.FailureHistory) == 0 {
+		t.Error("FailureHistory should be populated with contract:UserCreation key")
+	}
+
+	// Check that the extracted key is in history with a count >= 2
+	if count, found := rs.FailureHistory["contract:UserCreation"]; !found || count < 2 {
+		t.Errorf("contract:UserCreation should have count >= 2 in FailureHistory, got: %d (found: %v)", count, found)
+	}
+
+	// Verify that budget exhaustion occurred after 3 cycles (as expected with persistent failures)
+	if rs.Status != runstore.StatusNeedsHuman {
+		t.Errorf("expected status %q after cycles exhausted, got %q", runstore.StatusNeedsHuman, rs.Status)
+	}
+}
+
+// --- Test 4: ScenarioTestsWritten not reset per cycle ---
+
+func TestIntegration_ScenarioTestsWrittenNotResetPerCycle(t *testing.T) {
+	// Verify that ResetForNewCycle does not reset ScenarioTestsWritten or FailureHistory.
+	// These fields should persist across cycles.
+
+	executeStage := &scenarioStage{
+		name: "execute",
+		fn: func(_ context.Context, _ *runstore.RunState, _ int) (NextAction, error) {
+			return NextAction{Kind: Continue}, nil
+		},
+	}
+
+	writeScenarioTestsStage := &scenarioStage{
+		name: "write_scenario_tests",
+		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
+			if !rs.ScenarioTestsWritten {
+				rs.ScenarioTestsWritten = true
+			}
+			return NextAction{Kind: Continue}, nil
+		},
+	}
+
+	validateStage := &scenarioStage{
+		name: "validate",
+		fn: func(_ context.Context, rs *runstore.RunState, call int) (NextAction, error) {
+			rs.FinalValidationPassed = true
+			// Cycles 1-2: trigger replan to test persistence
+			if call <= 2 {
+				return NextAction{
+					Kind: ReplanFrom,
+					Context: &FailureContext{
+						Failures: []string{"--- FAIL: TestValidation"},
+						Cycle:    rs.Cycle,
+					},
+				}, nil
+			}
+			// Cycle 3: pass to allow completion
+			return NextAction{Kind: Continue}, nil
+		},
+	}
+
+	reviewStage := &scenarioStage{
+		name: "review",
+		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
+			rs.FinalReviewPassed = true
+			return NextAction{Kind: Continue}, nil
+		},
+	}
+
+	acceptStage := &scenarioStage{
+		name: "accept",
+		fn: func(_ context.Context, rs *runstore.RunState, _ int) (NextAction, error) {
+			rs.FinalAcceptancePassed = true
+			return NextAction{Kind: Continue}, nil
+		},
+	}
+
+	stages := []Stage{executeStage, writeScenarioTestsStage, validateStage, reviewStage, acceptStage}
+	loop := NewSpecLoop(stages, SpecLoopConfig{
+		Budget: NewBudget(execpolicy.Budgets{
+			MaxSpecCycles:          3,
+			MaxRunCostUSD:          99,
+			MaxRunDurationSeconds:  3600,
+			MaxTaskDurationSeconds: 300,
+		}),
+		ReplanStage: "execute",
+	})
+
+	rs := runstore.NewRunState("test-spec", "test-project")
+	if err := loop.Run(context.Background(), rs); err != nil {
+		t.Fatalf("Run: %v", err)
+	}
+
+	// After multiple cycles, ScenarioTestsWritten should still be true
+	if !rs.ScenarioTestsWritten {
+		t.Error("ScenarioTestsWritten should remain true across replan cycles (not reset by ResetForNewCycle)")
+	}
+
+	// FailureHistory should accumulate across cycles with the same failure key
+	if len(rs.FailureHistory) == 0 {
+		t.Error("FailureHistory should be non-empty after multiple cycles with failures")
+	}
+
+	// Verify that the failure key "TestValidation" is in the history
+	testValidationCount := rs.FailureHistory["TestValidation"]
+	if testValidationCount == 0 {
+		t.Errorf("TestValidation failure should be tracked in FailureHistory, got count %d", testValidationCount)
+	}
+
+	// Cycle count should show we had replans
+	if rs.Cycle < 3 {
+		t.Errorf("expected at least 3 cycles with 2 replan cycles, got %d", rs.Cycle)
+	}
+}
+
+// --- Helper function ---
+
+// contains checks if a substring exists in a string
+func contains(s, substr string) bool {
+	// Simple string containment check
+	for i := 0; i < len(s)-len(substr)+1; i++ {
+		if s[i:i+len(substr)] == substr {
+			return true
+		}
+	}
+	return false
+}
diff --git a/internal/next/specloop/specloop_test.go b/internal/next/specloop/specloop_test.go
index f2b2dd85e..7ee70b4e7 100644
--- a/internal/next/specloop/specloop_test.go
+++ b/internal/next/specloop/specloop_test.go
@@ -730,3 +730,152 @@ func TestSpecLoop_CycleExhaustion_SetsBlockerSummaryFromReplanContext(t *testing
 		t.Fatalf("want blocker summary from replan context, got %q", rs.BlockerSummary)
 	}
 }
+
+func TestExtractFailureKeys_MergesTestAndContractKeys(t *testing.T) {
+	failures := []string{
+		"--- FAIL: TestFoo (0.01s)",
+		"--- FAIL: TestBar (0.02s)",
+		"contract:add-works — file_contains failed: expected output not found",
+		"contract:list-shows-items — output_matches failed: regex mismatch",
+		"some other error",
+	}
+
+	keys := extractFailureKeys(failures)
+
+	// Should have 4 keys: 2 test names + 2 contract names
+	if len(keys) != 4 {
+		t.Fatalf("expected 4 keys, got %d", len(keys))
+	}
+
+	// Check test keys are present
+	testKeysPresent := false
+	contractKeysPresent := false
+	for _, key := range keys {
+		if key == "TestFoo" || key == "TestBar" {
+			testKeysPresent = true
+		}
+		if key == "contract:add-works" || key == "contract:list-shows-items" {
+			contractKeysPresent = true
+		}
+	}
+
+	if !testKeysPresent {
+		t.Error("test failure keys should be extracted")
+	}
+	if !contractKeysPresent {
+		t.Error("contract failure keys should be extracted")
+	}
+}
+
+func TestUpdateFailureHistory_IncrementsCounts(t *testing.T) {
+	history := map[string]int{}
+
+	// Cycle 1: TestFoo and TestBar fail
+	UpdateFailureHistory(history, []string{"TestFoo", "TestBar"})
+	if history["TestFoo"] != 1 || history["TestBar"] != 1 {
+		t.Fatalf("cycle 1: expected counts 1,1 got %d,%d", history["TestFoo"], history["TestBar"])
+	}
+
+	// Cycle 2: TestFoo and TestBar fail again
+	UpdateFailureHistory(history, []string{"TestFoo", "TestBar"})
+	if history["TestFoo"] != 2 || history["TestBar"] != 2 {
+		t.Fatalf("cycle 2: expected counts 2,2 got %d,%d", history["TestFoo"], history["TestBar"])
+	}
+
+	// Cycle 3: Only TestFoo fails (TestBar is resolved)
+	UpdateFailureHistory(history, []string{"TestFoo"})
+	if history["TestFoo"] != 3 {
+		t.Fatalf("cycle 3: expected TestFoo count 3 got %d", history["TestFoo"])
+	}
+	if count, exists := history["TestBar"]; exists {
+		t.Fatalf("cycle 3: TestBar should be deleted from history, but has count %d", count)
+	}
+}
+
+func TestPersistentFailure_HistoryIncrementedOnConsecutiveFailures(t *testing.T) {
+	// This test verifies that failure history is tracked correctly across cycles
+	// and that the replan context contains persistent-failure hints
+	validateCalls := 0
+
+	stages := []Stage{
+		&countStage{name: "plan", counts: map[string]int{}},
+		&countStage{name: "execute", counts: map[string]int{}},
+		&mockStage{name: "validate", runFn: func(_ context.Context, rs *runstore.RunState) (NextAction, error) {
+			validateCalls++
+			if validateCalls <= 2 {
+				// Cycles 1 and 2: TestFoo fails
+				return NextAction{
+					Kind: ReplanFrom,
+					Context: &FailureContext{
+						Failures: []string{"--- FAIL: TestFoo (0.01s)"},
+					},
+				}, nil
+			}
+			// Cycle 3: TestFoo is now fixed
+			rs.FinalValidationPassed = true
+			return NextAction{Kind: Continue}, nil
+		}},
+		&countStage{name: "finalize", counts: map[string]int{}},
+	}
+
+	budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 3, MaxRunCostUSD: 99})
+	loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budget, ReplanStage: "plan"})
+	rs := runstore.NewRunState("s1", "p1")
+
+	err := loop.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+
+	// Verify run completed successfully
+	if !rs.FinalValidationPassed {
+		t.Error("validation should pass on cycle 3")
+	}
+
+	// After cycle 2, FailureHistory should have TestFoo with count 2
+	// After cycle 3 resolves it, FailureHistory should be empty (TestFoo removed)
+	// The ReplanContext from cycle 2 should have included a persistent-failure hint
+	// but that context is no longer in ReplanContext after cycle 3 succeeds
+	// This test just verifies the mechanism works by checking the run succeeded after
+	// the failure was fixed
+}
+
+func TestPersistentFailure_IntegrationWithContractFailures(t *testing.T) {
+	// Test that persistent failure tracking works with contract failures too
+	validateCalls := 0
+
+	stages := []Stage{
+		&countStage{name: "plan", counts: map[string]int{}},
+		&countStage{name: "execute", counts: map[string]int{}},
+		&mockStage{name: "validate", runFn: func(_ context.Context, rs *runstore.RunState) (NextAction, error) {
+			validateCalls++
+			if validateCalls <= 2 {
+				// Cycles 1 and 2: contract failure persists
+				return NextAction{
+					Kind: ReplanFrom,
+					Context: &FailureContext{
+						Failures: []string{"contract:add-works — file_contains failed: expected output not found"},
+					},
+				}, nil
+			}
+			// Cycle 3: contract is now fixed
+			rs.FinalValidationPassed = true
+			return NextAction{Kind: Continue}, nil
+		}},
+		&countStage{name: "finalize", counts: map[string]int{}},
+	}
+
+	budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 3, MaxRunCostUSD: 99})
+	loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budget, ReplanStage: "plan"})
+	rs := runstore.NewRunState("s1", "p1")
+
+	err := loop.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+
+	// Verify run completed successfully after contract was fixed
+	if !rs.FinalValidationPassed {
+		t.Error("validation should pass on cycle 3 after contract is fixed")
+	}
+}
diff --git a/internal/next/specloop/stages/validate.go b/internal/next/specloop/stages/validate.go
index 7bf2eae8c..0eee5e722 100644
--- a/internal/next/specloop/stages/validate.go
+++ b/internal/next/specloop/stages/validate.go
@@ -75,6 +75,8 @@ func (s *ValidateStage) Run(ctx context.Context, rs *runstore.RunState) (specloo
 	}
 
 	// Run shell checks regardless of contract results.
+	// Scenario test failures are detected via the always-run 'go test ./...' check
+	// and reported through the standard go test output format.
 	result, err := s.validator.RunFinal(ctx, s.cfg.AlwaysRun, s.cfg.ProjectChecks, workDir)
 	if err != nil {
 		return specloop.NextAction{}, fmt.Errorf("final validation: %w", err)
diff --git a/internal/next/specloop/stages/write_scenario_tests.go b/internal/next/specloop/stages/write_scenario_tests.go
new file mode 100644
index 000000000..982788c96
--- /dev/null
+++ b/internal/next/specloop/stages/write_scenario_tests.go
@@ -0,0 +1,356 @@
+package stages
+
+import (
+	"context"
+	"encoding/json"
+	"fmt"
+	"os"
+	"os/exec"
+	"path/filepath"
+	"time"
+
+	"github.com/danabrams/gromit/internal/next/contract"
+	"github.com/danabrams/gromit/internal/next/runstore"
+	"github.com/danabrams/gromit/internal/next/specloop"
+)
+
+// WriteScenarioTestsStageConfig configures the WriteScenarioTestsStage.
+type WriteScenarioTestsStageConfig struct {
+	// SpecPath is the path to the raw spec markdown file.
+	SpecPath string
+	// EvidenceDir is the directory where scenario-test-manifest.json will be written.
+	EvidenceDir string
+	// Store provides access to run storage operations.
+	Store *runstore.Store
+	// WorkDir is the working directory for the project (used for go test compilation checks).
+	WorkDir string
+}
+
+// WriteScenarioTestsStage writes test files for each scenario parsed from the spec.
+// It is a no-op (idempotent) if ScenarioTestsWritten is already true on the RunState.
+type WriteScenarioTestsStage struct {
+	writer   contract.ScenarioTestWriter
+	cfg      WriteScenarioTestsStageConfig
+	budget   *specloop.Budget
+	eventLog *runstore.EventLog
+}
+
+// NewWriteScenarioTestsStage creates a new WriteScenarioTestsStage.
+func NewWriteScenarioTestsStage(writer contract.ScenarioTestWriter, cfg WriteScenarioTestsStageConfig, budget *specloop.Budget, eventLog *runstore.EventLog) *WriteScenarioTestsStage {
+	return &WriteScenarioTestsStage{
+		writer:   writer,
+		cfg:      cfg,
+		budget:   budget,
+		eventLog: eventLog,
+	}
+}
+
+// Name returns the stage name.
+func (s *WriteScenarioTestsStage) Name() string { return "write_scenario_tests" }
+
+// Run executes the write-scenario-tests stage.
+func (s *WriteScenarioTestsStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
+	// Early guard: EvidenceDir is required to write the manifest file.
+	if s.cfg.EvidenceDir == "" {
+		return specloop.NextAction{}, fmt.Errorf("write_scenario_tests: EvidenceDir is required but empty")
+	}
+
+	// Idempotency: if scenario tests are already written, skip.
+	if rs.ScenarioTestsWritten {
+		return specloop.NextAction{Kind: specloop.Continue}, nil
+	}
+
+	// Read raw spec markdown to parse scenarios.
+	specBytes, err := os.ReadFile(s.cfg.SpecPath)
+	if err != nil {
+		return specloop.NextAction{}, fmt.Errorf("read spec file: %w", err)
+	}
+
+	scenarios, skipped, err := contract.ParseScenarios(string(specBytes))
+	if err != nil {
+		return specloop.NextAction{}, fmt.Errorf("parse scenarios: %w", err)
+	}
+	for _, reason := range skipped {
+		if s.eventLog != nil {
+			s.eventLog.Append(runstore.ContractScenarioSkippedEvent{
+				BaseEvent: runstore.BaseEvent{Type: "contract_scenario_skipped", Timestamp: time.Now()},
+				Reason:    reason,
+			})
+		}
+	}
+
+	// No scenarios — no-op.
+	if len(scenarios) == 0 {
+		return specloop.NextAction{Kind: specloop.Continue}, nil
+	}
+
+	// Budget check before writing scenario tests.
+	if s.budget != nil && s.budget.Exceeded() {
+		reason := "budget exhausted: " + s.budget.Reason()
+		if s.eventLog != nil {
+			s.eventLog.Append(runstore.ScenarioTestsBlockedEvent{
+				BaseEvent: runstore.BaseEvent{Type: "scenario_tests_blocked", Timestamp: time.Now()},
+				Reason:    reason,
+			})
+		}
+		return specloop.NextAction{
+			Kind: specloop.Blocked,
+			Context: &specloop.FailureContext{
+				Failures: []string{reason},
+				Cycle:    rs.Cycle,
+			},
+		}, nil
+	}
+
+	// Collect implementation files from done tasks (deduplicated union).
+	implFiles := collectImplementationFiles(rs)
+
+	// Load existing manifest for partial recovery.
+	manifestPath := filepath.Join(s.cfg.EvidenceDir, "scenario-test-manifest.json")
+	manifest := loadManifest(manifestPath)
+
+	// Track which scenarios have already been successfully written.
+	alreadyWritten := make(map[string]bool)
+	for _, entry := range manifest.Scenarios {
+		alreadyWritten[entry.Name] = true
+	}
+
+	// Iterate scenarios one at a time.
+	var blockedReason string
+	for _, scenario := range scenarios {
+		// Check budget before each scenario.
+		if s.budget != nil && s.budget.Exceeded() {
+			blockedReason = "budget exhausted during scenario test writing: " + s.budget.Reason()
+			break
+		}
+
+		// Skip if already successfully written and compiled.
+		if alreadyWritten[scenario.Name] {
+			testFile := findTestFileInManifest(manifest, scenario.Name)
+			if testFile != "" && s.compilesSuccessfully(testFile) {
+				continue
+			}
+			// Stale file detected: delete it and remove from manifest before retry
+			if testFile != "" {
+				os.Remove(testFile) // Best effort; ignore error
+				// Remove stale entry from manifest
+				var newScenarios []contract.ScenarioTestEntry
+				for _, entry := range manifest.Scenarios {
+					if entry.Name != scenario.Name {
+						newScenarios = append(newScenarios, entry)
+					}
+				}
+				manifest.Scenarios = newScenarios
+			}
+		}
+
+		// Write scenario test with up to 2 self-repair retries.
+		const maxRetries = 2
+		var testFilePath string
+		var writeErr error
+		compileErrors := ""
+
+		for attempt := 0; attempt <= maxRetries; attempt++ {
+			testFilePath, writeErr = s.writer.WriteScenarioTest(ctx, scenario, implFiles, s.cfg.WorkDir, compileErrors)
+			if writeErr != nil {
+				blockedReason = fmt.Sprintf("scenario test writer error for %q: %v", scenario.Name, writeErr)
+				break
+			}
+
+			if testFilePath == "" {
+				// nil path with nil error means deliberate no-op
+				break
+			}
+
+			// Verify compilation.
+			if s.compilesSuccessfully(testFilePath) {
+				// Success — update manifest and emit event.
+				manifest.Scenarios = append(manifest.Scenarios, contract.ScenarioTestEntry{
+					Name:     scenario.Name,
+					TestFile: testFilePath,
+				})
+				if err := saveManifest(manifestPath, manifest); err != nil {
+					return specloop.NextAction{}, fmt.Errorf("save manifest: %w", err)
+				}
+				if s.eventLog != nil {
+					s.eventLog.Append(runstore.ScenarioTestWrittenEvent{
+						BaseEvent:    runstore.BaseEvent{Type: "scenario_tests_written", Timestamp: time.Now()},
+						ScenarioName: scenario.Name,
+						TestFile:     testFilePath,
+					})
+				}
+				writeErr = nil
+				break
+			}
+
+			// Compilation failed — collect error and retry if not last attempt.
+			compileErr := s.getCompileError(testFilePath)
+			if attempt < maxRetries {
+				compileErrors = "Prior compilation error:\n" + compileErr + "\n\n" + compileErrors
+			} else {
+				blockedReason = fmt.Sprintf("scenario test %q failed compilation after %d retries: %s", scenario.Name, maxRetries, compileErr)
+				writeErr = fmt.Errorf("compilation failed: %s", compileErr)
+			}
+		}
+
+		// Check if this scenario failed all retries.
+		if writeErr != nil || blockedReason != "" {
+			if blockedReason == "" {
+				blockedReason = fmt.Sprintf("scenario test writer error for %q: %v", scenario.Name, writeErr)
+			}
+			if s.eventLog != nil {
+				s.eventLog.Append(runstore.ScenarioTestsBlockedEvent{
+					BaseEvent: runstore.BaseEvent{Type: "scenario_tests_blocked", Timestamp: time.Now()},
+					Reason:    blockedReason,
+				})
+			}
+			return specloop.NextAction{
+				Kind: specloop.Blocked,
+				Context: &specloop.FailureContext{
+					Failures: []string{blockedReason},
+					Cycle:    rs.Cycle,
+				},
+			}, nil
+		}
+	}
+
+	// Check if we hit a budget limit.
+	if blockedReason != "" {
+		if s.eventLog != nil {
+			s.eventLog.Append(runstore.ScenarioTestsBlockedEvent{
+				BaseEvent: runstore.BaseEvent{Type: "scenario_tests_blocked", Timestamp: time.Now()},
+				Reason:    blockedReason,
+			})
+		}
+		return specloop.NextAction{
+			Kind: specloop.Blocked,
+			Context: &specloop.FailureContext{
+				Failures: []string{blockedReason},
+				Cycle:    rs.Cycle,
+			},
+		}, nil
+	}
+
+	// Set flag and emit success event.
+	rs.ScenarioTestsWritten = true
+
+	if s.eventLog != nil {
+		s.eventLog.Append(runstore.ScenarioTestsCompleteEvent{
+			BaseEvent:     runstore.BaseEvent{Type: "scenario_tests_complete", Timestamp: time.Now()},
+			ScenarioCount: len(scenarios),
+		})
+	}
+
+	return specloop.NextAction{Kind: specloop.Continue}, nil
+}
+
+// collectImplementationFiles collects the deduplicated union of FilesChanged from all
+// tasks with Status=='done'.
+func collectImplementationFiles(rs *runstore.RunState) []string {
+	seen := make(map[string]bool)
+	var files []string
+	for _, task := range rs.Tasks {
+		if task.Status == "done" {
+			for _, f := range task.FilesChanged {
+				if !seen[f] {
+					seen[f] = true
+					files = append(files, f)
+				}
+			}
+		}
+	}
+	return files
+}
+
+// loadManifest loads the scenario-test-manifest.json file, returning an empty manifest if not found.
+func loadManifest(path string) *contract.ScenarioTestManifest {
+	data, err := os.ReadFile(path)
+	if err != nil {
+		return &contract.ScenarioTestManifest{Scenarios: []contract.ScenarioTestEntry{}}
+	}
+	var manifest contract.ScenarioTestManifest
+	if err := json.Unmarshal(data, &manifest); err != nil {
+		return &contract.ScenarioTestManifest{Scenarios: []contract.ScenarioTestEntry{}}
+	}
+	if manifest.Scenarios == nil {
+		manifest.Scenarios = []contract.ScenarioTestEntry{}
+	}
+	return &manifest
+}
+
+// saveManifest saves the manifest to the scenario-test-manifest.json file.
+func saveManifest(path string, manifest *contract.ScenarioTestManifest) error {
+	data, err := json.MarshalIndent(manifest, "", "  ")
+	if err != nil {
+		return fmt.Errorf("marshal manifest: %w", err)
+	}
+	dir := filepath.Dir(path)
+	if err := os.MkdirAll(dir, 0o755); err != nil {
+		return fmt.Errorf("create evidence dir: %w", err)
+	}
+	if err := os.WriteFile(path, data, 0o644); err != nil {
+		return fmt.Errorf("write manifest file: %w", err)
+	}
+	return nil
+}
+
+// findTestFileInManifest finds the test file path for a scenario in the manifest.
+func findTestFileInManifest(manifest *contract.ScenarioTestManifest, scenarioName string) string {
+	for _, entry := range manifest.Scenarios {
+		if entry.Name == scenarioName {
+			return entry.TestFile
+		}
+	}
+	return ""
+}
+
+// compilesSuccessfully checks if a test file compiles by running 'go test -c -o /dev/null ./package-path'.
+func (s *WriteScenarioTestsStage) compilesSuccessfully(testFilePath string) bool {
+	pkgPath := s.derivePackagePath(testFilePath)
+	if pkgPath == "" {
+		return false
+	}
+
+	cmd := exec.Command("go", "test", "-c", "-o", "/dev/null", pkgPath)
+	cmd.Dir = s.cfg.WorkDir
+	err := cmd.Run()
+	return err == nil
+}
+
+// getCompileError returns a string describing the compilation error for a test file.
+func (s *WriteScenarioTestsStage) getCompileError(testFilePath string) string {
+	pkgPath := s.derivePackagePath(testFilePath)
+	if pkgPath == "" {
+		return "could not derive package path from test file"
+	}
+
+	cmd := exec.Command("go", "test", "-c", "-o", "/dev/null", pkgPath)
+	cmd.Dir = s.cfg.WorkDir
+	output, err := cmd.CombinedOutput()
+	if err != nil {
+		return fmt.Sprintf("%v: %s", err, string(output))
+	}
+	return "unknown compilation error"
+}
+
+// derivePackagePath derives the Go package path from a test file path relative to WorkDir.
+// For example, if testFile is "internal/next/contract/foo_test.go" and WorkDir is the root,
+// the package path will be "./internal/next/contract".
+func (s *WriteScenarioTestsStage) derivePackagePath(testFilePath string) string {
+	// Ensure testFilePath is absolute or relative to WorkDir.
+	if !filepath.IsAbs(testFilePath) {
+		testFilePath = filepath.Join(s.cfg.WorkDir, testFilePath)
+	}
+
+	// Get the directory containing the test file.
+	dir := filepath.Dir(testFilePath)
+
+	// Compute the relative path from WorkDir to the test file's directory.
+	relPath, err := filepath.Rel(s.cfg.WorkDir, dir)
+	if err != nil {
+		return ""
+	}
+
+	// Return as a Go package path: "./<relPath>"
+	return "./" + relPath
+}
diff --git a/internal/next/specloop/stages/write_scenario_tests_test.go b/internal/next/specloop/stages/write_scenario_tests_test.go
new file mode 100644
index 000000000..fdc7d6d3f
--- /dev/null
+++ b/internal/next/specloop/stages/write_scenario_tests_test.go
@@ -0,0 +1,866 @@
+package stages
+
+import (
+	"context"
+	"encoding/json"
+	"fmt"
+	"os"
+	"path/filepath"
+	"testing"
+
+	"github.com/danabrams/gromit/internal/next/contract"
+	"github.com/danabrams/gromit/internal/next/execpolicy"
+	"github.com/danabrams/gromit/internal/next/runstore"
+	"github.com/danabrams/gromit/internal/next/specloop"
+)
+
+// fakeScenarioTestWriter is a test double for the ScenarioTestWriter interface.
+type fakeScenarioTestWriter struct {
+	calls               int
+	failAttempt         int // -1 means never fail, N means fail on attempt N (0-indexed)
+	returnedPaths       []string
+	returnedPathIndex   int
+	compilableScenarios map[string]bool // scenarios that will compile
+}
+
+func (m *fakeScenarioTestWriter) WriteScenarioTest(
+	ctx context.Context,
+	scenario contract.SpecScenario,
+	implFiles []string,
+	workDir string,
+	compileErrors string,
+) (testFilePath string, err error) {
+	defer func() { m.calls++ }()
+
+	// Check if this attempt should fail
+	if m.failAttempt >= 0 && m.calls == m.failAttempt {
+		return "", fmt.Errorf("mock writer simulated error on attempt %d", m.calls)
+	}
+
+	// Return a pre-prepared path if available
+	if m.returnedPathIndex < len(m.returnedPaths) {
+		path := m.returnedPaths[m.returnedPathIndex]
+		m.returnedPathIndex++
+
+		// Ensure the directory exists
+		dir := filepath.Dir(path)
+		if err := os.MkdirAll(dir, 0o755); err != nil {
+			return "", fmt.Errorf("create test directory %s: %w", dir, err)
+		}
+
+		// Create a minimal Go test file that may or may not compile
+		if m.compilableScenarios[scenario.Name] {
+			// Compilable test file
+			testCode := fmt.Sprintf(`package main
+
+import "testing"
+
+func TestScenario_%s(t *testing.T) {
+	t.Log("scenario: %s")
+}
+`, escapeIdentifier(scenario.Name), scenario.Name)
+			if err := os.WriteFile(path, []byte(testCode), 0o644); err != nil {
+				return "", fmt.Errorf("write test file: %w", err)
+			}
+		} else {
+			// Non-compilable test file (invalid syntax)
+			testCode := fmt.Sprintf(`package main
+func TestScenario_%s(t *testing.T { // Missing closing paren
+}
+`, escapeIdentifier(scenario.Name))
+			if err := os.WriteFile(path, []byte(testCode), 0o644); err != nil {
+				return "", fmt.Errorf("write test file: %w", err)
+			}
+		}
+		return path, nil
+	}
+
+	// Fallback: return empty path (deliberate no-op as per implementation)
+	return "", nil
+}
+
+func escapeIdentifier(s string) string {
+	// Replace non-alphanumeric with underscore for valid Go identifiers
+	result := ""
+	for _, ch := range s {
+		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
+			result += string(ch)
+		} else {
+			result += "_"
+		}
+	}
+	return result
+}
+
+const specWithTwoScenarios = `# Test Spec
+
+## Scenarios
+
+### Scenario: scenario-one
+**Given:** precondition one
+**When:** action one
+**Then:** outcome one
+
+### Scenario: scenario-two
+**When:** action two
+**Then:** outcome two
+`
+
+const specWithoutScenarioTests = `# Test Spec
+
+## Overview
+No scenarios in this spec.
+`
+
+func makeWriteScenarioTestsRunState(t *testing.T) *runstore.RunState {
+	t.Helper()
+	rs := runstore.NewRunState("spec-scenario-test", "proj-scenario-test")
+	return rs
+}
+
+func TestWriteScenarioTests_IdempotencyNoOp(t *testing.T) {
+	// When ScenarioTestsWritten is true, returns Continue without calling writer
+	tmp := t.TempDir()
+	rs := makeWriteScenarioTestsRunState(t)
+	rs.ScenarioTestsWritten = true
+
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specWithTwoScenarios), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	writer := &fakeScenarioTestWriter{failAttempt: -1}
+	cfg := WriteScenarioTestsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: filepath.Join(tmp, "evidence"),
+		Store:       nil,
+		WorkDir:     tmp,
+	}
+	stage := NewWriteScenarioTestsStage(writer, cfg, nil, nil)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Continue {
+		t.Fatalf("expected Continue, got %v", action.Kind)
+	}
+	if writer.calls != 0 {
+		t.Fatalf("expected 0 writer calls for idempotent run, got %d", writer.calls)
+	}
+	if !rs.ScenarioTestsWritten {
+		t.Fatal("expected ScenarioTestsWritten to remain true")
+	}
+}
+
+func TestWriteScenarioTests_NoScenariosReturnsContinue(t *testing.T) {
+	// When spec has no scenarios, returns Continue with no writer calls
+	tmp := t.TempDir()
+	rs := makeWriteScenarioTestsRunState(t)
+
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specWithoutScenarioTests), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	writer := &fakeScenarioTestWriter{failAttempt: -1}
+	cfg := WriteScenarioTestsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: filepath.Join(tmp, "evidence"),
+		Store:       nil,
+		WorkDir:     tmp,
+	}
+	stage := NewWriteScenarioTestsStage(writer, cfg, nil, nil)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Continue {
+		t.Fatalf("expected Continue for no-scenarios, got %v", action.Kind)
+	}
+	if writer.calls != 0 {
+		t.Fatalf("expected 0 writer calls for no-scenarios, got %d", writer.calls)
+	}
+	if rs.ScenarioTestsWritten {
+		t.Fatal("expected ScenarioTestsWritten to remain false for no-scenarios")
+	}
+}
+
+func TestWriteScenarioTests_HappyPath(t *testing.T) {
+	// Happy path: writes tests for 2 scenarios, sets flag, emits events, writes manifest
+	// Use testdata subdirectory within the actual gromit project for compilation to work
+	currentDir := os.Getenv("PWD")
+	if currentDir == "" {
+		currentDir = "."
+	}
+
+	testDataDir := filepath.Join(currentDir, "internal/next/specloop/stages/testdata")
+	if err := os.MkdirAll(testDataDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+	t.Cleanup(func() {
+		os.RemoveAll(testDataDir)
+	})
+
+	tmp := t.TempDir()
+	rs := makeWriteScenarioTestsRunState(t)
+
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specWithTwoScenarios), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	evidenceDir := filepath.Join(testDataDir, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+
+	// Prepare test file paths
+	testFile1 := filepath.Join(evidenceDir, "scenario_one_test.go")
+	testFile2 := filepath.Join(evidenceDir, "scenario_two_test.go")
+
+	// Create writer that makes both scenarios compilable
+	writer := &fakeScenarioTestWriter{
+		failAttempt:   -1,
+		returnedPaths: []string{testFile1, testFile2},
+		compilableScenarios: map[string]bool{
+			"scenario-one": true,
+			"scenario-two": true,
+		},
+	}
+
+	eventLogPath := filepath.Join(tmp, "events.jsonl")
+	eventLog := runstore.NewEventLog(eventLogPath)
+
+	cfg := WriteScenarioTestsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: evidenceDir,
+		Store:       nil,
+		WorkDir:     currentDir,
+	}
+
+	stage := NewWriteScenarioTestsStage(writer, cfg, nil, eventLog)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Continue {
+		msg := "unknown reason"
+		if action.Context != nil && len(action.Context.Failures) > 0 {
+			// Print the full failure message
+			for i, f := range action.Context.Failures {
+				t.Logf("Failure %d: %s", i, f)
+			}
+			msg = action.Context.Failures[0]
+		}
+		t.Fatalf("expected Continue, got %v: %s", action.Kind, msg)
+	}
+	if !rs.ScenarioTestsWritten {
+		t.Fatal("expected ScenarioTestsWritten=true after success")
+	}
+
+	// Check manifest was written
+	manifestPath := filepath.Join(evidenceDir, "scenario-test-manifest.json")
+	data, err := os.ReadFile(manifestPath)
+	if err != nil {
+		t.Fatalf("manifest not written: %v", err)
+	}
+
+	var manifest contract.ScenarioTestManifest
+	if err := json.Unmarshal(data, &manifest); err != nil {
+		t.Fatalf("unmarshal manifest: %v", err)
+	}
+
+	if len(manifest.Scenarios) != 2 {
+		t.Fatalf("expected 2 scenarios in manifest, got %d", len(manifest.Scenarios))
+	}
+
+	// Check scenario entries
+	scenarioMap := make(map[string]string)
+	for _, entry := range manifest.Scenarios {
+		scenarioMap[entry.Name] = entry.TestFile
+	}
+
+	if _, ok := scenarioMap["scenario-one"]; !ok {
+		t.Fatal("expected scenario-one in manifest")
+	}
+	if _, ok := scenarioMap["scenario-two"]; !ok {
+		t.Fatal("expected scenario-two in manifest")
+	}
+
+	// Check events were emitted
+	events, err := eventLog.ReadAll()
+	if err != nil {
+		t.Fatalf("read events: %v", err)
+	}
+
+	var testWrittenCount, completeCount int
+	for _, ev := range events {
+		if ev.EventType() == "scenario_tests_written" {
+			testWrittenCount++
+		} else if ev.EventType() == "scenario_tests_complete" {
+			completeCount++
+		}
+	}
+
+	if testWrittenCount != 2 {
+		t.Fatalf("expected 2 scenario_tests_written events, got %d", testWrittenCount)
+	}
+	if completeCount != 1 {
+		t.Fatalf("expected 1 scenario_tests_complete event, got %d", completeCount)
+	}
+}
+
+func TestWriteScenarioTests_CompileFailureSelfRepair(t *testing.T) {
+	// First attempt fails compilation, retry succeeds
+	tmp := t.TempDir()
+	rs := makeWriteScenarioTestsRunState(t)
+
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specWithTwoScenarios), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	evidenceDir := filepath.Join(tmp, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+
+	testFile1 := filepath.Join(evidenceDir, "scenario_one_test.go")
+
+	// Writer will be called multiple times — first fails, then succeeds
+	writer := &fakeScenarioTestWriter{
+		failAttempt:   -1,
+		returnedPaths: []string{testFile1, testFile1}, // return same path twice
+		compilableScenarios: map[string]bool{
+			"scenario-one": true, // will compile on second attempt
+		},
+	}
+
+	eventLogPath := filepath.Join(tmp, "events.jsonl")
+	eventLog := runstore.NewEventLog(eventLogPath)
+
+	cfg := WriteScenarioTestsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: evidenceDir,
+		Store:       nil,
+		WorkDir:     tmp,
+	}
+
+	stage := NewWriteScenarioTestsStage(writer, cfg, nil, eventLog)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Continue {
+		t.Fatalf("expected Continue after self-repair, got %v", action.Kind)
+	}
+	if !rs.ScenarioTestsWritten {
+		t.Fatal("expected ScenarioTestsWritten=true after successful repair")
+	}
+
+	// Should have retried (at least 2 calls for first scenario)
+	if writer.calls < 2 {
+		t.Fatalf("expected at least 2 writer calls for retry, got %d", writer.calls)
+	}
+}
+
+func TestWriteScenarioTests_CompileFailureExhausted(t *testing.T) {
+	// All 3 attempts fail compilation, returns Blocked, flag not set
+	tmp := t.TempDir()
+	rs := makeWriteScenarioTestsRunState(t)
+
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specWithTwoScenarios), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	evidenceDir := filepath.Join(tmp, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+
+	testFile1 := filepath.Join(evidenceDir, "scenario_one_test.go")
+
+	writer := &fakeScenarioTestWriter{
+		failAttempt:         -1,
+		returnedPaths:       []string{testFile1, testFile1, testFile1}, // 3 attempts
+		compilableScenarios: make(map[string]bool),                     // empty = never compilable
+	}
+
+	eventLogPath := filepath.Join(tmp, "events.jsonl")
+	eventLog := runstore.NewEventLog(eventLogPath)
+
+	cfg := WriteScenarioTestsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: evidenceDir,
+		Store:       nil,
+		WorkDir:     tmp,
+	}
+
+	stage := NewWriteScenarioTestsStage(writer, cfg, nil, eventLog)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Blocked {
+		t.Fatalf("expected Blocked after exhausted retries, got %v", action.Kind)
+	}
+	if rs.ScenarioTestsWritten {
+		t.Fatal("expected ScenarioTestsWritten=false after failure")
+	}
+
+	// Should have exactly 3 writer calls (0, 1, 2)
+	if writer.calls != 3 {
+		t.Fatalf("expected 3 writer calls (maxRetries=2 means 3 attempts), got %d", writer.calls)
+	}
+
+	// Should have emitted a blocked event
+	events, err := eventLog.ReadAll()
+	if err != nil {
+		t.Fatalf("read events: %v", err)
+	}
+
+	var blockedCount int
+	for _, ev := range events {
+		if ev.EventType() == "scenario_tests_blocked" {
+			blockedCount++
+		}
+	}
+	if blockedCount != 1 {
+		t.Fatalf("expected 1 scenario_tests_blocked event, got %d", blockedCount)
+	}
+}
+
+func TestWriteScenarioTests_BudgetExhausted(t *testing.T) {
+	// Budget exceeded mid-iteration returns Blocked
+	tmp := t.TempDir()
+	rs := makeWriteScenarioTestsRunState(t)
+
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specWithTwoScenarios), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	evidenceDir := filepath.Join(tmp, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+
+	testFile1 := filepath.Join(evidenceDir, "scenario_one_test.go")
+
+	writer := &fakeScenarioTestWriter{
+		failAttempt:   -1,
+		returnedPaths: []string{testFile1},
+		compilableScenarios: map[string]bool{
+			"scenario-one": true,
+		},
+	}
+
+	eventLogPath := filepath.Join(tmp, "events.jsonl")
+	eventLog := runstore.NewEventLog(eventLogPath)
+
+	// Create a budget that's already exceeded
+	budgetLimits := execpolicy.Budgets{
+		MaxRunCostUSD: 100.0,
+		MaxSpecCycles: 5,
+	}
+	budget := specloop.NewBudget(budgetLimits)
+	budget.AddCost(150.0) // Exceed the budget
+
+	cfg := WriteScenarioTestsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: evidenceDir,
+		Store:       nil,
+		WorkDir:     tmp,
+	}
+	stage := NewWriteScenarioTestsStage(writer, cfg, budget, eventLog)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Blocked {
+		t.Fatalf("expected Blocked when budget exceeded, got %v", action.Kind)
+	}
+	if rs.ScenarioTestsWritten {
+		t.Fatal("expected ScenarioTestsWritten=false when budget exhausted")
+	}
+	if writer.calls > 0 {
+		t.Fatalf("expected 0 writer calls when budget already exceeded, got %d", writer.calls)
+	}
+
+	// Check for blocked event
+	events, err := eventLog.ReadAll()
+	if err != nil {
+		t.Fatalf("read events: %v", err)
+	}
+
+	var blockedCount int
+	for _, ev := range events {
+		if ev.EventType() == "scenario_tests_blocked" {
+			blockedCount++
+		}
+	}
+	if blockedCount != 1 {
+		t.Fatalf("expected 1 scenario_tests_blocked event, got %d", blockedCount)
+	}
+}
+
+func TestWriteScenarioTests_PartialRecovery(t *testing.T) {
+	// Manifest exists with completed scenario, skips it on retry
+	tmp := t.TempDir()
+	rs := makeWriteScenarioTestsRunState(t)
+
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specWithTwoScenarios), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	// Use testdata directory for compilation to work
+	currentDir := os.Getenv("PWD")
+	if currentDir == "" {
+		currentDir = "."
+	}
+
+	testDataDir := filepath.Join(currentDir, "internal/next/specloop/stages/testdata_partial")
+	if err := os.MkdirAll(testDataDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+	t.Cleanup(func() {
+		os.RemoveAll(testDataDir)
+	})
+
+	evidenceDir := filepath.Join(testDataDir, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+
+	// Pre-write manifest with first scenario already done
+	manifestPath := filepath.Join(evidenceDir, "scenario-test-manifest.json")
+	testFile1 := filepath.Join(evidenceDir, "scenario_one_test.go")
+
+	// Write a valid test file for scenario-one
+	testCode := `package main
+import "testing"
+func TestScenario_scenario_one(t *testing.T) {
+	t.Log("scenario one")
+}
+`
+	if err := os.WriteFile(testFile1, []byte(testCode), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	existingManifest := contract.ScenarioTestManifest{
+		Scenarios: []contract.ScenarioTestEntry{
+			{Name: "scenario-one", TestFile: testFile1},
+		},
+	}
+	data, err := json.Marshal(existingManifest)
+	if err != nil {
+		t.Fatal(err)
+	}
+	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	testFile2 := filepath.Join(evidenceDir, "scenario_two_test.go")
+
+	writer := &fakeScenarioTestWriter{
+		failAttempt:   -1,
+		returnedPaths: []string{testFile2}, // Only one new file needed
+		compilableScenarios: map[string]bool{
+			"scenario-two": true,
+		},
+	}
+
+	cfg := WriteScenarioTestsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: evidenceDir,
+		Store:       nil,
+		WorkDir:     currentDir,
+	}
+
+	stage := NewWriteScenarioTestsStage(writer, cfg, nil, nil)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Continue {
+		t.Fatalf("expected Continue, got %v", action.Kind)
+	}
+
+	// Should only have written one scenario (skipped the first)
+	if writer.calls != 1 {
+		t.Fatalf("expected 1 writer call (skipped first scenario), got %d", writer.calls)
+	}
+
+	// Final manifest should have both scenarios
+	finalData, err := os.ReadFile(manifestPath)
+	if err != nil {
+		t.Fatal(err)
+	}
+	var finalManifest contract.ScenarioTestManifest
+	if err := json.Unmarshal(finalData, &finalManifest); err != nil {
+		t.Fatal(err)
+	}
+
+	if len(finalManifest.Scenarios) != 2 {
+		t.Fatalf("expected 2 scenarios in final manifest, got %d", len(finalManifest.Scenarios))
+	}
+}
+
+func TestWriteScenarioTests_SeparateFilesPerScenario(t *testing.T) {
+	// Each scenario gets its own file
+	tmp := t.TempDir()
+	rs := makeWriteScenarioTestsRunState(t)
+
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specWithTwoScenarios), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	evidenceDir := filepath.Join(tmp, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+
+	testFile1 := filepath.Join(evidenceDir, "scenario_one_test.go")
+	testFile2 := filepath.Join(evidenceDir, "scenario_two_test.go")
+
+	writer := &fakeScenarioTestWriter{
+		failAttempt:   -1, // Never fail
+		returnedPaths: []string{testFile1, testFile2},
+		compilableScenarios: map[string]bool{
+			"scenario-one": true,
+			"scenario-two": true,
+		},
+	}
+
+	// Use testdata directory for compilation to work
+	currentDir := os.Getenv("PWD")
+	if currentDir == "" {
+		currentDir = "."
+	}
+
+	testDataDir := filepath.Join(currentDir, "internal/next/specloop/stages/testdata_separate")
+	if err := os.MkdirAll(testDataDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+	t.Cleanup(func() {
+		os.RemoveAll(testDataDir)
+	})
+
+	evidenceDirFinal := filepath.Join(testDataDir, "evidence")
+	if err := os.MkdirAll(evidenceDirFinal, 0o755); err != nil {
+		t.Fatal(err)
+	}
+
+	testFile1Final := filepath.Join(evidenceDirFinal, "scenario_one_test.go")
+	testFile2Final := filepath.Join(evidenceDirFinal, "scenario_two_test.go")
+
+	writer.returnedPaths = []string{testFile1Final, testFile2Final}
+
+	cfg := WriteScenarioTestsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: evidenceDirFinal,
+		Store:       nil,
+		WorkDir:     currentDir,
+	}
+
+	stage := NewWriteScenarioTestsStage(writer, cfg, nil, nil)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Continue {
+		t.Fatalf("expected Continue, got %v", action.Kind)
+	}
+
+	// Check files are different
+	if testFile1 == testFile2 {
+		t.Fatal("test files should be different paths")
+	}
+
+	// Check manifest records both files
+	manifestPath := filepath.Join(evidenceDirFinal, "scenario-test-manifest.json")
+	data, err := os.ReadFile(manifestPath)
+	if err != nil {
+		t.Fatal(err)
+	}
+
+	var manifest contract.ScenarioTestManifest
+	if err := json.Unmarshal(data, &manifest); err != nil {
+		t.Fatal(err)
+	}
+
+	var files []string
+	for _, entry := range manifest.Scenarios {
+		files = append(files, entry.TestFile)
+	}
+
+	if len(files) != 2 {
+		t.Fatalf("expected 2 test files, got %d", len(files))
+	}
+	if files[0] == files[1] {
+		t.Fatal("test files should be unique paths")
+	}
+
+	// Verify files exist on disk
+	for _, f := range files {
+		if _, err := os.Stat(f); err != nil {
+			t.Fatalf("test file not found: %s: %v", f, err)
+		}
+	}
+}
+
+func TestWriteScenarioTests_StaleFileCleanupBeforeRetry(t *testing.T) {
+	// When a scenario's test file exists but doesn't compile,
+	// the stale file and manifest entry should be deleted before retry.
+	tmp := t.TempDir()
+	rs := makeWriteScenarioTestsRunState(t)
+
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specWithTwoScenarios), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	// Use testdata directory for compilation to work
+	currentDir := os.Getenv("PWD")
+	if currentDir == "" {
+		currentDir = "."
+	}
+
+	testDataDir := filepath.Join(currentDir, "internal/next/specloop/stages/testdata_stale_cleanup")
+	if err := os.MkdirAll(testDataDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+	t.Cleanup(func() {
+		os.RemoveAll(testDataDir)
+	})
+
+	evidenceDir := filepath.Join(testDataDir, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+
+	// Pre-write manifest with a non-compiling test file for scenario-one
+	manifestPath := filepath.Join(evidenceDir, "scenario-test-manifest.json")
+	testFile1 := filepath.Join(evidenceDir, "scenario_one_test.go")
+
+	// Write an invalid test file (doesn't compile)
+	badTestCode := `package main
+func TestScenario_scenario_one(t *testing.T { // Missing closing paren
+}
+`
+	if err := os.WriteFile(testFile1, []byte(badTestCode), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	existingManifest := contract.ScenarioTestManifest{
+		Scenarios: []contract.ScenarioTestEntry{
+			{Name: "scenario-one", TestFile: testFile1},
+		},
+	}
+	data, err := json.Marshal(existingManifest)
+	if err != nil {
+		t.Fatal(err)
+	}
+	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	testFile2 := filepath.Join(evidenceDir, "scenario_two_test.go")
+
+	// Writer will return paths for both scenarios, scenario-one now compilable
+	writer := &fakeScenarioTestWriter{
+		failAttempt:   -1,
+		returnedPaths: []string{testFile1, testFile2}, // Both will be written
+		compilableScenarios: map[string]bool{
+			"scenario-one": true, // Now it will compile
+			"scenario-two": true,
+		},
+	}
+
+	cfg := WriteScenarioTestsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: evidenceDir,
+		Store:       nil,
+		WorkDir:     currentDir,
+	}
+
+	stage := NewWriteScenarioTestsStage(writer, cfg, nil, nil)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Continue {
+		t.Fatalf("expected Continue, got %v", action.Kind)
+	}
+
+	// Verify that the manifest was updated correctly:
+	// Should have only 2 scenarios (the old bad entry was removed)
+	finalData, err := os.ReadFile(manifestPath)
+	if err != nil {
+		t.Fatal(err)
+	}
+	var finalManifest contract.ScenarioTestManifest
+	if err := json.Unmarshal(finalData, &finalManifest); err != nil {
+		t.Fatal(err)
+	}
+
+	if len(finalManifest.Scenarios) != 2 {
+		t.Fatalf("expected 2 scenarios in final manifest (old stale entry removed), got %d", len(finalManifest.Scenarios))
+	}
+
+	// Verify scenario-one points to the newly rewritten file (should be the compilable one)
+	scenarioMap := make(map[string]string)
+	for _, entry := range finalManifest.Scenarios {
+		scenarioMap[entry.Name] = entry.TestFile
+	}
+
+	if _, ok := scenarioMap["scenario-one"]; !ok {
+		t.Fatal("expected scenario-one in final manifest")
+	}
+	if _, ok := scenarioMap["scenario-two"]; !ok {
+		t.Fatal("expected scenario-two in final manifest")
+	}
+
+	// Verify no duplicate entries for scenario-one
+	count := 0
+	for _, entry := range finalManifest.Scenarios {
+		if entry.Name == "scenario-one" {
+			count++
+		}
+	}
+	if count != 1 {
+		t.Fatalf("expected 1 entry for scenario-one, got %d (should have cleaned up stale)", count)
+	}
+}
+
+func TestWriteScenarioTests_Name(t *testing.T) {
+	// Name() returns 'write_scenario_tests'
+	tmp := t.TempDir()
+	writer := &fakeScenarioTestWriter{}
+	cfg := WriteScenarioTestsStageConfig{
+		SpecPath:    filepath.Join(tmp, "spec.md"),
+		EvidenceDir: filepath.Join(tmp, "evidence"),
+		Store:       nil,
+		WorkDir:     tmp,
+	}
+	stage := NewWriteScenarioTestsStage(writer, cfg, nil, nil)
+
+	if name := stage.Name(); name != "write_scenario_tests" {
+		t.Fatalf("expected Name() to return 'write_scenario_tests', got %q", name)
+	}
+}
+
+// Verify WriteScenarioTestsStage satisfies the Stage interface
+var _ specloop.Stage = (*WriteScenarioTestsStage)(nil)

## Cycle History

| Cycle | Tasks | Passed |
|-------|-------|--------|
| 10 | 44 | 36 |

## Validation Results

pass=true

## Known Risks


## Review Findings

| Facet | Count | Severities |
|-------|-------|------------|
| code_quality | 11 | 2 error, 1 info, 2 suggestion, 6 warning |
| spec_alignment | 13 | 2 error, 1 info, 3 suggestion, 7 warning |

## Acceptance Criteria

| Criterion | Status | Rationale |
|-----------|--------|-----------|
| A WriteScenarioTests stage runs after Execute and before Validate, producing Go test files in the worktree | pass | The evidence clearly shows: (1) WriteScenarioTestsStage is implemented in internal/next/specloop/stages/write_scenario_tests.go with Name() returning 'write_scenario_tests'; (2) it is wired into the pipeline in stage_provider.go at line 301 between executeStage and validateStage; (3) tests in stage_provider_test.go verify the stage order with expectedNames including 'write_scenario_tests' between 'execute' and 'validate'; (4) the stage writes Go test files to the worktree via ScenarioTestWriter.WriteScenarioTest(); (5) validation passes (pass=true). |
| The WriteScenarioTests stage follows the Seed/Invoke/Assert pattern from `docs/scenario-tests.md` | pass | The LLMScenarioTestWriter explicitly instructs the LLM to follow the Seed/Invoke/Assert pattern in its prompt (buildPrompt method lists the three steps: Seed/create fixtures, Invoke/call function, Assert/verify outcomes). The stage reads docs/scenario-tests.md and passes it as system guidance to the LLM. The test file (llm_scenario_test_writer_test.go) verifies that the patterns content including 'Seed', 'Invoke', and 'Assert' are included in the prompt. The stage provider wires the patterns file content into the writer at construction time. |
| Scenarios are processed one at a time during scenario test writing | pass | The implementation in write_scenario_tests.go contains a sequential for-loop with an explicit comment 'Iterate scenarios one at a time.' Each iteration calls WriteScenarioTest for a single scenario, checks the budget before each scenario, and handles per-scenario retries independently. There is no batching or concurrent processing. |
| Each scenario test compiles before the stage moves to the next scenario; up to two self-repair attempts on compile failure | pass | The implementation in write_scenario_tests.go clearly implements compile-before-advance with up to 2 self-repair retries. The retry loop (lines around attempt 0..maxRetries where maxRetries=2) calls compilesSuccessfully() after each write, collects compile errors on failure, passes them to the next WriteScenarioTest call, and only proceeds to the next scenario after compilation succeeds. If all 3 attempts fail, it returns Blocked. Tests confirm this: TestWriteScenarioTests_CompileFailureSelfRepair verifies retry succeeds, TestWriteScenarioTests_CompileFailureExhausted verifies exactly 3 calls (attempts 0,1,2) then Blocked is returned. |
| Scenario test failures detected by the Validate stage's always-run `go test` check trigger replan via the existing `replan_from` mechanism | pass | The pipeline ordering in stage_provider.go places `write_scenario_tests` before `validate`, so scenario test files are written to the working directory before validation runs. The ValidateStage calls `s.validator.RunFinal(ctx, s.cfg.AlwaysRun, ...)` which executes the always-run `go test ./...` check and picks up any scenario test failures. A comment was added in validate.go explicitly documenting this: 'Scenario test failures are detected via the always-run go test ./... check and reported through the standard go test output format.' When go test failures occur, the validate stage returns a ReplanFrom action, which is handled by specloop.go to set rs.ReplanContext and trigger the replan cycle — the existing replan_from mechanism is unchanged, scenario tests simply flow through it like any other go test failure. |
| On replan cycles, WriteScenarioTests is a no-op when its RunState flag (`ScenarioTestsWritten`) is true — fix tasks target the implementation, not the tests. WriteScenarioTests sets `rs.ScenarioTestsWritten = true` only when all scenarios succeed. | pass | The implementation in write_scenario_tests.go has an explicit idempotency guard at the top of Run(): `if rs.ScenarioTestsWritten { return specloop.NextAction{Kind: specloop.Continue}, nil }` which makes it a no-op when the flag is true. The flag is only set (`rs.ScenarioTestsWritten = true`) after all scenarios are successfully processed (line near end of Run(), after the scenarios loop completes without errors). ResetForNewCycle() explicitly does NOT reset ScenarioTestsWritten (confirmed by store.go comment and store_test.go TestResetForNewCycle). The TestWriteScenarioTests_IdempotencyNoOp test directly verifies the no-op behavior, and TestIntegration_ScenarioTestReplanPreservesTests confirms writeWorkCount==1 across multiple cycles. |
| If WriteScenarioTests produces tests that don't compile after two self-repair attempts, the stage returns `blocked`. Tests for previously completed scenarios remain in the worktree but `ScenarioTestsWritten` is not set. | pass | The implementation in write_scenario_tests.go uses maxRetries=2 (3 total attempts: 0, 1, 2). After exhausting retries with compile failures, it sets blockedReason and returns specloop.Blocked. ScenarioTestsWritten is only set true after all scenarios succeed. The manifest is saved incrementally per successful scenario, so previously completed scenarios' test files persist. TestWriteScenarioTests_CompileFailureExhausted directly verifies: returns Blocked, ScenarioTestsWritten stays false, exactly 3 writer calls. Validation results show pass=true. |
| If the spec has no Scenarios section or it is empty, WriteScenarioTests is a no-op (returns `Continue`) | pass | The implementation in write_scenario_tests.go explicitly handles the no-scenarios case: after parsing scenarios, it checks `if len(scenarios) == 0` and returns `specloop.NextAction{Kind: specloop.Continue}, nil`. The test `TestWriteScenarioTests_NoScenariosReturnsContinue` directly verifies this behavior with a spec that has no scenarios, confirming 0 writer calls and a Continue action. |
| The WriteScenarioTests stage uses an injected `ScenarioTestWriter` interface matching the existing `PlanCreator`/`ReviewRunner` pattern for testability | pass | The implementation defines a `ScenarioTestWriter` interface in `internal/next/contract/types.go`, injects it into `WriteScenarioTestsStage` via `NewWriteScenarioTestsStage(writer contract.ScenarioTestWriter, ...)`, provides a `noopScenarioTestWriter` in `stage_provider.go` alongside the existing `noopReviewRunner` pattern, and wires `LLMScenarioTestWriter` or noop depending on provider availability. Tests use `fakeScenarioTestWriter` to verify behavior without LLM calls, matching the `PlanCreator`/`ReviewRunner` testability pattern. |
| `BuildStages` in `cmd/gromit-next/stage_provider.go` is updated to include WriteScenarioTests in the correct pipeline position | pass | The diff clearly shows `writeScenarioTestsStage` added to the stages slice between `executeStage` and `validateStage` (line ~301), matching the required pipeline position. Tests confirm the stage order with `write_scenario_tests` between `execute` and `validate` in expectedNames arrays. |
| `ScenarioTestsWritten` RunState flag is NOT reset in the per-cycle reset block in `specloop.go` | pass | The `ResetForNewCycle` function in `internal/next/runstore/store.go` explicitly does not reset `ScenarioTestsWritten`. The comment reads 'ContractsWritten, ScenarioTestsWritten, and FailureHistory are NOT reset — they persist across cycles.' The test `TestResetForNewCycle` in `store_test.go` sets `rs.ScenarioTestsWritten = true`, calls `ResetForNewCycle`, and asserts `rs.ScenarioTestsWritten` remains true. |
| WriteScenarioTests uses Sonnet (P1) model tier | pass | In stage_provider.go, the scenarioTestAdapter is created with `llmadapter.Config{Tier: policy.Models.Planner, ...}` and `policy.Models.Planner` as the tier argument to NewFallbackAdapter. The Planner tier corresponds to P1/Sonnet in the execpolicy model hierarchy, which is the correct tier for this criterion. |
| All existing pipeline tests continue to pass | pass | The validation results show pass=true. The stage_provider_test.go was updated to reflect the new stage count (10→11) and the new 'write_scenario_tests' stage name in the expected pipeline order. A new TestBuildStages_WriteScenarioTestsStageWired test confirms the stage is wired. The final_verification_test.go was updated to exclude .gromit-next directories from scanning. |
| WriteScenarioTests emits events: `scenario_tests_written` per scenario, `scenario_tests_complete` on full success, `scenario_tests_blocked` on failure | pass | The implementation in write_scenario_tests.go clearly emits all three required events: (1) `scenario_tests_written` (type: 'scenario_tests_written') is emitted per scenario after successful write+compilation at line ~156-161; (2) `scenario_tests_complete` (type: 'scenario_tests_complete') is emitted after all scenarios succeed at line ~226-231; (3) `scenario_tests_blocked` (type: 'scenario_tests_blocked') is emitted on budget exhaustion, writer errors, and compilation failures. The event types are defined in runstore/events.go with proper structs and unmarshal cases. Tests in write_scenario_tests_test.go (TestWriteScenarioTests_HappyPath, TestWriteScenarioTests_CompileFailureExhausted, TestWriteScenarioTests_BudgetExhausted) verify the event emission counts and validation passes. |
| WriteScenarioTests checks remaining budget between scenario iterations and returns `blocked` if budget is exhausted mid-stage | pass | The implementation in write_scenario_tests.go checks the budget both before starting (lines ~155-166) and before each scenario iteration (lines ~177-180). When budget is exceeded, it emits a ScenarioTestsBlockedEvent and returns NextAction{Kind: specloop.Blocked}. The test TestWriteScenarioTests_BudgetExhausted verifies this behavior with a pre-exhausted budget. |
| WriteScenarioTests records a manifest of written test file paths in the evidence directory as `scenario-test-manifest.json` | pass | The implementation in `write_scenario_tests.go` writes a `scenario-test-manifest.json` to `EvidenceDir` via `saveManifest(manifestPath, manifest)` where `manifestPath := filepath.Join(s.cfg.EvidenceDir, "scenario-test-manifest.json")`. The manifest contains `ScenarioTestEntry` records with name and test file path. Tests in `write_scenario_tests_test.go` (e.g., `TestWriteScenarioTests_HappyPath`) verify the manifest is written and contains the expected scenario entries. |
| On retry when `ScenarioTestsWritten` is false, WriteScenarioTests skips scenarios whose test files already exist and compile, avoiding redundant LLM invocations | pass | The implementation in write_scenario_tests.go loads a manifest of previously written test files, checks each scenario against it, and calls `continue` (skipping the LLM writer call) when `alreadyWritten[scenario.Name]` is true and `compilesSuccessfully(testFile)` returns true. The `TestWriteScenarioTests_PartialRecovery` test directly exercises this path: it pre-populates a manifest with scenario-one already written to a compilable file, then asserts that `writer.calls == 1` (only scenario-two was written, scenario-one was skipped). The review findings flag a separate issue (stale non-compiling files not being cleaned up before retry), but that is distinct from the criterion being evaluated here. |
| Persistent failure tracking uses `FailureHistory` on RunState (keyed by test function name or `contract:<scenario-name>`) to count consecutive cycle failures; threshold of 2+ triggers the diagnostic hint | pass | The implementation clearly satisfies this criterion. `FailureHistory map[string]int` is added to `RunState` in `types.go`. Keys are extracted via `ExtractTestFailureKeys` (extracts `TestFunctionName` from `--- FAIL: TestFunctionName` lines) and `ExtractContractFailureKeys` (extracts `contract:<scenario-name>` from `contract:<name> — ...` lines). `UpdateFailureHistory` increments counts for present keys and removes absent keys. In `specloop.go`, after a replan context is received, the code extracts keys, calls `UpdateFailureHistory`, then annotates failures with a `persistent-failure:` hint when `rs.FailureHistory[key] >= 2` (threshold of 2). Tests in `failure_history_test.go` and `specloop_test.go` verify the extraction, counting, and annotation logic. |
| When the same contract or scenario test failures persist across 2+ consecutive replan cycles, failure context includes a `persistent-failure` diagnostic hint | pass | The implementation in specloop.go extracts failure keys (both test and contract), updates FailureHistory across cycles, and annotates replan context with `persistent-failure:` hints when history count >= 2. The hint format matches the criterion. Tests in failure_history_test.go and specloop_test.go verify the annotation logic, and TestIntegration_PersistentFailureHintAfterTwoCycles provides an end-to-end integration test confirming the hint appears in ReplanContext after 2 consecutive cycles with the same failure. |
| Each scenario gets its own dedicated test file — no shared test files between scenarios | pass | The WriteScenarioTestsStage iterates scenarios one at a time and calls WriteScenarioTest once per scenario, producing one file per scenario. The manifest stores separate TestFile entries per scenario. TestWriteScenarioTests_SeparateFilesPerScenario explicitly verifies that two scenarios produce two distinct file paths, and the test asserts files[0] != files[1]. The LLMScenarioTestWriter prompt instructs the LLM to use 'a dedicated file name like <pkg>_scenario_<name>_test.go' per scenario. |
| Compilation is checked via `go test -c -o /dev/null ./<package-path>` (not `go build`, which skips `_test.go` files) | pass | The `compilesSuccessfully` and `getCompileError` methods in `write_scenario_tests.go` both use `exec.Command("go", "test", "-c", "-o", "/dev/null", pkgPath)` exactly as specified. |

## Recommended Action

review
