package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readExploreSource(t *testing.T) string {
	t.Helper()

	sourcePath := "explore.go"
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", sourcePath, err)
	}
	return string(data)
}

// TestExploreCommandHasAgentFlag verifies explore command has --agent flag.
func TestExploreCommandHasAgentFlag(t *testing.T) {
	// Expected failure: explore command does not define --agent flag or exploreAgentFlagName constant yet
	flag := exploreCmd.Flags().Lookup("agent")
	if flag == nil {
		t.Error("explore command missing --agent flag")
	}

	if flag != nil && flag.Value.Type() != "string" {
		t.Errorf("--agent flag type = %q, want %q", flag.Value.Type(), "string")
	}
}

// TestExploreCommandHasChooseAgentFlag verifies explore command has --choose-agent flag.
func TestExploreCommandHasChooseAgentFlag(t *testing.T) {
	// Expected failure: explore command does not define --choose-agent flag or exploreChooseAgentFlagName constant yet
	flag := exploreCmd.Flags().Lookup("choose-agent")
	if flag == nil {
		t.Error("explore command missing --choose-agent flag")
	}

	if flag != nil && flag.Value.Type() != "bool" {
		t.Errorf("--choose-agent flag type = %q, want %q", flag.Value.Type(), "bool")
	}
}

// TestExploreCommand_WiresAgentFlagsIntoExploreInput verifies runExplore wires agent flags into ExploreInput.
func TestExploreCommand_WiresAgentFlagsIntoExploreInput(t *testing.T) {
	// Expected failure: runExplore does not read --agent/--choose-agent or populate ExploreInput.ChooseAgent yet
	source := readExploreSource(t)

	if !strings.Contains(source, `GetString("agent")`) {
		t.Error("explore.go does not read --agent flag via GetString(\"agent\")")
	}

	if !strings.Contains(source, `GetBool("choose-agent")`) {
		t.Error("explore.go does not read --choose-agent flag via GetBool(\"choose-agent\")")
	}

	if strings.Contains(source, `AgentName: ""`) {
		t.Error("explore.go still hardcodes AgentName to empty string instead of flag value")
	}

	if !strings.Contains(source, "ChooseAgent:") {
		t.Error("explore.go does not populate ExploreInput.ChooseAgent from --choose-agent flag")
	}
}

func TestExplorePhaseConfigSelectsAgent_Reclassified(t *testing.T) {
	source, err := os.ReadFile("adapters.go")
	if err != nil {
		t.Fatalf("failed to read adapters.go: %v", err)
	}
	sourceStr := string(source)

	if !strings.Contains(sourceStr, "type cmdAgentResolver struct") {
		t.Fatal("adapters.go missing cmdAgentResolver adapter")
	}
	if !strings.Contains(sourceStr, "func (r *cmdAgentResolver) Resolve(phase string") {
		t.Fatal("cmdAgentResolver.Resolve missing")
	}
	if !strings.Contains(sourceStr, "agent.Resolve(r.cfg, phase, flagOverride, choosePicker") {
		t.Fatal("explore agent resolver must pass through phase for phase-config selection")
	}
}

func TestExploreResolverAdapterSingleSource(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		contentStr := string(content)

		if path != "adapters.go" && strings.Contains(contentStr, "agent.Resolve(") {
			t.Errorf("%s must not call agent.Resolve directly; use cmdAgentResolver from adapters.go", path)
		}
		if path != "adapters.go" && strings.Contains(contentStr, "type exploreAgentResolver struct") {
			t.Errorf("%s must not define exploreAgentResolver; resolver adapters are centralized in adapters.go", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk cmd/gromit files: %v", err)
	}
}
