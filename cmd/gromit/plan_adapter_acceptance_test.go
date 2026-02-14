//go:build acceptance

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPlanCommandThinAdapter verifies cmd/gromit/plan.go is a thin adapter
// Expected failure: runPlan() still contains business logic instead of delegating to Pipeline.Plan()
func TestPlanCommandThinAdapter(t *testing.T) {
	// Read the plan.go file
	planGoPath := filepath.Join(".", "plan.go")
	content, err := os.ReadFile(planGoPath)
	if err != nil {
		t.Fatalf("failed to read plan.go: %v", err)
	}

	planGoContent := string(content)

	// Verify it imports the pipeline package
	if !strings.Contains(planGoContent, `"github.com/danabrams/gromit/internal/pipeline"`) {
		t.Error("plan.go does not import internal/pipeline - not delegating to pipeline layer")
	}

	// Verify runPlan calls Pipeline.Plan()
	if !strings.Contains(planGoContent, "Pipeline.Plan") && !strings.Contains(planGoContent, "pipeline.Plan") {
		t.Error("plan.go does not call Pipeline.Plan() - business logic not extracted to pipeline")
	}

	// Verify business logic has been removed (these patterns should NOT appear in the thin adapter)
	businessLogicPatterns := []string{
		"frontmatter.ReadFile", // Spec loading moved to pipeline
		"beadClient.List()",    // Bead context gathering moved to pipeline
		"strings.Builder",      // Context building moved to pipeline
		"os.CreateTemp",        // Temp file writing moved to pipeline
		"agent.Resolve",        // Agent resolution moved to pipeline
		"selectedAgent.Launch", // Agent launch moved to pipeline
		"os.Stat(planPath)",    // Plan existence check moved to pipeline (after agent launch)
	}

	for _, pattern := range businessLogicPatterns {
		if strings.Contains(planGoContent, pattern) {
			t.Errorf("plan.go still contains business logic pattern %q - should be delegated to Pipeline.Plan()", pattern)
		}
	}
}

// TestPlanCommandPreservesInteractivePicker verifies CLI still handles interactive spec picker
// Expected failure: plan.go either removed picker or picker is in wrong layer
func TestPlanCommandPreservesInteractivePicker(t *testing.T) {
	// Read the plan.go file
	planGoPath := filepath.Join(".", "plan.go")
	content, err := os.ReadFile(planGoPath)
	if err != nil {
		t.Fatalf("failed to read plan.go: %v", err)
	}

	planGoContent := string(content)

	// Interactive picker should remain in CLI (interface layer responsibility)
	pickerPatterns := []string{
		"bufio.NewReader",
		"ReadString",
		"Select a spec",
	}

	missingPicker := true
	for _, pattern := range pickerPatterns {
		if strings.Contains(planGoContent, pattern) {
			missingPicker = false
			break
		}
	}

	if missingPicker {
		t.Error("plan.go does not contain interactive picker - user interaction should stay in CLI layer")
	}
}

// TestPlanCommandPreservesChaining verifies CLI still handles post-plan chaining
// Expected failure: chainAfterPlan is removed or chaining is in wrong layer
func TestPlanCommandPreservesChaining(t *testing.T) {
	// Read the plan.go file
	planGoPath := filepath.Join(".", "plan.go")
	content, err := os.ReadFile(planGoPath)
	if err != nil {
		t.Fatalf("failed to read plan.go: %v", err)
	}

	planGoContent := string(content)

	// Chaining should remain in CLI (interface layer responsibility)
	if !strings.Contains(planGoContent, "chainAfterPlan") && !strings.Contains(planGoContent, "chain") {
		t.Error("plan.go does not contain chaining logic - chaining should stay in CLI layer per spec")
	}
}

// TestPlanCommandUsesInteractiveSession verifies CLI gets session from Pipeline.Plan()
// Expected failure: runPlan does not use PlanSession returned by Pipeline.Plan()
func TestPlanCommandUsesInteractiveSession(t *testing.T) {
	// Read the plan.go file
	planGoPath := filepath.Join(".", "plan.go")
	content, err := os.ReadFile(planGoPath)
	if err != nil {
		t.Fatalf("failed to read plan.go: %v", err)
	}

	planGoContent := string(content)

	// Should use session returned by Pipeline.Plan()
	sessionPatterns := []string{
		"PlanSession",
		"session.Wait()",
		"session.Result()",
	}

	foundSessionUsage := false
	for _, pattern := range sessionPatterns {
		if strings.Contains(planGoContent, pattern) {
			foundSessionUsage = true
			break
		}
	}

	if !foundSessionUsage {
		t.Error("plan.go does not use PlanSession - should call Pipeline.Plan() and handle returned session")
	}
}

