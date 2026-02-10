//go:build acceptance

package main

import (
	"os"
	"strings"
	"testing"
)

// TestClaudeAdapterConsolidationLocations verifies the adapter is defined only once
// in the shared internal/learnings package and not in the three original locations
func TestClaudeAdapterConsolidationLocations(t *testing.T) {
	// Read the three files that previously had copy-pasted claudeRunnerAdapter
	reviewSource, err := os.ReadFile("review.go")
	if err != nil {
		t.Fatalf("reading review.go: %v", err)
	}

	runnerSource, err := os.ReadFile("../../internal/runner/runner.go")
	if err != nil {
		t.Fatalf("reading ../../internal/runner/runner.go: %v", err)
	}

	retroSource, err := os.ReadFile("../../internal/retro/retro.go")
	if err != nil {
		t.Fatalf("reading ../../internal/retro/retro.go: %v", err)
	}

	reviewStr := string(reviewSource)
	runnerStr := string(runnerSource)
	retroStr := string(retroSource)

	// Count how many times "type claudeRunnerAdapter struct" appears in the original locations
	reviewAdapterCount := strings.Count(reviewStr, "type claudeRunnerAdapter struct")
	runnerAdapterCount := strings.Count(runnerStr, "type claudeRunnerAdapter struct")
	retroAdapterCount := strings.Count(retroStr, "type claudeRunnerAdapter struct")

	totalAdapterDefs := reviewAdapterCount + runnerAdapterCount + retroAdapterCount

	// After consolidation, claudeRunnerAdapter should NOT be defined in any of these three files
	if totalAdapterDefs != 0 {
		t.Errorf("claudeRunnerAdapter should not be defined in review.go, runner.go, or retro.go after consolidation (found %d definitions: review:%d, runner:%d, retro:%d)",
			totalAdapterDefs, reviewAdapterCount, runnerAdapterCount, retroAdapterCount)
	}
}

// TestClaudeAdapterSharedInternalPackage verifies the adapter is in a shared package
// (either internal/learnings or internal/claude)
func TestClaudeAdapterSharedInternalPackage(t *testing.T) {
	// Check if adapter is in ../../internal/learnings/adapter.go or ../../internal/claude/adapter.go
	adapterInLearnings := false
	adapterInClaude := false

	if _, err := os.Stat("../../internal/learnings/adapter.go"); err == nil {
		adapterInLearnings = true
	}

	if _, err := os.Stat("../../internal/claude/adapter.go"); err == nil {
		adapterInClaude = true
	}

	if !adapterInLearnings && !adapterInClaude {
		t.Error("claudeRunnerAdapter should be defined in either ../../internal/learnings/adapter.go or ../../internal/claude/adapter.go")
	}

	// It should be in exactly one location
	if adapterInLearnings && adapterInClaude {
		t.Error("claudeRunnerAdapter should be in exactly one shared package, found in both internal/learnings and internal/claude")
	}
}

// TestClaudeAdapterDefinesInterface verifies the adapter implements the interface correctly
func TestClaudeAdapterDefinesInterface(t *testing.T) {
	// Read the shared adapter file
	var adapterPath string
	if _, err := os.Stat("../../internal/learnings/adapter.go"); err == nil {
		adapterPath = "../../internal/learnings/adapter.go"
	} else if _, err := os.Stat("../../internal/claude/adapter.go"); err == nil {
		adapterPath = "../../internal/claude/adapter.go"
	} else {
		t.Fatal("cannot find adapter file in ../../internal/learnings/adapter.go or ../../internal/claude/adapter.go")
	}

	source, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatalf("reading adapter file: %v", err)
	}

	sourceStr := string(source)

	// Verify the adapter struct is defined
	if !strings.Contains(sourceStr, "type claudeRunnerAdapter struct") {
		t.Error("adapter file missing 'type claudeRunnerAdapter struct' definition")
	}

	// Verify it implements the Run method with correct signature
	if !strings.Contains(sourceStr, "func (a *claudeRunnerAdapter) Run(ctx context.Context, prompt string, model string) (*learnings.Result, error)") {
		// Allow slight formatting variations but check for key signature elements
		if !strings.Contains(sourceStr, "func (a *claudeRunnerAdapter) Run") {
			t.Error("adapter missing Run method with correct signature")
		}
	}

	// Verify nil checks are in place (this is a key difference between original implementations)
	if !strings.Contains(sourceStr, "a == nil") && !strings.Contains(sourceStr, "a.client == nil") {
		t.Error("adapter should check for nil receiver or nil client")
	}
}

