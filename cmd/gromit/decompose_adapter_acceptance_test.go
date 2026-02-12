//go:build acceptance

package main

import (
	"os"
	"strings"
	"testing"
)

// TestDecomposeCLIAdapterIsThin verifies decompose.go becomes a thin adapter after refactoring.
// Expected failure: decomposeSinglePlan() still contains business logic (JSON parsing, bead creation,
// frontmatter updates, dependency mapping) instead of delegating to pipeline.Pipeline.Decompose().
func TestDecomposeCLIAdapterIsThin(t *testing.T) {
	// Read decompose.go source
	sourceContent, err := os.ReadFile("decompose.go")
	if err != nil {
		t.Fatalf("Failed to read decompose.go: %v", err)
	}

	source := string(sourceContent)

	// After refactoring, these business logic calls should NOT be in decompose.go
	businessLogicIndicators := []string{
		"jsonutil.ExtractJSON(",           // JSON parsing moved to pipeline
		"beadClient.CreateWithDepsAndDescription(", // Bead creation moved to pipeline
		"frontmatter.UpdateFile(",         // Frontmatter updates moved to pipeline
		"buildDecomposePrompt(",           // Prompt building moved to pipeline
		"for i, def := range beadDefs",    // Iteration over bead defs moved to pipeline
		"depends_on_index",                // Dependency mapping moved to pipeline
		"parsePriority(",                  // Priority parsing moved to pipeline
		"DependsOnIndex",                  // Dependency index handling moved to pipeline
		"createdBeads = append(createdBeads", // Bead tracking moved to pipeline
		"spec:%s\", planName",             // Label construction moved to pipeline
	}

	var foundBusinessLogic []string
	for _, indicator := range businessLogicIndicators {
		if strings.Contains(source, indicator) {
			foundBusinessLogic = append(foundBusinessLogic, indicator)
		}
	}

	if len(foundBusinessLogic) > 0 {
		t.Errorf("decompose.go still contains business logic after refactoring:\n%s\n\nExpected these to be moved to pipeline.Pipeline.Decompose()",
			strings.Join(foundBusinessLogic, "\n"))
	}
}

// TestDecomposeCLICallsPipelineDecompose verifies decompose command delegates to pipeline.
// Expected failure: decomposeSinglePlan() does not call pipeline.Pipeline.Decompose() yet.
func TestDecomposeCLICallsPipelineDecompose(t *testing.T) {
	sourceContent, err := os.ReadFile("decompose.go")
	if err != nil {
		t.Fatalf("Failed to read decompose.go: %v", err)
	}

	source := string(sourceContent)

	// After refactoring, should call pipeline methods
	if !strings.Contains(source, "pipeline.Decompose(") && !strings.Contains(source, "p.Decompose(") {
		t.Error("decompose.go does not call pipeline.Decompose(), expected delegation to pipeline package")
	}

	// Should create Pipeline instance
	if !strings.Contains(source, "pipeline.New(") {
		t.Error("decompose.go does not call pipeline.New(), expected Pipeline initialization")
	}
}

// TestDecomposeCLIAdapterRetainsUIResponsibilities verifies CLI layer keeps interface duties.
// Expected failure: after refactoring, these responsibilities might be incorrectly moved
// to pipeline package instead of staying in CLI adapter.
func TestDecomposeCLIAdapterRetainsUIResponsibilities(t *testing.T) {
	sourceContent, err := os.ReadFile("decompose.go")
	if err != nil {
		t.Fatalf("Failed to read decompose.go: %v", err)
	}

	source := string(sourceContent)

	// These should remain in CLI layer
	cliResponsibilities := map[string]string{
		"cobra.Command":        "Cobra command definition",
		"Flags()":              "Flag parsing",
		"fmt.Printf":           "Output formatting",
		"fmt.Println":          "Output formatting",
		"bufio.Reader":         "User input handling (picker)",
		"confirmPrompt":        "Review mode confirmation",
		"filterUndecomposedPlans": "Plan picker logic",
	}

	for pattern, description := range cliResponsibilities {
		if !strings.Contains(source, pattern) {
			t.Errorf("decompose.go is missing %s (%s) - CLI adapter should retain UI responsibilities",
				description, pattern)
		}
	}
}

