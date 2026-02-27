package main

import (
	"os"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

// TestNewPipelineDeps_WiresAllRequiredAdapters verifies that NewPipelineDeps
// properly instantiates and wires all required adapter implementations.
// This test validates that pipeline.Deps is fully initialized with all non-nil implementations.
func TestNewPipelineDeps_WiresAllRequiredAdapters(t *testing.T) {
	t.Parallel()

	gromitDir := t.TempDir()
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary: "claude",
			Flags:  []string{},
		},
	}

	deps, err := NewPipelineDeps(cfg, gromitDir)
	if err != nil {
		t.Fatalf("NewPipelineDeps failed: %v", err)
	}

	if deps == nil {
		t.Fatal("NewPipelineDeps returned nil deps")
	}

	// Verify all required interfaces are implemented and non-nil
	requiredAdapters := map[string]interface{}{
		"AgentResolver":     deps.AgentResolver,
		"LLMClient":         deps.LLMClient,
		"ReviewInvoker":     deps.ReviewInvoker,
		"TrackerClient":     deps.TrackerClient,
		"BacklogClient":     deps.BacklogClient,
		"BacklogWriter":     deps.BacklogWriter,
		"RefineRenderer":    deps.RefineRenderer,
		"PlanRenderer":      deps.PlanRenderer,
		"DecomposeRenderer": deps.DecomposeRenderer,
		"ReviewRenderer":    deps.ReviewRenderer,
		"ExploreRenderer":   deps.ExploreRenderer,
		"LearningsManager":  deps.LearningsManager,
		"StateManager":      deps.StateManager,
		"LogWriter":         deps.LogWriter,
	}

	for name, adapter := range requiredAdapters {
		if adapter == nil {
			t.Errorf("deps.%s is nil - adapter not wired", name)
		}
	}
}

// TestNewPipelineDeps_AdaptersMatchInterfaceSignatures verifies that the wired
// adapters actually implement their claimed interfaces with proper method signatures.
// This is a compile-time check demonstrated at runtime.
func TestNewPipelineDeps_AdaptersMatchInterfaceSignatures(t *testing.T) {
	t.Parallel()

	gromitDir := t.TempDir()
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary: "claude",
			Flags:  []string{},
		},
	}

	deps, err := NewPipelineDeps(cfg, gromitDir)
	if err != nil {
		t.Fatalf("NewPipelineDeps failed: %v", err)
	}

	// Verify adapters implement correct types
	var _ pipeline.AgentResolver = deps.AgentResolver
	var _ pipeline.LLMClient = deps.LLMClient
	var _ pipeline.ReviewInvoker = deps.ReviewInvoker
	var _ pipeline.TrackerClient = deps.TrackerClient
	var _ pipeline.BacklogClient = deps.BacklogClient
	var _ pipeline.BacklogWriter = deps.BacklogWriter
	var _ pipeline.RefineRenderer = deps.RefineRenderer
	var _ pipeline.PlanRenderer = deps.PlanRenderer
	var _ pipeline.DecomposeRenderer = deps.DecomposeRenderer
	var _ pipeline.ReviewRenderer = deps.ReviewRenderer
	var _ pipeline.ExploreRenderer = deps.ExploreRenderer
	var _ pipeline.LearningsManager = deps.LearningsManager
	var _ pipeline.StateManager = deps.StateManager
	var _ pipeline.LogWriter = deps.LogWriter

	t.Log("All adapters match their interface signatures")
}

// TestNewPipelineDeps_ConstructorDocuments that it's the single dependency injection point
func TestNewPipelineDeps_IsDocumentedAsSingleDIPoint(t *testing.T) {
	t.Parallel()

	// This test documents the architectural decision that NewPipelineDeps
	// is the single point where all adapter dependencies are constructed.
	// This ensures consistent wiring across all command workflows.

	// Read the source file to verify documentation
	sourceFile := "adapter_deps.go"
	content, err := os.ReadFile(sourceFile)
	if err != nil {
		t.Skipf("Could not read %s: %v", sourceFile, err)
	}

	sourceStr := string(content)

	// Verify that the comment describes it as the single DI point
	if !strings.Contains(sourceStr, "single dependency injection point") {
		t.Errorf("NewPipelineDeps should be documented as the single DI point")
	}
}

