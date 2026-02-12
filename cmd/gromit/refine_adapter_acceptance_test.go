//go:build acceptance

package main

import (
	"os"
	"strings"
	"testing"
)

// TestRefineCLIAdapterIsThin verifies refine.go becomes a thin adapter after refactoring.
// Expected failure: runRefine() still contains business logic (backlog loading, spec detection,
// post-processing) instead of delegating to pipeline.Pipeline.Refine().
func TestRefineCLIAdapterIsThin(t *testing.T) {
	// Read refine.go source
	sourceContent, err := os.ReadFile("refine.go")
	if err != nil {
		t.Fatalf("Failed to read refine.go: %v", err)
	}

	source := string(sourceContent)

	// After refactoring, these business logic calls should NOT be in refine.go
	businessLogicIndicators := []string{
		"getSpecFiles(specsDir)",                    // Spec detection moved to pipeline
		"containsSpec(",                             // Comparison logic moved to pipeline
		"bf.Update(",                                // Backlog updates moved to pipeline
		"bf.Add(",                                   // Backlog creation moved to pipeline
		"extractSpecTitle(",                         // Title extraction moved to pipeline
		"for _, spec := range newSpecs",             // Iteration over new specs moved to pipeline
		"if fromBacklog && len(createdSpecs) > 0 {", // Conditional logic moved to pipeline
		"if isBlankSession && len(createdSpecs)",    // Blank session logic moved to pipeline
	}

	var foundBusinessLogic []string
	for _, indicator := range businessLogicIndicators {
		if strings.Contains(source, indicator) {
			foundBusinessLogic = append(foundBusinessLogic, indicator)
		}
	}

	if len(foundBusinessLogic) > 0 {
		t.Errorf("refine.go still contains business logic after refactoring:\n%s\n\nExpected these to be moved to pipeline.Pipeline.Refine()",
			strings.Join(foundBusinessLogic, "\n"))
	}
}

// TestRefineCLICallsPipelineRefine verifies refine command delegates to pipeline.
// Expected failure: runRefine() does not call pipeline.Pipeline.Refine() yet.
func TestRefineCLICallsPipelineRefine(t *testing.T) {
	sourceContent, err := os.ReadFile("refine.go")
	if err != nil {
		t.Fatalf("Failed to read refine.go: %v", err)
	}

	source := string(sourceContent)

	// After refactoring, should call pipeline methods
	if !strings.Contains(source, "pipeline.Refine(") && !strings.Contains(source, "p.Refine(") {
		t.Error("refine.go does not call pipeline.Refine(), expected delegation to pipeline package")
	}

	// Should create Pipeline instance
	if !strings.Contains(source, "pipeline.New(") {
		t.Error("refine.go does not call pipeline.New(), expected Pipeline initialization")
	}
}

// TestRefineCLIAdapterRetainsUIResponsibilities verifies CLI layer keeps interface duties.
// Expected failure: after refactoring, these responsibilities might be incorrectly moved
// to pipeline package instead of staying in CLI adapter.
func TestRefineCLIAdapterRetainsUIResponsibilities(t *testing.T) {
	sourceContent, err := os.ReadFile("refine.go")
	if err != nil {
		t.Fatalf("Failed to read refine.go: %v", err)
	}

	source := string(sourceContent)

	// These should remain in CLI layer
	cliResponsibilities := map[string]string{
		"cobra.Command": "Cobra command definition",
		"Flags()":       "Flag parsing",
		"fmt.Printf":    "Output formatting",
		"fmt.Println":   "Output formatting",
		"bufio.Reader":  "User input handling (picker)",
	}

	for pattern, description := range cliResponsibilities {
		if !strings.Contains(source, pattern) {
			t.Errorf("refine.go is missing %s (%s) - CLI adapter should retain UI responsibilities",
				description, pattern)
		}
	}
}

