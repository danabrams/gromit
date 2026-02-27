package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/prompt"
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

// NewPipelineDeps constructs a complete pipeline.Deps with all adapters wired together.
// This is the single dependency injection point for the entire pipeline.
func NewPipelineDeps(cfg *config.Config, gromitDir string) (*pipeline.Deps, error) {
	// Create all required adapters

	// LLM clients
	llmTimeoutSecs, _, _, _ := cfg.Claude.TimeoutsForModel("haiku")
	claudeClient, err := claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, llmTimeoutSecs)
	if err != nil {
		return nil, err
	}
	claudeAdapter := &claudeClientAdapter{
		Client:  claudeClient,
		Timeout: time.Duration(llmTimeoutSecs) * time.Second,
	}

	// Agent resolver
	agentResolver := agent.NewResolver(cfg)

	// Tracker client - wrap bead client with tracker adapter
	beadClient, err := bead.NewClient()
	if err != nil {
		return nil, err
	}
	bdAdapter := bead.NewBDAdapter(beadClient)
	trackerAdapter := &trackerClientAdapter{
		Client: bdAdapter,
	}

	// Backlog client
	backlogFile, err := backlog.NewFile(gromitDir)
	if err != nil {
		return nil, err
	}
	backlogClient := &backlogClientAdapter{
		file: backlogFile,
	}
	backlogWriter := &cliBacklogClient{
		beadClient: beadClient,
	}

	// Prompt renderers
	templatesDir := filepath.Join(gromitDir, "templates")
	specsDir := filepath.Join(gromitDir, "specs")
	claudeMDPath := filepath.Join(gromitDir, "CLAUDE.md")
	promptRenderer, err := prompt.NewRenderer(templatesDir, specsDir, claudeMDPath, gromitDir)
	if err != nil {
		return nil, err
	}

	refineRenderer := &refinePromptRenderer{renderer: promptRenderer}
	planRenderer := &planPromptRenderer{renderer: promptRenderer}
	decomposeRenderer := &decomposePromptRenderer{renderer: promptRenderer}
	reviewRenderer := &cliPromptRenderer{renderer: promptRenderer}
	exploreRenderer := &explorePromptRenderer{renderer: promptRenderer}

	// Learning and state managers
	learningsManager := &cliLearningsManager{
		gromitDir: gromitDir,
		runner:    nil, // Will be injected by caller
	}
	stateManager := &cliStateManager{
		gromitDir: gromitDir,
	}

	// Log writer
	logWriter := &cliLogWriter{
		logsDir:                   filepath.Join(gromitDir, "logs"),
		promptDiagnosticsProvider: nil, // Will be injected by caller
	}

	return &pipeline.Deps{
		AgentResolver:     agentResolver,
		LLMClient:         claudeAdapter,
		ReviewInvoker:     claudeAdapter,
		TrackerClient:     trackerAdapter,
		BacklogClient:     backlogClient,
		BacklogWriter:     backlogWriter,
		RefineRenderer:    refineRenderer,
		PlanRenderer:      planRenderer,
		DecomposeRenderer: decomposeRenderer,
		ReviewRenderer:    reviewRenderer,
		ExploreRenderer:   exploreRenderer,
		LearningsManager:  learningsManager,
		StateManager:      stateManager,
		LogWriter:         logWriter,
	}, nil
}