// TestDecomposeCLIAdapterHandlesChaining verifies chaining stays in CLI layer.
// Expected failure: chainAfterDecompose call might be removed during refactoring, but
// chaining is a CLI responsibility and should remain.
func TestDecomposeCLIAdapterHandlesChaining(t *testing.T) {
	sourceContent, err := os.ReadFile("decompose.go")
	if err != nil {
		t.Fatalf("Failed to read decompose.go: %v", err)
	}

	source := string(sourceContent)

	// Chaining should remain in CLI
	if !strings.Contains(source, "chainAfterDecompose(") {
		t.Error("decompose.go does not call chainAfterDecompose(), chaining should remain in CLI adapter")
	}
}

// TestDecomposeCLIAdapterFormatsOutput verifies CLI formats result for display.
// Expected failure: decomposeSinglePlan() does not extract and format result.CreatedBeads
// for user display yet - result formatting should be CLI responsibility.
func TestDecomposeCLIAdapterFormatsOutput(t *testing.T) {
	sourceContent, err := os.ReadFile("decompose.go")
	if err != nil {
		t.Fatalf("Failed to read decompose.go: %v", err)
	}

	source := string(sourceContent)

	// Should access result from pipeline
	if !strings.Contains(source, "result.CreatedBeads") && !strings.Contains(source, ".CreatedBeads") {
		t.Error("decompose.go does not access result.CreatedBeads, expected CLI to format results for display")
	}
}

// TestDecomposeCLIAdapterHandlesReviewMode verifies review mode uses pipeline Review flag.
// Expected failure: decompose.go still implements review confirmation inline instead of
// passing Review flag to pipeline.Decompose() and handling result display.
func TestDecomposeCLIAdapterHandlesReviewMode(t *testing.T) {
	sourceContent, err := os.ReadFile("decompose.go")
	if err != nil {
		t.Fatalf("Failed to read decompose.go: %v", err)
	}

	source := string(sourceContent)

	// Should pass Review flag to pipeline in DecomposeInput
	if !strings.Contains(source, "Review:") && !strings.Contains(source, "review:") {
		t.Error("decompose.go does not set Review field in DecomposeInput, expected review mode to be passed to pipeline")
	}

	// Review confirmation display should remain in CLI
	if !strings.Contains(source, "promptReviewBeads(") {
		t.Error("decompose.go missing promptReviewBeads(), review display is CLI responsibility")
	}
}

// TestDecomposeCLIAdapterPassesForceFlag verifies Force flag is passed to pipeline.
// Expected failure: decompose.go does not pass Force flag to pipeline.Decompose() in DecomposeInput yet.
func TestDecomposeCLIAdapterPassesForceFlag(t *testing.T) {
	sourceContent, err := os.ReadFile("decompose.go")
	if err != nil {
		t.Fatalf("Failed to read decompose.go: %v", err)
	}

	source := string(sourceContent)

	// Should pass Force flag in DecomposeInput
	if !strings.Contains(source, "Force:") && !strings.Contains(source, "force:") {
		t.Error("decompose.go does not set Force field in DecomposeInput, expected force flag to be passed to pipeline")
	}
}

