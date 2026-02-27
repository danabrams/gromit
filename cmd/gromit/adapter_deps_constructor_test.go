package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline" // Used in deps type inference and in t.Errorf message
)

// TestNewPipelineDeps_ConstructsCompleteConfig verifies that a dependency injection function
// can construct a complete pipeline.Deps struct with all required adapters.
func TestNewPipelineDeps_ConstructsCompleteConfig(t *testing.T) {
	t.Parallel()

	// Create temporary directories for testing
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("creating gromit dir: %v", err)
	}

	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary: "claude",
		},
	}

	// Call the constructor function
	deps, err := NewPipelineDeps(cfg, gromitDir)
	if err != nil {
		t.Fatalf("NewPipelineDeps failed: %v", err)
	}

	// Verify all fields are non-nil
	if deps == nil {
		var _ *pipeline.Deps // Ensure pipeline package is imported
		t.Fatal("NewPipelineDeps returned nil deps")
	}

	// Check each required field is present and non-nil
	tests := []struct {
		name string
		dep  interface{}
	}{
		{"AgentResolver", deps.AgentResolver},
		{"LLMClient", deps.LLMClient},
		{"ReviewInvoker", deps.ReviewInvoker},
		{"TrackerClient", deps.TrackerClient},
		{"BacklogClient", deps.BacklogClient},
		{"BacklogWriter", deps.BacklogWriter},
		{"RefineRenderer", deps.RefineRenderer},
		{"PlanRenderer", deps.PlanRenderer},
		{"DecomposeRenderer", deps.DecomposeRenderer},
		{"ReviewRenderer", deps.ReviewRenderer},
		{"ExploreRenderer", deps.ExploreRenderer},
		{"LearningsManager", deps.LearningsManager},
		{"StateManager", deps.StateManager},
		{"LogWriter", deps.LogWriter},
	}

	for _, tt := range tests {
		if tt.dep == nil {
			t.Errorf("pipeline.Deps.%s is nil", tt.name)
		}
	}
}