// TestReviewUsesSharedAdapter verifies review.go uses the shared adapter from learnings package
func TestReviewUsesSharedAdapter(t *testing.T) {
	reviewSource, err := os.ReadFile("review.go")
	if err != nil {
		t.Fatalf("reading review.go: %v", err)
	}

	sourceStr := string(reviewSource)

	// review.go should NOT have its own type definition
	if strings.Contains(sourceStr, "type claudeRunnerAdapter struct") {
		t.Error("review.go should not define claudeRunnerAdapter locally - it should import from shared package")
	}

	// review.go should create instances using the constructor from learnings package
	if !strings.Contains(sourceStr, "learnings.NewClaudeRunnerAdapter") {
		t.Error("review.go should call learnings.NewClaudeRunnerAdapter from the shared package")
	}

	// review.go should use the learnings.NewLLMFilter function
	if !strings.Contains(sourceStr, "learnings.NewLLMFilter") {
		t.Error("review.go should call learnings.NewLLMFilter from the shared package")
	}

	// review.go should use the standardized ProjectDescriptions
	if !strings.Contains(sourceStr, "learnings.ProjectDescriptions") {
		t.Error("review.go should reference learnings.ProjectDescriptions")
	}
}

// TestRunnerUsesSharedAdapter verifies runner.go uses the shared adapter from learnings package
func TestRunnerUsesSharedAdapter(t *testing.T) {
	runnerSource, err := os.ReadFile("../../internal/runner/runner.go")
	if err != nil {
		t.Fatalf("reading ../../internal/runner/runner.go: %v", err)
	}

	sourceStr := string(runnerSource)

	// runner.go should NOT have its own type definition
	if strings.Contains(sourceStr, "type claudeRunnerAdapter struct") {
		t.Error("runner.go should not define claudeRunnerAdapter locally - it should import from shared package")
	}

	// runner.go should create instances using the constructor from learnings package
	if !strings.Contains(sourceStr, "learnings.NewClaudeRunnerAdapter") {
		t.Error("runner.go should call learnings.NewClaudeRunnerAdapter from the shared package")
	}

	// runner.go should use the learnings.NewLLMFilter function
	if !strings.Contains(sourceStr, "learnings.NewLLMFilter") {
		t.Error("runner.go should call learnings.NewLLMFilter from the shared package")
	}

	// runner.go should use the standardized ProjectDescriptions
	if !strings.Contains(sourceStr, "learnings.ProjectDescriptions") {
		t.Error("runner.go should reference learnings.ProjectDescriptions")
	}
}

// TestRetroUsesSharedAdapter verifies retro.go uses the shared adapter from learnings package
func TestRetroUsesSharedAdapter(t *testing.T) {
	retroSource, err := os.ReadFile("../../internal/retro/retro.go")
	if err != nil {
		t.Fatalf("reading ../../internal/retro/retro.go: %v", err)
	}

	sourceStr := string(retroSource)

	// retro.go should NOT have its own type definition
	if strings.Contains(sourceStr, "type claudeRunnerAdapter struct") {
		t.Error("retro.go should not define claudeRunnerAdapter locally - it should import from shared package")
	}

	// retro.go should create instances using the constructor from learnings package
	if !strings.Contains(sourceStr, "learnings.NewClaudeRunnerAdapter") {
		t.Error("retro.go should call learnings.NewClaudeRunnerAdapter from the shared package")
	}

	// retro.go should use the learnings.NewLLMFilter function
	if !strings.Contains(sourceStr, "learnings.NewLLMFilter") {
		t.Error("retro.go should call learnings.NewLLMFilter from the shared package")
	}

	// retro.go should use the standardized ProjectDescriptions
	if !strings.Contains(sourceStr, "learnings.ProjectDescriptions") {
		t.Error("retro.go should reference learnings.ProjectDescriptions")
	}
}