// TestDecomposeCLIAdapterBuildsDecomposeInput verifies DecomposeInput construction.
// Expected failure: decomposeSinglePlan() does not create DecomposeInput struct yet.
func TestDecomposeCLIAdapterBuildsDecomposeInput(t *testing.T) {
	sourceContent, err := os.ReadFile("decompose.go")
	if err != nil {
		t.Fatalf("Failed to read decompose.go: %v", err)
	}

	source := string(sourceContent)

	// Should construct DecomposeInput
	if !strings.Contains(source, "DecomposeInput{") && !strings.Contains(source, "pipeline.DecomposeInput{") {
		t.Error("decompose.go does not create DecomposeInput struct, expected input struct construction for pipeline")
	}

	// Should set PlanName field
	if !strings.Contains(source, "PlanName:") {
		t.Error("decompose.go does not set PlanName field in DecomposeInput")
	}
}

// TestDecomposeCLIAdapterHandlesDecomposeAllFlow verifies decomposeAll iterates plans correctly.
// Expected failure: decomposeAll() still calls decomposeSinglePlan() with business logic
// instead of calling pipeline.Decompose() for each plan.
func TestDecomposeCLIAdapterHandlesDecomposeAllFlow(t *testing.T) {
	sourceContent, err := os.ReadFile("decompose.go")
	if err != nil {
		t.Fatalf("Failed to read decompose.go: %v", err)
	}

	source := string(sourceContent)

	// decomposeAll should remain in CLI as it handles iteration and progress display
	if !strings.Contains(source, "func decomposeAll(") {
		t.Error("decompose.go missing decomposeAll(), batch processing should remain in CLI adapter")
	}

	// Should format summary output
	if !strings.Contains(source, "successCount") || !strings.Contains(source, "failedPlans") {
		t.Error("decompose.go missing summary tracking in decomposeAll(), CLI should track and display batch results")
	}
}

// TestDecomposeCLIAdapterCreatesBeadClientInterface verifies bead client is passed to pipeline.
// Expected failure: decompose.go does not instantiate bead.Client and pass to pipeline.Deps yet.
func TestDecomposeCLIAdapterCreatesBeadClientInterface(t *testing.T) {
	sourceContent, err := os.ReadFile("decompose.go")
	if err != nil {
		t.Fatalf("Failed to read decompose.go: %v", err)
	}

	source := string(sourceContent)

	// Should create bead client
	if !strings.Contains(source, "bead.NewClient()") {
		t.Error("decompose.go does not call bead.NewClient(), expected bead client instantiation")
	}

	// Should create Deps struct with BeadClient
	if !strings.Contains(source, "Deps{") && !strings.Contains(source, "pipeline.Deps{") {
		t.Error("decompose.go does not create pipeline.Deps struct, expected dependency injection")
	}
}

// TestDecomposeCLIAdapterCreatesClaudeClientInterface verifies Claude client is passed to pipeline.
// Expected failure: decompose.go does not instantiate claude.Client and pass to pipeline.Deps yet.
func TestDecomposeCLIAdapterCreatesClaudeClientInterface(t *testing.T) {
	sourceContent, err := os.ReadFile("decompose.go")
	if err != nil {
		t.Fatalf("Failed to read decompose.go: %v", err)
	}

	source := string(sourceContent)

	// Should create Claude client
	if !strings.Contains(source, "claude.NewClient(") {
		t.Error("decompose.go does not call claude.NewClient(), expected Claude client instantiation")
	}

	// ClaudeClient should be passed in Deps
	if !strings.Contains(source, "ClaudeClient:") {
		t.Error("decompose.go does not set ClaudeClient field in pipeline.Deps")
	}
}

// TestDecomposeCLIAdapterRemovesBeadDefStruct verifies beadDef type is removed from CLI.
// Expected failure: beadDef struct still exists in decompose.go instead of being moved to pipeline package.
func TestDecomposeCLIAdapterRemovesBeadDefStruct(t *testing.T) {
	sourceContent, err := os.ReadFile("decompose.go")
	if err != nil {
		t.Fatalf("Failed to read decompose.go: %v", err)
	}

	source := string(sourceContent)

	// beadDef should be moved to pipeline package
	if strings.Contains(source, "type beadDef struct") {
		t.Error("decompose.go still defines beadDef struct, expected to be moved to pipeline package")
	}
}

