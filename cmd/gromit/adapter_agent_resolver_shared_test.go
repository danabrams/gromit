package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// sourcePathForSharedResolverTests returns the absolute path to a source file in the cmd/gromit package.
func sourcePathForSharedResolverTests(filename string) string {
	_, testFile, _, _ := runtime.Caller(1)
	dir := filepath.Dir(testFile)
	return filepath.Join(dir, filename)
}

func TestSharedAgentResolver_UsedInExplore(t *testing.T) {
	t.Parallel()

	exploreSource, err := os.ReadFile(sourcePathForSharedResolverTests("explore.go"))
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

	refineSource, err := os.ReadFile(sourcePathForSharedResolverTests("refine.go"))
	if err != nil {
		t.Fatalf("failed to read refine.go: %v", err)
	}
	sourceStr := string(refineSource)

	// refine.go should NOT have its own agent resolver implementation
	if strings.Contains(sourceStr, "type agentResolverAdapter") {
		t.Fatal("refine.go should not have its own agentResolverAdapter - use shared agent.NewResolver")
	}

	// refine.go SHOULD use NewPipelineDeps (which constructs the shared agent.NewResolver)
	// OR use agent.NewResolver(cfg) directly
	usesNewPipelineDeps := strings.Contains(sourceStr, "NewPipelineDeps(")
	usesDirectResolver := strings.Contains(sourceStr, "agent.NewResolver(cfg)")
	if !usesNewPipelineDeps && !usesDirectResolver {
		t.Fatal("refine.go should use either NewPipelineDeps() or agent.NewResolver(cfg)")
	}
}

func TestSharedAgentResolver_UsedInReview(t *testing.T) {
	t.Parallel()

	reviewSource, err := os.ReadFile(sourcePathForSharedResolverTests("review.go"))
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
		source, err := os.ReadFile(sourcePathForSharedResolverTests(file))
		if err != nil {
			t.Skipf("Cannot read %s: %v", file, err)
		}
		sourceStr := string(source)

		// Each should import config
		if !strings.Contains(sourceStr, `"github.com/danabrams/gromit/internal/config"`) {
			t.Logf("Warning: %s should import config package", file)
		}

		// All files should use NewPipelineDeps for consistent dependency injection
		if !strings.Contains(sourceStr, forceNewPipelineDepsFn) {
			t.Errorf("%s should use NewPipelineDeps() for dependency injection", file)
		}
	}
}