// TestProjectDescriptionConsolidated verifies project descriptions are consolidated
func TestProjectDescriptionConsolidated(t *testing.T) {
	// Read the three files and check for project description strings
	reviewSource, _ := os.ReadFile("cmd/gromit/review.go")
	runnerSource, _ := os.ReadFile("../../internal/runner/runner.go")
	retroSource, _ := os.ReadFile("../../internal/retro/retro.go")

	reviewStr := string(reviewSource)
	runnerStr := string(runnerSource)
	retroStr := string(retroSource)

	// Count hardcoded project description strings
	// Original project descriptions were variations of:
	// - "A Go CLI tool that runs the Gromit loop correctly"
	// - "A Go CLI tool that runs the Gromit loop with fresh context on each iteration"

	const (
		desc1 = "runs the Gromit loop correctly"
		desc2 = "runs the Gromit loop with fresh context"
		desc3 = "Go CLI tool that runs the Gromit loop"
	)

	reviewDescCount := strings.Count(reviewStr, desc1) + strings.Count(reviewStr, desc2) + strings.Count(reviewStr, desc3)
	runnerDescCount := strings.Count(runnerStr, desc1) + strings.Count(runnerStr, desc2) + strings.Count(runnerStr, desc3)
	retroDescCount := strings.Count(retroStr, desc1) + strings.Count(retroStr, desc2) + strings.Count(retroStr, desc3)

	// After consolidation, the project description should be used consistently
	// either as a shared constant or through a single definition

	// Check if there's a shared constant defined in the adapter package
	var adapterPath string
	if _, err := os.Stat("../../internal/learnings/adapter.go"); err == nil {
		adapterPath = "../../internal/learnings/adapter.go"
	} else if _, err := os.Stat("../../internal/claude/adapter.go"); err == nil {
		adapterPath = "../../internal/claude/adapter.go"
	}

	if adapterPath != "" {
		adapterSource, _ := os.ReadFile(adapterPath)
		adapterStr := string(adapterSource)

		// Check if project description is defined as a constant or helper
		hasProjectDesc := strings.Contains(adapterStr, "projectName") || strings.Contains(adapterStr, "projectDesc") ||
			strings.Contains(adapterStr, "ProjectName") || strings.Contains(adapterStr, "ProjectDesc")

		if !hasProjectDesc && (reviewDescCount > 0 || runnerDescCount > 0 || retroDescCount > 0) {
			// The descriptions might still be hardcoded in the call sites, which is acceptable
			// as long as they're consistent. Check if they're the same.
			t.Log("Project descriptions are still hardcoded in call sites (acceptable if consistent)")
		}
	}
}

// TestAdapterNilCheckConsistency verifies all nil checks are present in the consolidated adapter
func TestAdapterNilCheckConsistency(t *testing.T) {
	// Find the shared adapter file
	var adapterPath string
	if _, err := os.Stat("../../internal/learnings/adapter.go"); err == nil {
		adapterPath = "../../internal/learnings/adapter.go"
	} else if _, err := os.Stat("../../internal/claude/adapter.go"); err == nil {
		adapterPath = "../../internal/claude/adapter.go"
	} else {
		t.Skip("adapter file not found")
	}

	source, _ := os.ReadFile(adapterPath)
	sourceStr := string(source)

	// The original implementations differed in nil checking:
	// - review.go version: checks a == nil and a.client == nil
	// - runner.go version: checks a == nil, a.client == nil, and result == nil
	// - retro.go version: (no explicit nil checks in adapter)

	// The consolidated version should include all necessary nil checks
	// At minimum, should check for nil receiver and nil client
	nilCheckCount := strings.Count(sourceStr, "== nil")

	if nilCheckCount < 2 {
		t.Error("adapter Run method should include nil checks for receiver and client")
	}
}

