package main

import (
	"os"
	"strings"
	"testing"
)

func TestSharedAgentResolver_DefinedInAdapters(t *testing.T) {
	t.Parallel()

	adaptersSource, err := os.ReadFile("adapters.go")
	if err != nil {
		t.Fatalf("failed to read adapters.go: %v", err)
	}
	sourceStr := string(adaptersSource)

	// Verify cmdAgentResolver is defined
	if !strings.Contains(sourceStr, "type cmdAgentResolver struct") {
		t.Fatal("cmdAgentResolver not found in adapters.go")
	}

	// Verify it implements pipeline.AgentResolver
	if !strings.Contains(sourceStr, "var _ pipeline.AgentResolver = (*cmdAgentResolver)(nil)") {
		t.Fatal("cmdAgentResolver does not declare pipeline.AgentResolver interface")
	}

	// Verify Resolve method delegates to agent.Resolve
	if !strings.Contains(sourceStr, "func (r *cmdAgentResolver) Resolve(phase string, flagOverride string, choosePicker bool)") {
		t.Fatal("cmdAgentResolver.Resolve method not found")
	}
	if !strings.Contains(sourceStr, "agent.Resolve(r.cfg, phase, flagOverride, choosePicker, os.Stdin, os.Stdout)") {
		t.Fatal("cmdAgentResolver.Resolve does not call agent.Resolve correctly")
	}

	// Verify newAgentResolver constructor exists
	if !strings.Contains(sourceStr, "func newAgentResolver(cfg *config.Config) pipeline.AgentResolver") {
		t.Fatal("newAgentResolver constructor not found")
	}
}

func TestSharedAgentResolver_UsedInExplore(t *testing.T) {
	t.Parallel()

	exploreSource, err := os.ReadFile("explore.go")
	if err != nil {
		t.Fatalf("failed to read explore.go: %v", err)
	}
	sourceStr := string(exploreSource)

	// explore.go should NOT have its own agent resolver implementation
	if strings.Contains(sourceStr, "type exploreAgentResolver") {
		t.Fatal("explore.go should not have its own exploreAgentResolver - use shared newAgentResolver")
	}

	// explore.go SHOULD use newAgentResolver
	if !strings.Contains(sourceStr, "newAgentResolver(cfg)") {
		t.Fatal("explore.go should use newAgentResolver(cfg)")
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
		t.Fatal("refine.go should not have its own agentResolverAdapter - use shared newAgentResolver")
	}

	// refine.go SHOULD use newAgentResolver
	if !strings.Contains(sourceStr, "newAgentResolver(cfg)") {
		t.Fatal("refine.go should use newAgentResolver(cfg)")
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
		t.Fatal("review.go should not have its own cliAgentResolver - use shared newAgentResolver")
	}

	// review.go SHOULD use newAgentResolver
	if !strings.Contains(sourceStr, "newAgentResolver(cfg)") {
		t.Fatal("review.go should use newAgentResolver(cfg)")
	}
}

func TestSharedAgentResolver_IntegrationAcrossCommands(t *testing.T) {
	t.Parallel()

	adaptersSource, err := os.ReadFile("adapters.go")
	if err != nil {
		t.Skipf("Cannot read adapters.go: %v", err)
	}
	adaptersStr := string(adaptersSource)

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

		// Each should use newAgentResolver
		if !strings.Contains(sourceStr, "newAgentResolver") {
			t.Errorf("%s does not use newAgentResolver - should not have duplicate adapter", file)
		}
	}

	// Shared adapter should have agent import
	if !strings.Contains(adaptersStr, `"github.com/danabrams/gromit/internal/agent"`) {
		t.Error("adapters.go must import agent package for shared resolver")
	}
}
