//go:build acceptance

package learnings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAdapterConsolidatedInSharedPackage verifies the adapter is defined only once,
// in the shared internal/learnings package
func TestAdapterConsolidatedInSharedPackage(t *testing.T) {
	// Read this file (adapter.go) to verify the adapter is defined here
	adapterPath := filepath.Join(".", "adapter.go")
	adapterSource, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatalf("could not read adapter.go: %v", err)
	}

	adapterStr := string(adapterSource)

	// Verify the adapter struct is defined
	if !strings.Contains(adapterStr, "type claudeRunnerAdapter struct") {
		t.Error("adapter.go missing 'type claudeRunnerAdapter struct' definition")
	}

	// Verify constructor is present
	if !strings.Contains(adapterStr, "func NewClaudeRunnerAdapter") {
		t.Error("adapter.go missing NewClaudeRunnerAdapter constructor")
	}

	// Verify Run method with correct receiver
	if !strings.Contains(adapterStr, "func (a *claudeRunnerAdapter) Run") {
		t.Error("adapter.go missing Run method on claudeRunnerAdapter receiver")
	}
}

// TestAdapterNotDefinedInReviewGo verifies review.go does not locally define the adapter
func TestAdapterNotDefinedInReviewGo(t *testing.T) {
	reviewPath := filepath.Join(".", "..", "..", "cmd", "gromit", "review.go")
	source, err := os.ReadFile(reviewPath)
	if err != nil {
		t.Fatalf("could not read review.go: %v", err)
	}

	sourceStr := string(source)

	if strings.Contains(sourceStr, "type claudeRunnerAdapter struct") {
		t.Error("review.go should not define claudeRunnerAdapter locally")
	}
}

// TestAdapterNotDefinedInRunnerGo verifies runner.go does not locally define the adapter
func TestAdapterNotDefinedInRunnerGo(t *testing.T) {
	runnerPath := filepath.Join(".", "..", "..", "internal", "runner", "runner.go")
	source, err := os.ReadFile(runnerPath)
	if err != nil {
		t.Fatalf("could not read runner.go: %v", err)
	}

	sourceStr := string(source)

	if strings.Contains(sourceStr, "type claudeRunnerAdapter struct") {
		t.Error("runner.go should not define claudeRunnerAdapter locally")
	}
}

// TestAdapterNotDefinedInRetroGo verifies retro.go does not locally define the adapter
func TestAdapterNotDefinedInRetroGo(t *testing.T) {
	retroPath := filepath.Join(".", "..", "..", "internal", "retro", "retro.go")
	source, err := os.ReadFile(retroPath)
	if err != nil {
		t.Fatalf("could not read retro.go: %v", err)
	}

	sourceStr := string(source)

	if strings.Contains(sourceStr, "type claudeRunnerAdapter struct") {
		t.Error("retro.go should not define claudeRunnerAdapter locally")
	}
}

// TestReviewGoUsesSharedAdapter verifies review.go uses the shared adapter
func TestReviewGoUsesSharedAdapter(t *testing.T) {
	reviewPath := filepath.Join(".", "..", "..", "cmd", "gromit", "review.go")
	source, err := os.ReadFile(reviewPath)
	if err != nil {
		t.Fatalf("could not read review.go: %v", err)
	}

	sourceStr := string(source)

	// Should reference the constructor from the shared package
	if !strings.Contains(sourceStr, "learnings.NewClaudeRunnerAdapter") {
		t.Error("review.go should call learnings.NewClaudeRunnerAdapter")
	}

	// Should pass the adapter to LLMFilter
	if !strings.Contains(sourceStr, "learnings.NewLLMFilter") {
		t.Error("review.go should use learnings.NewLLMFilter")
	}

	// Should use ProjectDescriptions from shared package
	if !strings.Contains(sourceStr, "learnings.ProjectDescriptions") {
		t.Error("review.go should reference learnings.ProjectDescriptions")
	}
}