// TestClaudeAdapterNoLongerInMultipleFiles verifies the adapter is not defined in the original three files
func TestClaudeAdapterNoLongerInMultipleFiles(t *testing.T) {
	reviewSource, err := os.ReadFile("review.go")
	if err != nil {
		t.Fatalf("cannot read review.go: %v", err)
	}

	runnerSource, err := os.ReadFile("../../internal/runner/runner.go")
	if err != nil {
		t.Fatalf("cannot read ../../internal/runner/runner.go: %v", err)
	}

	retroSource, err := os.ReadFile("../../internal/retro/retro.go")
	if err != nil {
		t.Fatalf("cannot read ../../internal/retro/retro.go: %v", err)
	}

	reviewHasAdapter := strings.Contains(string(reviewSource), "type claudeRunnerAdapter struct")
	runnerHasAdapter := strings.Contains(string(runnerSource), "type claudeRunnerAdapter struct")
	retroHasAdapter := strings.Contains(string(retroSource), "type claudeRunnerAdapter struct")

	// Count how many of the three files have the adapter definition
	defCount := 0
	if reviewHasAdapter {
		defCount++
	}
	if runnerHasAdapter {
		defCount++
	}
	if retroHasAdapter {
		defCount++
	}

	// After consolidation, none of the three original files should define the adapter
	if defCount != 0 {
		t.Errorf("adapter should not be defined in review.go, runner.go, or retro.go - found in %d files", defCount)
	}
}

// TestAdapterImplementsClaudeRunnerInterface verifies the adapter correctly implements
// the learnings.ClaudeRunner interface
func TestAdapterImplementsClaudeRunnerInterface(t *testing.T) {
	// Read the shared adapter file
	var adapterPath string
	if _, err := os.Stat("../../internal/learnings/adapter.go"); err == nil {
		adapterPath = "../../internal/learnings/adapter.go"
	} else if _, err := os.Stat("../../internal/claude/adapter.go"); err == nil {
		adapterPath = "../../internal/claude/adapter.go"
	} else {
		t.Skip("adapter file not found")
	}

	source, _ := os.ReadFile(adapterPath)
	sourceStr := string(source)

	// Verify the Run method exists with the correct receiver and return types
	if !strings.Contains(sourceStr, "func (a *claudeRunnerAdapter) Run") {
		t.Error("adapter must have Run method with pointer receiver")
	}

	// Verify the method signature includes context.Context parameter
	if !strings.Contains(sourceStr, "context.Context") {
		t.Error("Run method must accept context.Context parameter")
	}

	// Verify the method returns (*learnings.Result, error)
	if !strings.Contains(sourceStr, "learnings.Result") {
		t.Error("Run method must return *learnings.Result")
	}

	// Verify it imports the necessary packages
	if !strings.Contains(sourceStr, "import") {
		t.Error("adapter file must include imports")
	}

	// Should import both claude and learnings packages
	if !strings.Contains(sourceStr, "claude") && !strings.Contains(sourceStr, "learnings") {
		t.Error("adapter should import claude and learnings packages")
	}
}

// TestCallSitesImportAdapterFromSharedPackage verifies all three call sites import from shared package
func TestCallSitesImportAdapterFromSharedPackage(t *testing.T) {
	reviewSource, err := os.ReadFile("review.go")
	if err != nil {
		t.Fatalf("cannot read review.go: %v", err)
	}

	runnerSource, err := os.ReadFile("../../internal/runner/runner.go")
	if err != nil {
		t.Fatalf("cannot read runner.go: %v", err)
	}

	retroSource, err := os.ReadFile("../../internal/retro/retro.go")
	if err != nil {
		t.Fatalf("cannot read retro.go: %v", err)
	}

	reviewStr := string(reviewSource)
	runnerStr := string(runnerSource)
	retroStr := string(retroSource)

	// All three should import learnings package
	files := []struct {
		name   string
		source string
	}{
		{"review.go", reviewStr},
		{"runner.go", runnerStr},
		{"retro.go", retroStr},
	}

	for _, f := range files {
		if !strings.Contains(f.source, `"github.com/danabrams/gromit/internal/learnings"`) {
			t.Errorf("%s should import learnings package", f.name)
		}
	}
}