// TestRefineCLIAdapterDrainsEventStream verifies CLI drains session event stream.
// Expected failure: runRefine() does not implement event stream draining loop yet -
// it does not call session.Events() and route events to stdout.
func TestRefineCLIAdapterDrainsEventStream(t *testing.T) {
	sourceContent, err := os.ReadFile("refine.go")
	if err != nil {
		t.Fatalf("Failed to read refine.go: %v", err)
	}

	source := string(sourceContent)

	// Should drain events from session
	if !strings.Contains(source, "session.Events()") && !strings.Contains(source, ".Events()") {
		t.Error("refine.go does not call session.Events(), expected event stream handling for interactive mode")
	}

	// Should iterate over events
	if !strings.Contains(source, "for") && !strings.Contains(source, "range") {
		// This is a weak check, but acceptance tests verify behavior not syntax
		t.Log("Warning: could not find event iteration pattern")
	}
}

// TestRefineCLIAdapterHandlesChaining verifies chaining stays in CLI layer.
// Expected failure: chainAfterRefine call might be removed during refactoring, but
// chaining is a CLI responsibility and should remain.
func TestRefineCLIAdapterHandlesChaining(t *testing.T) {
	sourceContent, err := os.ReadFile("refine.go")
	if err != nil {
		t.Fatalf("Failed to read refine.go: %v", err)
	}

	source := string(sourceContent)

	// Chaining should remain in CLI
	if !strings.Contains(source, "chainAfterRefine(") {
		t.Error("refine.go does not call chainAfterRefine(), chaining should remain in CLI adapter")
	}
}

// TestRefineCLIAdapterFormatsOutput verifies CLI formats result for display.
// Expected failure: runRefine() does not extract and format result.CreatedSpecs for
// user display yet - result formatting should be CLI responsibility.
func TestRefineCLIAdapterFormatsOutput(t *testing.T) {
	sourceContent, err := os.ReadFile("refine.go")
	if err != nil {
		t.Fatalf("Failed to read refine.go: %v", err)
	}

	source := string(sourceContent)

	// Should access result from session
	if !strings.Contains(source, "result.CreatedSpecs") && !strings.Contains(source, ".CreatedSpecs") {
		t.Error("refine.go does not access result.CreatedSpecs, expected CLI to format results for display")
	}

	// Should print success messages
	hasSuccessOutput := strings.Contains(source, "Spec files created") ||
		strings.Contains(source, "Created backlog item") ||
		strings.Contains(source, "Marked backlog item")

	if !hasSuccessOutput {
		t.Error("refine.go missing success output messages, CLI should format results for user")
	}
}

// TestRefineCLIAdapterSizeConstraint verifies refactored CLI is approximately 50 lines.
// Expected failure: refine.go is currently ~400 lines, should be ~50 lines after extraction.
func TestRefineCLIAdapterSizeConstraint(t *testing.T) {
	sourceContent, err := os.ReadFile("refine.go")
	if err != nil {
		t.Fatalf("Failed to read refine.go: %v", err)
	}

	lines := strings.Split(string(sourceContent), "\n")

	// Count non-empty, non-comment lines in runRefine function
	inRunRefine := false
	funcLines := 0
	braceDepth := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "func runRefine(") {
			inRunRefine = true
			funcLines = 1
			continue
		}

		if !inRunRefine {
			continue
		}

		// Track brace depth to know when function ends
		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")

		if braceDepth == 0 && funcLines > 0 {
			// Function ended
			break
		}

		// Count non-empty, non-comment lines
		if trimmed != "" && !strings.HasPrefix(trimmed, "//") {
			funcLines++
		}
	}

	// Allow some flexibility: target is 50 lines, but accept up to 80 lines
	maxAcceptableLines := 80
	if funcLines > maxAcceptableLines {
		t.Errorf("runRefine() has %d significant lines, want ≤%d lines (target ~50)\nRefactoring should move business logic to pipeline package",
			funcLines, maxAcceptableLines)
	}
}

