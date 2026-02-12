//go:build acceptance

package learnings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunnerUsesProviderAdapter verifies runner.go now uses Provider-based adapter
func TestRunnerUsesProviderAdapter(t *testing.T) {
	runnerPath := filepath.Join(".", "..", "..", "internal", "runner", "runner.go")
	source, err := os.ReadFile(runnerPath)
	if err != nil {
		t.Fatalf("could not read runner.go: %v", err)
	}

	sourceStr := string(source)

	// Should reference the Provider adapter constructor
	if !strings.Contains(sourceStr, "learnings.NewProviderRunnerAdapter") {
		t.Error("runner.go should call learnings.NewProviderRunnerAdapter (migrated to Provider)")
	}

	// Should still use LLMFilter
	if !strings.Contains(sourceStr, "learnings.NewLLMFilter") {
		t.Error("runner.go should use learnings.NewLLMFilter")
	}

	// Should use ProjectDescriptions from shared package
	if !strings.Contains(sourceStr, "learnings.ProjectDescriptions") {
		t.Error("runner.go should reference learnings.ProjectDescriptions")
	}
}

// TestRetroSupportsProviderAdapter verifies retro.go now supports Provider-based adapter
func TestRetroSupportsProviderAdapter(t *testing.T) {
	retroPath := filepath.Join(".", "..", "..", "internal", "retro", "retro.go")
	source, err := os.ReadFile(retroPath)
	if err != nil {
		t.Fatalf("could not read retro.go: %v", err)
	}

	sourceStr := string(source)

	// Should reference Provider adapter constructor
	if !strings.Contains(sourceStr, "learnings.NewProviderRunnerAdapter") {
		t.Error("retro.go should support learnings.NewProviderRunnerAdapter")
	}

	// Should maintain backward compatibility with Claude adapter
	if !strings.Contains(sourceStr, "learnings.NewClaudeRunnerAdapter") {
		t.Error("retro.go should maintain backward compatibility with NewClaudeRunnerAdapter")
	}

	// Should use LLMFilter
	if !strings.Contains(sourceStr, "learnings.NewLLMFilter") {
		t.Error("retro.go should use learnings.NewLLMFilter")
	}

	// Should use ProjectDescriptions from shared package
	if !strings.Contains(sourceStr, "learnings.ProjectDescriptions") {
		t.Error("retro.go should reference learnings.ProjectDescriptions")
	}
}
