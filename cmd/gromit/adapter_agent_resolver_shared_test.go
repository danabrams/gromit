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
		t.Fatal("explore.go should not have its own exploreAgentResolver - use shared agents.NewResolver")
	}

	// explore.go SHOULD use the shared agents.NewResolver
	if !strings.Contains(sourceStr, "agents.NewResolver(cfg)") {
		t.Fatal("explore.go should use agents.NewResolver(cfg)")
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
		t.Fatal("refine.go should not have its own agentResolverAdapter - use shared agents.NewResolver")
	}

	// refine.go SHOULD use the shared agents.NewResolver
	if !strings.Contains(sourceStr, "agents.NewResolver(cfg)") {
		t.Fatal("refine.go should use agents.NewResolver(cfg)")
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
		t.Fatal("review.go should not have its own cliAgentResolver - use shared agents.NewResolver")
	}

	// review.go SHOULD use the shared agents.NewResolver
	if !strings.Contains(sourceStr, "agents.NewResolver(cfg)") {
		t.Fatal("review.go should use agents.NewResolver(cfg)")
	}
}

func TestSharedAgentResolver_IntegrationAcrossCommands(t *testing.T) {
	t.Parallel()

	// All three command files should be using the shared resolver
	files := []string{"explore.go", "refine.go", "review.go"}
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

		// Each should import the shared resolver package
		if !strings.Contains(sourceStr, `"github.com/danabrams/gromit/internal/agents"`) {
			t.Errorf("%s does not import agents package for the shared resolver", file)
		}

		// Each should use agents.NewResolver
		if !strings.Contains(sourceStr, "agents.NewResolver") {
			t.Errorf("%s does not use agents.NewResolver - should not have duplicate adapter", file)
		}
	}
}
