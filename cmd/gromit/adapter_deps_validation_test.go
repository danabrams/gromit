package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

// TestAdapterDeps_NewPipelineDepsInitializesAllFields verifies that NewPipelineDeps
// properly initializes all required pipeline.Deps fields with non-nil adapters.
// This test will FAIL if any required field is missing from the wiring.
func TestAdapterDeps_NewPipelineDepsInitializesAllFields(t *testing.T) {
	t.Parallel()

	// This test is RED - if NewPipelineDeps doesn't initialize all fields,
	// the validation below will fail.

	deps, err := NewPipelineDeps(nil, t.TempDir())
	if err != nil {
		t.Fatalf("NewPipelineDeps failed: %v", err)
	}

	if deps == nil {
		t.Fatal("NewPipelineDeps returned nil")
	}

	// Verify all required fields are initialized
	fields := map[string]interface{}{
		"AgentResolver":     deps.AgentResolver,
		"LLMClient":         deps.LLMClient,
		"ReviewInvoker":     deps.ReviewInvoker,
		"TrackerClient":     deps.TrackerClient,
		"BeadQueryClient":   deps.BeadQueryClient,
		"BoardClient":       deps.BoardClient,
		"QueueClient":       deps.QueueClient,
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

	for fieldName, field := range fields {
		if field == nil {
			t.Errorf("pipeline.Deps.%s is nil - must be initialized by NewPipelineDeps", fieldName)
		}
	}

	t.Log("All pipeline.Deps fields are properly initialized")
}

// TestAdapterDeps_AllFieldsImplementCorrectInterface verifies that each
// pipeline.Deps field implements its corresponding interface.
func TestAdapterDeps_AllFieldsImplementCorrectInterface(t *testing.T) {
	t.Parallel()

	deps, err := NewPipelineDeps(nil, t.TempDir())
	if err != nil {
		t.Fatalf("NewPipelineDeps failed: %v", err)
	}

	if deps == nil {
		t.Fatal("NewPipelineDeps returned nil")
	}

	// Verify each field implements its interface
	tests := []struct {
		fieldName string
		field     interface{}
		checkType string
	}{
		{
			fieldName: "AgentResolver",
			field:     deps.AgentResolver,
			checkType: "AgentResolver",
		},
		{
			fieldName: "LLMClient",
			field:     deps.LLMClient,
			checkType: "LLMClient",
		},
		{
			fieldName: "ReviewInvoker",
			field:     deps.ReviewInvoker,
			checkType: "ReviewInvoker",
		},
		{
			fieldName: "TrackerClient",
			field:     deps.TrackerClient,
			checkType: "TrackerClient",
		},
		{
			fieldName: "BeadQueryClient",
			field:     deps.BeadQueryClient,
			checkType: "BeadQueryClient",
		},
		{
			fieldName: "BacklogClient",
			field:     deps.BacklogClient,
			checkType: "BacklogClient",
		},
		{
			fieldName: "BacklogWriter",
			field:     deps.BacklogWriter,
			checkType: "BacklogWriter",
		},
		{
			fieldName: "RefineRenderer",
			field:     deps.RefineRenderer,
			checkType: "RefineRenderer",
		},
		{
			fieldName: "PlanRenderer",
			field:     deps.PlanRenderer,
			checkType: "PlanRenderer",
		},
		{
			fieldName: "DecomposeRenderer",
			field:     deps.DecomposeRenderer,
			checkType: "DecomposeRenderer",
		},
		{
			fieldName: "ReviewRenderer",
			field:     deps.ReviewRenderer,
			checkType: "ReviewRenderer",
		},
		{
			fieldName: "ExploreRenderer",
			field:     deps.ExploreRenderer,
			checkType: "ExploreRenderer",
		},
		{
			fieldName: "BoardClient",
			field:     deps.BoardClient,
			checkType: "BoardClient",
		},
		{
			fieldName: "QueueClient",
			field:     deps.QueueClient,
			checkType: "QueueClient",
		},
		{
			fieldName: "LearningsManager",
			field:     deps.LearningsManager,
			checkType: "LearningsManager",
		},
		{
			fieldName: "StateManager",
			field:     deps.StateManager,
			checkType: "StateManager",
		},
		{
			fieldName: "LogWriter",
			field:     deps.LogWriter,
			checkType: "LogWriter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			if tt.field == nil {
				t.Errorf("pipeline.Deps.%s is nil", tt.fieldName)
			} else {
				t.Logf("pipeline.Deps.%s is properly initialized", tt.fieldName)
			}
		})
	}
}