// TestRefineCLIAdapterPreservesExistingBehavior verifies no user-visible changes.
// Expected failure: this is a meta-test checking that all original behaviors (flags,
// input modes, error messages) are preserved during refactoring.
func TestRefineCLIAdapterPreservesExistingBehavior(t *testing.T) {
	sourceContent, err := os.ReadFile("refine.go")
	if err != nil {
		t.Fatalf("Failed to read refine.go: %v", err)
	}

	source := string(sourceContent)

	// Check that all original features are still present
	requiredFeatures := map[string]string{
		"--agent":              "Agent override flag",
		"--choose-agent":       "Interactive agent picker flag",
		"\"idea-\"":            "Backlog ID prefix detection",
		"\"refined\"":          "Backlog status update",
		"\"Something new...\"": "Blank session picker option",
		"resolveGromitDir":     "Config path resolver usage",
		"resolveSpecsDir":      "Config path resolver usage",
	}

	for pattern, description := range requiredFeatures {
		if !strings.Contains(source, pattern) {
			t.Errorf("Missing feature: %s (%s) - refactoring should preserve all existing functionality",
				description, pattern)
		}
	}
}

// TestRefineHelperFunctionsMovedOrRemoved verifies helper functions are moved to pipeline or removed.
// Expected failure: helper functions like getSpecFiles, containsSpec, extractSpecTitle are still
// in refine.go instead of being moved to pipeline package or helpers.
func TestRefineHelperFunctionsMovedOrRemoved(t *testing.T) {
	sourceContent, err := os.ReadFile("refine.go")
	if err != nil {
		t.Fatalf("Failed to read refine.go: %v", err)
	}

	source := string(sourceContent)

	// These helper functions should be removed from refine.go (moved to pipeline)
	helpersThatShouldMove := []string{
		"func getSpecFiles(",
		"func containsSpec(",
		"func extractSpecTitle(",
	}

	var stillPresent []string
	for _, helper := range helpersThatShouldMove {
		if strings.Contains(source, helper) {
			stillPresent = append(stillPresent, helper)
		}
	}

	if len(stillPresent) > 0 {
		t.Errorf("Helper functions still in refine.go:\n%s\n\nExpected these to be moved to pipeline package or internal helpers",
			strings.Join(stillPresent, "\n"))
	}

	// listMarkdownFiles might stay or move - either is acceptable
	// formatTypeLabel should stay (UI formatting)
}

// TestRefineCLIAdapterImportsPipeline verifies refine.go imports pipeline package.
// Expected failure: refine.go does not import internal/pipeline package yet.
func TestRefineCLIAdapterImportsPipeline(t *testing.T) {
	sourceContent, err := os.ReadFile("refine.go")
	if err != nil {
		t.Fatalf("Failed to read refine.go: %v", err)
	}

	source := string(sourceContent)

	// Should import pipeline package
	hasImport := strings.Contains(source, "\"github.com/danabrams/gromit/internal/pipeline\"") ||
		strings.Contains(source, "github.com/danabrams/gromit/internal/pipeline")

	if !hasImport {
		t.Error("refine.go does not import internal/pipeline package, expected import after refactoring")
	}
}

// TestRefineCLICreatesDepsAndPaths verifies CLI constructs pipeline dependencies.
// Expected failure: runRefine() does not create pipeline.Deps and pipeline.Paths structs yet.
func TestRefineCLICreatesDepsAndPaths(t *testing.T) {
	sourceContent, err := os.ReadFile("refine.go")
	if err != nil {
		t.Fatalf("Failed to read refine.go: %v", err)
	}

	source := string(sourceContent)

	// Should construct Deps struct
	if !strings.Contains(source, "pipeline.Deps{") && !strings.Contains(source, "&pipeline.Deps{") {
		t.Error("refine.go does not construct pipeline.Deps, expected dependency injection setup")
	}

	// Should construct Paths struct
	if !strings.Contains(source, "pipeline.Paths{") && !strings.Contains(source, "&pipeline.Paths{") {
		t.Error("refine.go does not construct pipeline.Paths, expected paths configuration")
	}
}

