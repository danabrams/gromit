package main

import (
	"path/filepath"
	"time"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/state"
)

// NewPipelineDeps constructs a complete pipeline.Deps with all adapters wired together.
// This is the single dependency injection point for the entire pipeline.
// It assembles all required adapters and returns a fully initialized Deps struct ready for use.
//
// === Adapter Wiring Contract ===
//
// Each adapter in pipeline.Deps:
// - Implements exactly one pipeline.* interface
// - Wraps exactly one internal dependency (wrapped field is primary state)
// - Delegates to wrapped dependency methods, performing minimal type transformation
// - Includes compile-time interface assertion (var _ interface = (*type)(nil))
// - Contains no business orchestration logic or prompt building
//
// Adapter Organization:
//
// adapters.go (LLM providers and task tracking):
//   LLMClient + ReviewInvoker implementations:
//     - claudeClientAdapter: Wraps claude.Client, adds timeout context
//     - llmRouterClientAdapter: Wraps provider.Router for multi-provider fallback
//
//   Task Tracking (bead/backlog management):
//     - trackerClientAdapter: Wraps tracker.Client (bead.BDAdapter), implements TrackerClient
//     - backlogClientAdapter: Wraps backlog.File, implements BacklogClient (read-only)
//     - beadQueryClientAdapter: Wraps bead.Client, implements BeadQueryClient (status queries)
//
// cli_adapters.go (CLI-specific integrations):
//   Prompt Rendering (all wrap prompt.Renderer):
//     - refinePromptRenderer: Implements RefineRenderer
//     - planPromptRenderer: Implements PlanRenderer
//     - decomposePromptRenderer: Implements DecomposeRenderer
//     - cliPromptRenderer: Implements ReviewRenderer (for thorough code review)
//     - explorePromptRenderer: Implements ExploreRenderer
//       NOTE: explorePromptRenderer currently contains business logic that should
//       be moved to prompt.Renderer.RenderExplore or a service layer.
//
//   CLI State Management:
//     - cliBacklogClient: Wraps bead.Client, implements BacklogWriter (write-only)
//     - cliLearningsManager: Wraps learnings.File, implements LearningsManager
//     - cliStateManager: Wraps state.File, implements StateManager
//     - cliLogWriter: Wraps logger facilities, implements LogWriter
//
// All adapters are instantiated here in NewPipelineDeps and wired into the Deps struct.
// Callers use NewPipelineDeps to get a complete, ready-to-use dependency container.
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

	// Bead query client
	beadQueryAdapter := &beadQueryClientAdapter{
		Client: beadClient,
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
	// Create and load learnings file with filter
	learningsFile, err := learnings.NewFile(gromitDir)
	if err != nil {
		return nil, err
	}
	// Note: Filter will be configured by caller via SetDepsLearningsFilter if needed

	learningsManager := &cliLearningsManager{
		file: learningsFile,
	}

	// Create and load state file
	stateFile, err := state.NewInteractiveFile(gromitDir)
	if err != nil {
		return nil, err
	}
	if err := stateFile.Load(); err != nil {
		return nil, err
	}

	stateManager := &cliStateManager{
		stateFile: stateFile,
	}

	// Log writer
	logWriter := &cliLogWriter{
		logsDir:                   filepath.Join(gromitDir, "logs"),
		logType:                   "review",
		logReviewType:             "thorough-cli",
		defaultModel:              "opus",
		promptDiagnosticsProvider: nil, // Will be injected by caller
	}

	deps := &pipeline.Deps{
		AgentResolver:     agentResolver,
		LLMClient:         claudeAdapter,
		ReviewInvoker:     claudeAdapter,
		TrackerClient:     trackerAdapter,
		BeadQueryClient:   beadQueryAdapter,
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