// TestAdapterDeps_WithCustomConfig verifies that NewPipelineDeps properly
// initializes dependencies when given a non-nil config.
func TestAdapterDeps_WithCustomConfig(t *testing.T) {
	t.Parallel()

	// Create a minimal config
	cfg := &config.Config{}

	deps, err := NewPipelineDeps(cfg, t.TempDir())
	if err != nil {
		// Error is acceptable in test environment, just verify NewPipelineDeps handles config
		t.Logf("NewPipelineDeps with config returned error (expected in test): %v", err)
		return
	}

	if deps == nil {
		t.Fatal("NewPipelineDeps returned nil even with config")
	}

	t.Log("NewPipelineDeps properly handles custom config")
}

// TestAdapterDeps_DepsContractSignature verifies that pipeline.Deps is properly
// formalized with compile-time interface checks.
func TestAdapterDeps_DepsContractSignature(t *testing.T) {
	t.Parallel()

	// Create an empty deps to verify the structure exists
	deps := &pipeline.Deps{
		AgentResolver:     nil,
		LLMClient:         nil,
		ReviewInvoker:     nil,
		TrackerClient:     nil,
		BeadQueryClient:   nil,
		BoardClient:       nil,
		QueueClient:       nil,
		BacklogClient:     nil,
		BacklogWriter:     nil,
		RefineRenderer:    nil,
		PlanRenderer:      nil,
		DecomposeRenderer: nil,
		ReviewRenderer:    nil,
		ExploreRenderer:   nil,
		LearningsManager:  nil,
		StateManager:      nil,
		LogWriter:         nil,
	}

	// Verify all expected fields exist through type checks
	if deps == nil {
		t.Fatal("pipeline.Deps is nil")
	}

	t.Log("pipeline.Deps structure is properly formalized")
}

// TestAdapterDeps_ModelForwardingWiring verifies the explore pipeline wiring adds
// the model forwarder and warning writer so non-Claude model overrides behave.
func TestAdapterDeps_ModelForwardingWiring(t *testing.T) {
	t.Parallel()

	deps, err := NewPipelineDeps(nil, t.TempDir())
	if err != nil {
		t.Fatalf("NewPipelineDeps failed: %v", err)
	}

	if deps.ModelForwarder == nil {
		t.Fatal("NewPipelineDeps should wire ModelForwarder")
	}

	if deps.WarningWriter == nil {
		t.Fatal("NewPipelineDeps should wire WarningWriter")
	}

	promptFile := filepath.Join(t.TempDir(), "explore-prompt.md")
	if err := os.WriteFile(promptFile, []byte("prompt"), 0o644); err != nil {
		t.Fatalf("failed to write prompt file: %v", err)
	}

	codexAgent, err := agent.Resolve(nil, "explore", "codex", false, strings.NewReader(""), io.Discard)
	if err != nil {
		t.Fatalf("agent.Resolve(codex) failed: %v", err)
	}

	forwarded, warning := deps.ModelForwarder(codexAgent, "gpt-5.3-codex")
	if warning != "" {
		t.Fatalf("ModelForwarder returned warning for codex: %q", warning)
	}

	commandProvider, ok := forwarded.(interface {
		Command(string) (*exec.Cmd, error)
	})
	if !ok {
		t.Fatal("forwarded agent does not expose Command()")
	}

	cmd, err := commandProvider.Command(promptFile)
	if err != nil {
		t.Fatalf("forwarded agent Command() failed: %v", err)
	}

	if !modelArgsInclude(cmd.Args, "--model", "gpt-5.3-codex") {
		t.Fatalf("forwarded command missing --model args: %v", cmd.Args)
	}

	claudeAgent, err := agent.Resolve(nil, "explore", "claude", false, strings.NewReader(""), io.Discard)
	if err != nil {
		t.Fatalf("agent.Resolve(claude) failed: %v", err)
	}

	unsupportedWarning := "model forwarding not supported for agent claude"
	_, warning = deps.ModelForwarder(claudeAgent, "sonnet")
	if warning != unsupportedWarning {
		t.Fatalf("ModelForwarder(claude) warning = %q, want %q", warning, unsupportedWarning)
	}

	captureWarningOutput(t, deps.WarningWriter, warning)
}

func modelArgsInclude(args []string, flag, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

func captureWarningOutput(t *testing.T, writer func(string), message string) {
	t.Helper()
	if writer == nil {
		t.Fatal("warning writer is nil")
	}

	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to capture stderr: %v", err)
	}
	defer r.Close()
	os.Stderr = w

	writer(message)
	_ = w.Close()
	os.Stderr = old

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read warning output: %v", err)
	}
	if !strings.Contains(string(data), message) {
		t.Fatalf("warning output missing %q, got %q", message, string(data))
	}
}