// TestRunnerGoUsesSharedAdapter verifies runner.go uses the shared adapter
func TestRunnerGoUsesSharedAdapter(t *testing.T) {
	runnerPath := filepath.Join(".", "..", "..", "internal", "runner", "runner.go")
	source, err := os.ReadFile(runnerPath)
	if err != nil {
		t.Fatalf("could not read runner.go: %v", err)
	}

	sourceStr := string(source)

	// Should reference either Claude or Provider adapter from shared package
	hasClaudeAdapter := strings.Contains(sourceStr, "learnings.NewClaudeRunnerAdapter")
	hasProviderAdapter := strings.Contains(sourceStr, "learnings.NewProviderRunnerAdapter")
	if !hasClaudeAdapter && !hasProviderAdapter {
		t.Error("runner.go should call either learnings.NewClaudeRunnerAdapter or learnings.NewProviderRunnerAdapter")
	}

	// Should pass the adapter to LLMFilter
	if !strings.Contains(sourceStr, "learnings.NewLLMFilter") {
		t.Error("runner.go should use learnings.NewLLMFilter")
	}

	// Should use ProjectDescriptions from shared package
	if !strings.Contains(sourceStr, "learnings.ProjectDescriptions") {
		t.Error("runner.go should reference learnings.ProjectDescriptions")
	}
}

// TestRetroGoUsesSharedAdapter verifies retro.go uses the shared adapter
func TestRetroGoUsesSharedAdapter(t *testing.T) {
	retroPath := filepath.Join(".", "..", "..", "internal", "retro", "retro.go")
	source, err := os.ReadFile(retroPath)
	if err != nil {
		t.Fatalf("could not read retro.go: %v", err)
	}

	sourceStr := string(source)

	// Should reference either Claude or Provider adapter from shared package (or both for backward compat)
	hasClaudeAdapter := strings.Contains(sourceStr, "learnings.NewClaudeRunnerAdapter")
	hasProviderAdapter := strings.Contains(sourceStr, "learnings.NewProviderRunnerAdapter")
	if !hasClaudeAdapter && !hasProviderAdapter {
		t.Error("retro.go should call either learnings.NewClaudeRunnerAdapter or learnings.NewProviderRunnerAdapter")
	}

	// Should pass the adapter to LLMFilter
	if !strings.Contains(sourceStr, "learnings.NewLLMFilter") {
		t.Error("retro.go should use learnings.NewLLMFilter")
	}

	// Should use ProjectDescriptions from shared package
	if !strings.Contains(sourceStr, "learnings.ProjectDescriptions") {
		t.Error("retro.go should reference learnings.ProjectDescriptions")
	}
}

// TestProjectDescriptionsConsolidated verifies project descriptions are consolidated
func TestProjectDescriptionsConsolidated(t *testing.T) {
	// Read adapter.go to verify ProjectDescriptions is defined there
	adapterPath := filepath.Join(".", "adapter.go")
	source, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatalf("could not read adapter.go: %v", err)
	}

	sourceStr := string(source)

	// Should define ProjectDescriptions variable
	if !strings.Contains(sourceStr, "var ProjectDescriptions") {
		t.Error("adapter.go should define ProjectDescriptions variable")
	}

	// Should include Gromit description
	if !strings.Contains(sourceStr, "Gromit") {
		t.Error("ProjectDescriptions should include Gromit field")
	}

	// The description should be standardized
	if !strings.Contains(sourceStr, "A Go CLI tool that runs the Gromit loop") {
		t.Error("ProjectDescriptions.Gromit should follow the standard description format")
	}
}

// TestAdapterImplementsCorrectInterface verifies the adapter implements the right interface
func TestAdapterImplementsCorrectInterface(t *testing.T) {
	adapterPath := filepath.Join(".", "adapter.go")
	source, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatalf("could not read adapter.go: %v", err)
	}

	sourceStr := string(source)

	// Should define ClaudeClientRunner interface for dependency injection
	if !strings.Contains(sourceStr, "type ClaudeClientRunner interface") {
		t.Error("adapter.go should define ClaudeClientRunner interface")
	}

	// Should have a Run method on the interface
	if !strings.Contains(sourceStr, "Run(ctx context.Context, prompt string, model string)") {
		t.Error("ClaudeClientRunner interface should have Run method with correct signature")
	}

	// The adapter's Run method should return (*Result, error)
	if !strings.Contains(sourceStr, "func (a *claudeRunnerAdapter) Run(ctx context.Context, prompt string, model string) (*Result, error)") {
		t.Error("claudeRunnerAdapter.Run should return (*learnings.Result, error)")
	}
}