// TestRefineCLIPickerInputHandling verifies picker logic remains in CLI.
// Expected failure: picker display and input reading might be incorrectly moved to
// pipeline during refactoring, but this is CLI responsibility.
func TestRefineCLIPickerInputHandling(t *testing.T) {
	sourceContent, err := os.ReadFile("refine.go")
	if err != nil {
		t.Fatalf("Failed to read refine.go: %v", err)
	}

	source := string(sourceContent)

	// Picker should remain in CLI
	pickerIndicators := []string{
		"Select an idea to refine",
		"bufio.NewReader",
		"reader.ReadString",
		"fmt.Sscanf",
		"Choice [",
	}

	var missingIndicators []string
	for _, indicator := range pickerIndicators {
		if !strings.Contains(source, indicator) {
			missingIndicators = append(missingIndicators, indicator)
		}
	}

	if len(missingIndicators) == len(pickerIndicators) {
		// All missing - likely moved incorrectly
		t.Error("Picker logic appears to be missing from refine.go - interactive input handling should remain in CLI layer")
	}
}

// TestRefineCLIErrorHandling verifies CLI handles pipeline errors appropriately.
// Expected failure: runRefine() does not check errors from pipeline.Refine() and session.Wait() yet.
func TestRefineCLIErrorHandling(t *testing.T) {
	sourceContent, err := os.ReadFile("refine.go")
	if err != nil {
		t.Fatalf("Failed to read refine.go: %v", err)
	}

	source := string(sourceContent)

	// Should check Refine() error
	hasRefineErrorCheck := strings.Contains(source, "p.Refine(") && strings.Contains(source, "if err")
	if !hasRefineErrorCheck {
		t.Error("refine.go does not check error from pipeline.Refine(), expected error handling")
	}

	// Should check Wait() error
	hasWaitErrorCheck := strings.Contains(source, ".Wait()") && strings.Contains(source, "if err")
	if !hasWaitErrorCheck {
		t.Error("refine.go does not check error from session.Wait(), expected error handling")
	}
}

// TestRefineConfigLoadingStaysInCLI verifies config loading remains in CLI.
// Expected failure: config loading might move to pipeline, but CLI should handle
// config loading and pass resolved values to pipeline.
func TestRefineConfigLoadingStaysInCLI(t *testing.T) {
	sourceContent, err := os.ReadFile("refine.go")
	if err != nil {
		t.Fatalf("Failed to read refine.go: %v", err)
	}

	source := string(sourceContent)

	// Config loading should stay in CLI
	if !strings.Contains(source, "loadConfig()") {
		t.Error("refine.go does not call loadConfig(), config loading should remain in CLI adapter")
	}

	// Path resolution should stay in CLI
	hasPathResolution := strings.Contains(source, "resolveGromitDir") ||
		strings.Contains(source, "resolveSpecsDir")

	if !hasPathResolution {
		t.Error("refine.go does not resolve paths, CLI should resolve config paths before passing to pipeline")
	}
}

// TestRefineFilePathConstruction verifies spec file paths are constructed in post-processing.
// Expected failure: after refactoring, CLI might be constructing spec paths instead of
// getting them from pipeline result.
func TestRefineFilePathConstruction(t *testing.T) {
	sourceContent, err := os.ReadFile("refine.go")
	if err != nil {
		t.Fatalf("Failed to read refine.go: %v", err)
	}

	source := string(sourceContent)

	// After refactoring, should NOT be doing filepath.Join on raw spec names
	// (pipeline should return full paths in result.CreatedSpecs)
	hasManualPathConstruction := strings.Contains(source, "filepath.Join(specsDir,") &&
		strings.Contains(source, "for _, spec := range")

	if hasManualPathConstruction {
		t.Error("refine.go appears to construct spec file paths manually - pipeline should return full paths in result.CreatedSpecs")
	}
}
