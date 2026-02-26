package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/pipeline"
)

// TestAdapterCompileTimeChecks verifies that adapter types implement their intended interfaces.
// This ensures all adapters have compile-time interface compliance checks via var _ declarations.
// If an adapter doesn't implement its interface, the compiler will error here first.
func TestAdapterCompileTimeChecks(t *testing.T) {
	t.Parallel(
	// These assignments verify the adapters implement their interfaces.
	// The var _ InterfaceType = (*ConcreteType)(nil) pattern in the source provides
	// clearer compile-time verification and improved IDE support.
	)

	// Adapters in cmd/gromit/adapters.go
	var _ pipeline.BeadClient = (*beadClientAdapter)(nil)

	// Adapters in cmd/gromit/cli_adapters.go
	var _ pipeline.ReviewRenderer = (*cliPromptRenderer)(nil)
	var _ pipeline.ExploreRenderer = (*explorePromptRenderer)(nil)

	// Adapters in cmd/gromit/review.go
	var _ learnings.ClaudeRunner = (*pipelineLearningsRunnerAdapter)(nil)

	t.Log("All adapter compile-time checks verified")
}