// TestPlanCommandLineCountReduced verifies plan.go is significantly reduced in size
// Expected failure: plan.go still has 265 lines - not thin enough for adapter
func TestPlanCommandLineCountReduced(t *testing.T) {
	// Run wc -l on plan.go
	cmd := exec.Command("wc", "-l", "plan.go")
	cmd.Dir = "."
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wc -l failed: %v\nOutput: %s", err, output)
	}

	// Parse line count
	parts := strings.Fields(string(output))
	if len(parts) < 2 {
		t.Fatalf("unexpected wc output: %s", output)
	}

	var lineCount int
	_, err = strings.NewReader(parts[0]).Read([]byte{})
	if _, scanErr := strings.NewReader(parts[0]).Read([]byte{}); scanErr == nil {
		// Parse the number
		if _, err := strings.NewReader(parts[0]).Read([]byte{}); err == nil {
			// Simple check: line count should be under 150 lines for thin adapter
			// Current: 265 lines. Target: <150 lines (flag parsing, picker, pipeline call, chaining)
			var count int
			if _, err := fmt.Sscanf(parts[0], "%d", &count); err == nil {
				lineCount = count
			}
		}
	}

	// Simpler approach: just parse the first field as int
	fmt.Sscanf(parts[0], "%d", &lineCount)

	// A thin adapter should be under 150 lines
	// Current plan.go is 265 lines, most of which is business logic
	if lineCount > 150 {
		t.Errorf("plan.go has %d lines, want <150 for thin adapter (currently 265 with business logic)", lineCount)
	}
}

// TestPlanCommandDetectsCreatedPlan verifies CLI uses PlanResult.CreatedPlans
// Expected failure: plan.go does not use PlanResult from Pipeline.Plan()
func TestPlanCommandDetectsCreatedPlan(t *testing.T) {
	// Read the plan.go file
	planGoPath := filepath.Join(".", "plan.go")
	content, err := os.ReadFile(planGoPath)
	if err != nil {
		t.Fatalf("failed to read plan.go: %v", err)
	}

	planGoContent := string(content)

	// Should use result.CreatedPlans instead of os.Stat(planPath)
	if strings.Contains(planGoContent, "os.Stat(planPath)") {
		t.Error("plan.go still uses os.Stat(planPath) - should use PlanResult.CreatedPlans from pipeline")
	}

	if !strings.Contains(planGoContent, "CreatedPlans") && !strings.Contains(planGoContent, "result") {
		t.Error("plan.go does not reference PlanResult - should call session.Result() for plan detection")
	}
}

// TestPlanInputStructUsage verifies CLI constructs PlanInput correctly
// Expected failure: runPlan does not construct pipeline.PlanInput
func TestPlanInputStructUsage(t *testing.T) {
	// Read the plan.go file
	planGoPath := filepath.Join(".", "plan.go")
	content, err := os.ReadFile(planGoPath)
	if err != nil {
		t.Fatalf("failed to read plan.go: %v", err)
	}

	planGoContent := string(content)

	// Should construct PlanInput with SpecName, AgentName, Force fields
	if !strings.Contains(planGoContent, "PlanInput") {
		t.Error("plan.go does not construct pipeline.PlanInput - should pass input struct to Pipeline.Plan()")
	}

	// Verify the three input fields are set
	inputFields := []string{
		"SpecName:",
		"AgentName:",
		"Force:",
	}

	missingFields := []string{}
	for _, field := range inputFields {
		if !strings.Contains(planGoContent, field) {
			missingFields = append(missingFields, field)
		}
	}

	if len(missingFields) > 0 {
		t.Errorf("plan.go missing PlanInput fields: %v - all input parameters must be passed to pipeline", missingFields)
	}
}