// TestDecomposeCLIAdapterRemovesBuildDecomposePrompt verifies buildDecomposePrompt is removed.
// Expected failure: buildDecomposePrompt() still exists in decompose.go instead of being
// moved to pipeline package or handled by PromptRenderer.
func TestDecomposeCLIAdapterRemovesBuildDecomposePrompt(t *testing.T) {
	sourceContent, err := os.ReadFile("decompose.go")
	if err != nil {
		t.Fatalf("Failed to read decompose.go: %v", err)
	}

	source := string(sourceContent)

	// buildDecomposePrompt should be moved to pipeline
	if strings.Contains(source, "func buildDecomposePrompt(") {
		t.Error("decompose.go still defines buildDecomposePrompt(), expected to be moved to pipeline package")
	}
}

// TestDecomposeCLIAdapterRemovesParsePriority verifies parsePriority is removed.
// Expected failure: parsePriority() still exists in decompose.go instead of being
// moved to pipeline package as internal parsing logic.
func TestDecomposeCLIAdapterRemovesParsePriority(t *testing.T) {
	sourceContent, err := os.ReadFile("decompose.go")
	if err != nil {
		t.Fatalf("Failed to read decompose.go: %v", err)
	}

	source := string(sourceContent)

	// parsePriority should be moved to pipeline
	if strings.Contains(source, "func parsePriority(") {
		t.Error("decompose.go still defines parsePriority(), expected to be moved to pipeline package")
	}
}

// TestDecomposeCLIAdapterHandlesErrors verifies error handling from pipeline.
// Expected failure: decompose.go does not check and format errors from pipeline.Decompose() yet.
func TestDecomposeCLIAdapterHandlesErrors(t *testing.T) {
	sourceContent, err := os.ReadFile("decompose.go")
	if err != nil {
		t.Fatalf("Failed to read decompose.go: %v", err)
	}

	source := string(sourceContent)

	// Should check error from Decompose call
	if !strings.Contains(source, "err := p.Decompose(") && !strings.Contains(source, ", err := p.Decompose(") {
		t.Error("decompose.go does not check error return from pipeline.Decompose()")
	}

	// Should handle errors appropriately
	if !strings.Contains(source, "if err != nil") {
		t.Error("decompose.go missing error handling after pipeline.Decompose() call")
	}
}

// TestDecomposeCLIAdapterCreatesPaths verifies Paths struct construction.
// Expected failure: decompose.go does not create pipeline.Paths struct yet.
func TestDecomposeCLIAdapterCreatesPaths(t *testing.T) {
	sourceContent, err := os.ReadFile("decompose.go")
	if err != nil {
		t.Fatalf("Failed to read decompose.go: %v", err)
	}

	source := string(sourceContent)

	// Should create Paths struct
	if !strings.Contains(source, "Paths{") && !strings.Contains(source, "pipeline.Paths{") {
		t.Error("decompose.go does not create pipeline.Paths struct, expected paths configuration")
	}

	// Should set PlansDir
	if !strings.Contains(source, "PlansDir:") {
		t.Error("decompose.go does not set PlansDir field in pipeline.Paths")
	}
}

// TestDecomposeCLIAdapterHandlesContext verifies context.Context is passed to pipeline.
// Expected failure: decompose.go does not create and pass context.Context to pipeline.Decompose() yet.
func TestDecomposeCLIAdapterHandlesContext(t *testing.T) {
	sourceContent, err := os.ReadFile("decompose.go")
	if err != nil {
		t.Fatalf("Failed to read decompose.go: %v", err)
	}

	source := string(sourceContent)

	// Should create context
	if !strings.Contains(source, "context.Background()") && !strings.Contains(source, "ctx := context.") {
		t.Error("decompose.go does not create context.Context for pipeline.Decompose() call")
	}
}
