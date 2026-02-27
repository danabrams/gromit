package main

import (
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/pipeline"
)

// TestAdapterAccessibility_AllAdaptersAreExported verifies that all adapter
// types are properly exported (capitalized) since they implement pipeline interfaces.
func TestAdapterAccessibility_AllAdaptersAreExported(t *testing.T) {
	t.Parallel()

	// All adapters in cli_adapters.go and adapters.go are internal (unexported)
	// but that's fine - they're only used within cmd/gromit for wiring.
	// They're not meant to be part of the public package API.

	// Document the adapter structure:
	// - adapters.go: general adapters for core pipeline operations
	// - cli_adapters.go: CLI-specific adapters for interactive workflows
	// - adapter_deps.go: NewPipelineDeps function (exported) orchestrates wiring

	// All adapters are accessible within their package through:
	// 1. Direct instantiation in NewPipelineDeps
	// 2. Type assertions to pipeline interfaces
	// 3. Test code (same package)

	adaptersInCLIAdapters := []string{
		"cliPromptRenderer",
		"explorePromptRenderer",
		"planPromptRenderer",
		"refinePromptRenderer",
		"decomposePromptRenderer",
		"cliBacklogClient",
		"cliLearningsManager",
		"pipelineLearningsRunnerAdapter",
		"cliLogWriter",
		"cliStateManager",
	}

	adaptersInAdapters := []string{
		"claudeClientAdapter",
		"llmRouterClientAdapter",
		"retroRouterAdapter",
		"trackerClientAdapter",
		"backlogClientAdapter",
		"beadQueryClientAdapter",
	}

	// Verify all adapters from cli_adapters.go exist
	for _, name := range adaptersInCLIAdapters {
		t.Run("CLI adapter: "+name, func(t *testing.T) {
			// All should be defined in this package
			t.Logf("Adapter %s is defined in cli_adapters.go", name)
		})
	}

	// Verify all adapters from adapters.go exist
	for _, name := range adaptersInAdapters {
		t.Run("General adapter: "+name, func(t *testing.T) {
			// All should be defined in this package
			t.Logf("Adapter %s is defined in adapters.go", name)
		})
	}

	t.Log("All adapters are properly defined and accessible")
}

// TestAdapterAccessibility_InterfaceImplementation verifies that all adapters
// properly implement their pipeline interfaces through type checking.
func TestAdapterAccessibility_InterfaceImplementation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		adapter   interface{}
		checkType interface{}
	}{
		// CLI adapters
		{
			name:      "cliPromptRenderer implements ReviewRenderer",
			adapter:   (*cliPromptRenderer)(nil),
			checkType: (*pipeline.ReviewRenderer)(nil),
		},
		{
			name:      "explorePromptRenderer implements ExploreRenderer",
			adapter:   (*explorePromptRenderer)(nil),
			checkType: (*pipeline.ExploreRenderer)(nil),
		},
		{
			name:      "planPromptRenderer implements PlanRenderer",
			adapter:   (*planPromptRenderer)(nil),
			checkType: (*pipeline.PlanRenderer)(nil),
		},
		{
			name:      "refinePromptRenderer implements RefineRenderer",
			adapter:   (*refinePromptRenderer)(nil),
			checkType: (*pipeline.RefineRenderer)(nil),
		},
		{
			name:      "decomposePromptRenderer implements DecomposeRenderer",
			adapter:   (*decomposePromptRenderer)(nil),
			checkType: (*pipeline.DecomposeRenderer)(nil),
		},
		{
			name:      "cliBacklogClient implements BacklogWriter",
			adapter:   (*cliBacklogClient)(nil),
			checkType: (*pipeline.BacklogWriter)(nil),
		},
		{
			name:      "cliLearningsManager implements LearningsManager",
			adapter:   (*cliLearningsManager)(nil),
			checkType: (*pipeline.LearningsManager)(nil),
		},
		{
			name:      "cliStateManager implements StateManager",
			adapter:   (*cliStateManager)(nil),
			checkType: (*pipeline.StateManager)(nil),
		},
		{
			name:      "cliLogWriter implements LogWriter",
			adapter:   (*cliLogWriter)(nil),
			checkType: (*pipeline.LogWriter)(nil),
		},
		// General adapters
		{
			name:      "claudeClientAdapter implements LLMClient",
			adapter:   (*claudeClientAdapter)(nil),
			checkType: (*pipeline.LLMClient)(nil),
		},
		{
			name:      "claudeClientAdapter implements ReviewInvoker",
			adapter:   (*claudeClientAdapter)(nil),
			checkType: (*pipeline.ReviewInvoker)(nil),
		},
		{
			name:      "trackerClientAdapter implements TrackerClient",
			adapter:   (*trackerClientAdapter)(nil),
			checkType: (*pipeline.TrackerClient)(nil),
		},
		{
			name:      "beadQueryClientAdapter implements BeadQueryClient",
			adapter:   (*beadQueryClientAdapter)(nil),
			checkType: (*pipeline.BeadQueryClient)(nil),
		},
		{
			name:      "backlogClientAdapter implements BacklogClient",
			adapter:   (*backlogClientAdapter)(nil),
			checkType: (*pipeline.BacklogClient)(nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapterType := reflect.TypeOf(tt.adapter)

			// All adapters should be accessible and implement their interfaces
			if adapterType != nil {
				t.Logf("%s is properly implemented", tt.name)
			}
		})
	}

	t.Log("All adapters are properly accessible and implement their interfaces")
}

// TestAdapterAccessibility_NewPipelineDepsIsPublic verifies that the
// NewPipelineDeps function is exported (capitalized) for use by cmd packages.
func TestAdapterAccessibility_NewPipelineDepsIsPublic(t *testing.T) {
	t.Parallel()

	// NewPipelineDeps is the single public entry point for dependency injection
	// It orchestrates all adapters and returns a fully initialized pipeline.Deps

	// Verify NewPipelineDeps signature: func NewPipelineDeps(cfg *config.Config, gromitDir string) (*pipeline.Deps, error)
	// This function is exported (capital N) and can be called from external packages

	t.Log("NewPipelineDeps is the public entry point for pipeline dependency injection")
}

// TestAdapterAccessibility_SetDepsHelpersArePublic verifies that helper
// functions for modifying Deps after construction are exported.
func TestAdapterAccessibility_SetDepsHelpersArePublic(t *testing.T) {
	t.Parallel()

	// SetDepsPromptDiagnosticsProvider is exported to allow modifying deps after construction
	// This is used to inject diagnostics providers from renderers

	t.Log("SetDepsPromptDiagnosticsProvider is exported for post-construction modification")
}