// TestPipelineInstantiation verifies CLI creates Pipeline with deps and paths
// Expected failure: runPlan does not instantiate pipeline.Pipeline
func TestPipelineInstantiation(t *testing.T) {
	// Read the plan.go file
	planGoPath := filepath.Join(".", "plan.go")
	content, err := os.ReadFile(planGoPath)
	if err != nil {
		t.Fatalf("failed to read plan.go: %v", err)
	}

	planGoContent := string(content)

	// Should instantiate Pipeline with New()
	if !strings.Contains(planGoContent, "pipeline.New") {
		t.Error("plan.go does not call pipeline.New() - should instantiate Pipeline with deps and paths")
	}

	// Should construct Deps struct
	if !strings.Contains(planGoContent, "pipeline.Deps") && !strings.Contains(planGoContent, "&Deps") {
		t.Error("plan.go does not construct pipeline.Deps - should provide dependencies to Pipeline")
	}

	// Should construct Paths struct
	if !strings.Contains(planGoContent, "pipeline.Paths") && !strings.Contains(planGoContent, "&Paths") {
		t.Error("plan.go does not construct pipeline.Paths - should provide paths to Pipeline")
	}
}

// TestNoDuplicateValidation verifies CLI does not duplicate spec validation
// Expected failure: plan.go still validates spec existence instead of letting Pipeline.Plan() handle it
func TestNoDuplicateValidation(t *testing.T) {
	// Read the plan.go file
	planGoPath := filepath.Join(".", "plan.go")
	content, err := os.ReadFile(planGoPath)
	if err != nil {
		t.Fatalf("failed to read plan.go: %v", err)
	}

	planGoContent := string(content)

	// CLI should NOT validate spec existence - that's pipeline's job
	// The only validation CLI does is before calling pipeline (e.g., checking args)
	if strings.Contains(planGoContent, `os.Stat(specPath); os.IsNotExist`) {
		t.Error("plan.go duplicates spec existence validation - Pipeline.Plan() should handle this")
	}

	// CLI should NOT check if plan already exists - that's pipeline's job
	if strings.Contains(planGoContent, "plan already exists") {
		t.Error("plan.go duplicates plan existence check - Pipeline.Plan() should handle this and respect Force flag")
	}
}

// TestContextPropagation verifies CLI passes context to Pipeline.Plan()
// Expected failure: runPlan does not create or pass context.Context
func TestContextPropagation(t *testing.T) {
	// Read the plan.go file
	planGoPath := filepath.Join(".", "plan.go")
	content, err := os.ReadFile(planGoPath)
	if err != nil {
		t.Fatalf("failed to read plan.go: %v", err)
	}

	planGoContent := string(content)

	// Should create context for cancellation/timeout support
	if !strings.Contains(planGoContent, "context.Background()") && !strings.Contains(planGoContent, "context.Context") {
		t.Error("plan.go does not use context.Context - should pass context to Pipeline.Plan() for cancellation support")
	}

	// Should pass context as first arg to Pipeline.Plan()
	if !strings.Contains(planGoContent, ".Plan(ctx,") && !strings.Contains(planGoContent, ".Plan(context.") {
		t.Error("plan.go does not pass context to Pipeline.Plan() - context should be first parameter")
	}
}

// TestFilterUnplannedSpecsInCLI verifies CLI still filters unplanned specs for picker
// Expected failure: filterUnplannedSpecs removed or moved to wrong layer
func TestFilterUnplannedSpecsInCLI(t *testing.T) {
	// Read the plan.go file
	planGoPath := filepath.Join(".", "plan.go")
	content, err := os.ReadFile(planGoPath)
	if err != nil {
		t.Fatalf("failed to read plan.go: %v", err)
	}

	planGoContent := string(content)

	// filterUnplannedSpecs should stay in CLI for picker (interface-specific logic)
	if !strings.Contains(planGoContent, "filterUnplannedSpecs") {
		t.Error("plan.go missing filterUnplannedSpecs - CLI should filter specs before showing picker")
	}
}

// TestGetSpecFilesInCLI verifies CLI still scans spec directory for picker
// Expected failure: getSpecFiles removed or moved to wrong layer
func TestGetSpecFilesInCLI(t *testing.T) {
	// Read the plan.go file
	planGoPath := filepath.Join(".", "plan.go")
	content, err := os.ReadFile(planGoPath)
	if err != nil {
		t.Fatalf("failed to read plan.go: %v", err)
	}

	planGoContent := string(content)

	// getSpecFiles should stay in CLI for picker (interface-specific logic)
	// OR it could be moved to a shared location, but picker needs it
	if !strings.Contains(planGoContent, "getSpecFiles") && !strings.Contains(planGoContent, "ListMarkdownFiles") {
		t.Error("plan.go missing spec file scanning - CLI needs to scan specs for interactive picker")
	}
}
