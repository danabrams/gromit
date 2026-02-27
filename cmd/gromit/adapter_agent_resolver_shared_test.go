package main

import (
	"os"
	"strings"
	"testing"
)

func TestSharedAgentResolver_UsedInExplore(t *testing.T) {
	t.Parallel()

	exploreSource, err := os.ReadFile("explore.go")
	if err != nil {
		t.Fatalf("failed to read explore.go: %v", err)
	}
	sourceStr := string(exploreSource)

	// explore.go should NOT have its own agent resolver implementation
	if strings.Contains(sourceStr, "type exploreAgentResolver") {
		t.Fatal("explore.go should not have its own exploreAgentResolver - use shared agent.NewResolver")
	}

	// explore.go SHOULD use NewPipelineDeps (which constructs the shared agent.NewResolver)
	// OR use agent.NewResolver(cfg) directly
	usesNewPipelineDeps := strings.Contains(sourceStr, "NewPipelineDeps(")
	usesDirectResolver := strings.Contains(sourceStr, "agent.NewResolver(cfg)")
	if !usesNewPipelineDeps && !usesDirectResolver {
		t.Fatal("explore.go should use either NewPipelineDeps() or agent.NewResolver(cfg)")
	}
}

func TestSharedAgentResolver_UsedInRefine(t *testing.T) {
	t.Parallel()

	refineSource, err := os.ReadFile("refine.go")
	if err != nil {
		t.Fatalf("failed to read refine.go: %v", err)
	}
	sourceStr := string(refineSource)

	// refine.go should NOT have its own agent resolver implementation
	if strings.Contains(sourceStr, "type agentResolverAdapter") {
		t.Fatal("refine.go should not have its own agentResolverAdapter - use shared agent.NewResolver")
	}

	// refine.go SHOULD use the shared agent.NewResolver
	if !strings.Contains(sourceStr, "agent.NewResolver(cfg)") {
		t.Fatal("refine.go should use agent.NewResolver(cfg)")
	}
}

func TestSharedAgentResolver_UsedInReview(t *testing.T) {
	t.Parallel()

	reviewSource, err := os.ReadFile("review.go")
	if err != nil {
		t.Fatalf("failed to read review.go: %v", err)
	}
	sourceStr := string(reviewSource)

	// review.go should NOT have its own agent resolver implementation
	if strings.Contains(sourceStr, "type cliAgentResolver") {
		t.Fatal("review.go should not have its own cliAgentResolver - use shared agent.NewResolver")
	}

	// review.go SHOULD use NewPipelineDeps which constructs the shared agent.NewResolver
	// OR use agent.NewResolver(cfg) directly
	usesNewPipelineDeps := strings.Contains(sourceStr, "NewPipelineDeps(")
	usesDirectResolver := strings.Contains(sourceStr, "agent.NewResolver(cfg)")
	if !usesNewPipelineDeps && !usesDirectResolver {
		t.Fatal("review.go should use either NewPipelineDeps() or agent.NewResolver(cfg)")
	}
}

func TestSharedAgentResolver_IntegrationAcrossCommands(t *testing.T) {
	t.Parallel()

	// All three command files should be using the shared resolver
	files := []string{"explore.go", "refine.go", "review.go"}
	forceNewPipelineDepsFn := "NewPipelineDeps("
	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Skipf("Cannot read %s: %v", file, err)
		}
		sourceStr := string(source)

		// Each should import config
		if !strings.Contains(sourceStr, `"github.com/danabrams/gromit/internal/config"`) {
			t.Logf("Warning: %s should import config package", file)
		}

		// review.go and explore.go use NewPipelineDeps which handles agent resolver
		// refine.go should import and use agent directly or NewPipelineDeps
		if file == "review.go" || file == "explore.go" {
			// These should use NewPipelineDeps for dependency injection
			if !strings.Contains(sourceStr, forceNewPipelineDepsFn) {
				t.Errorf("%s should use NewPipelineDeps() for dependency injection", file)
			}
		} else {
			// refine.go should use either NewPipelineDeps or agent.NewResolver directly
			usesNewPipelineDeps := strings.Contains(sourceStr, forceNewPipelineDepsFn)
			usesDirectResolver := strings.Contains(sourceStr, "agent.NewResolver")
			if !usesNewPipelineDeps && !usesDirectResolver {
				t.Errorf("%s should use either NewPipelineDeps() or agent.NewResolver()", file)
			}
		}
	}
}
