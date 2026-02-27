package main

import (
	"path/filepath"
	"time"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/prompt"
)

// NewPipelineDeps constructs a complete pipeline.Deps with all adapters wired together.
// This is the single dependency injection point for the entire pipeline.
// It assembles all required adapters and returns a fully initialized Deps struct ready for use.
func NewPipelineDeps(cfg *config.Config, gromitDir string) (*pipeline.Deps, error) {
	// Create all required adapters

	// LLM clients - use default timeout if config is nil
	var llmTimeoutSecs int
	var claudeBinary string
	var claudeFlags []string
	if cfg != nil {
		llmTimeoutSecs, _, _, _ = cfg.Claude.TimeoutsForModel("haiku")
		claudeBinary = cfg.Claude.Binary
		claudeFlags = cfg.Claude.Flags
	} else {
		llmTimeoutSecs = 60 // default timeout
		claudeBinary = "claude"
		claudeFlags = []string{}
	}
	claudeClient, err := claude.NewClient(claudeBinary, claudeFlags, llmTimeoutSecs)
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

	deps := &pipeline.Deps{
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
	}

	return deps, nil
}

// SetDepsPromptDiagnosticsProvider updates the LogWriter's diagnostics provider.
// This is used when a workflow needs to provide diagnostics from its renderer.
func SetDepsPromptDiagnosticsProvider(deps *pipeline.Deps, provider func() *prompt.PromptDiagnostics) {
	if deps == nil || deps.LogWriter == nil {
		return
	}
	if logWriter, ok := deps.LogWriter.(*cliLogWriter); ok {
		logWriter.promptDiagnosticsProvider = provider
	}
}