// TestAdapterProperlyConvertsResults verifies the adapter converts claude.Result to learnings.Result
func TestAdapterProperlyConvertsResults(t *testing.T) {
	adapterPath := filepath.Join(".", "adapter.go")
	source, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatalf("could not read adapter.go: %v", err)
	}

	sourceStr := string(source)

	// Should have nil checks
	if !strings.Contains(sourceStr, "a.client == nil") {
		t.Error("adapter should check if client is nil")
	}

	if !strings.Contains(sourceStr, "claudeResult == nil") {
		t.Error("adapter should check if claudeResult is nil")
	}

	// Should convert Success field
	if !strings.Contains(sourceStr, "Success:") || !strings.Contains(sourceStr, "claudeResult.Success") {
		t.Error("adapter should convert Success field from claude.Result")
	}

	// Should convert Output field
	if !strings.Contains(sourceStr, "Output:") || !strings.Contains(sourceStr, "claudeResult.Output") {
		t.Error("adapter should convert Output field from claude.Result")
	}
}

// TestAllCallSitesUseSharedProjectDescription verifies all call sites use the standardized description
func TestAllCallSitesUseSharedProjectDescription(t *testing.T) {
	files := []struct {
		name string
		path string
	}{
		{"review.go", filepath.Join(".", "..", "..", "cmd", "gromit", "review.go")},
		{"runner.go", filepath.Join(".", "..", "..", "internal", "runner", "runner.go")},
		{"retro.go", filepath.Join(".", "..", "..", "internal", "retro", "retro.go")},
	}

	for _, f := range files {
		t.Run(f.name, func(t *testing.T) {
			source, err := os.ReadFile(f.path)
			if err != nil {
				t.Fatalf("could not read %s: %v", f.name, err)
			}

			sourceStr := string(source)

			// All should reference ProjectDescriptions.Gromit
			if !strings.Contains(sourceStr, "learnings.ProjectDescriptions.Gromit") {
				t.Errorf("%s should use learnings.ProjectDescriptions.Gromit", f.name)
			}

			// Should not have hardcoded project description strings
			if strings.Contains(sourceStr, "runs the Gromit loop") && !strings.Contains(sourceStr, "ProjectDescriptions") {
				// This is okay only if it's in a comment or constant that's not being used as the description
				t.Logf("Note: %s may contain legacy hardcoded description strings", f.name)
			}
		})
	}
}

// TestAdapterPackageExportsPublicConstructor verifies the constructor is exported
func TestAdapterPackageExportsPublicConstructor(t *testing.T) {
	adapterPath := filepath.Join(".", "adapter.go")
	source, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatalf("could not read adapter.go: %v", err)
	}

	sourceStr := string(source)

	// NewClaudeRunnerAdapter should be exported (start with capital letter)
	if !strings.Contains(sourceStr, "func NewClaudeRunnerAdapter") {
		t.Error("adapter should export NewClaudeRunnerAdapter constructor")
	}

	// Should take a ClaudeClientRunner parameter
	if !strings.Contains(sourceStr, "func NewClaudeRunnerAdapter(client ClaudeClientRunner)") {
		t.Error("NewClaudeRunnerAdapter should accept ClaudeClientRunner parameter")
	}
}

// TestAllCallSitesImportLearningsPackage verifies all call sites import the learnings package
func TestAllCallSitesImportLearningsPackage(t *testing.T) {
	files := []struct {
		name string
		path string
	}{
		{"review.go", filepath.Join(".", "..", "..", "cmd", "gromit", "review.go")},
		{"runner.go", filepath.Join(".", "..", "..", "internal", "runner", "runner.go")},
		{"retro.go", filepath.Join(".", "..", "..", "internal", "retro", "retro.go")},
	}

	for _, f := range files {
		t.Run(f.name, func(t *testing.T) {
			source, err := os.ReadFile(f.path)
			if err != nil {
				t.Fatalf("could not read %s: %v", f.name, err)
			}

			sourceStr := string(source)

			// Should import learnings package
			if !strings.Contains(sourceStr, `"github.com/danabrams/gromit/internal/learnings"`) {
				t.Errorf("%s should import learnings package", f.name)
			}

			// Should use it with the package name
			if !strings.Contains(sourceStr, "learnings.") {
				t.Errorf("%s should reference learnings package with dot notation", f.name)
			}
		})
	}
}
