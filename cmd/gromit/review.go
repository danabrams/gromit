package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/scope"
	"github.com/danabrams/gromit/internal/state"
	"github.com/spf13/cobra"
)

var (
	reviewNonInteractive bool
	reviewSince          string
	reviewEpic           string
	reviewSpec           string
	reviewDryRun         bool
	reviewAgent          string
	reviewChooseAgent    bool
)

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Run a thorough code review",
	Long: `Run a thorough review of recent changes.

Interactive mode (default): Launches an interactive agent session for collaborative review.
Non-interactive mode (--non-interactive): Runs autonomously, creates beads for issues found.

Scope options:
  --since <commit>   Review from a specific commit
  --spec <name>      Review changes from a spec's beads
  --epic <id>        Review changes from an epic's beads
  (default)          Review since last thorough review`,
	RunE: runReview,
}

func init() {
	reviewCmd.Flags().BoolVar(&reviewNonInteractive, "non-interactive", false, "Run review autonomously without interactive session")
	reviewCmd.Flags().StringVar(&reviewSince, "since", "", "Review from this commit")
	reviewCmd.Flags().StringVar(&reviewEpic, "epic", "", "Review changes from this epic's beads")
	reviewCmd.Flags().StringVar(&reviewSpec, "spec", "", "Review changes from this spec's beads")
	reviewCmd.Flags().BoolVar(&reviewDryRun, "dry-run", false, "Preview what would be reviewed")
	reviewCmd.Flags().StringVar(&reviewAgent, "agent", "", "Override the agent to use for interactive review")
	reviewCmd.Flags().BoolVar(&reviewChooseAgent, "choose-agent", false, "Show picker to select agent for interactive review")
	rootCmd.AddCommand(reviewCmd)
}

func runReview(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Determine scope (from commit)
	fromCommit, err := determineReviewScope(cfg)
	if err != nil {
		return fmt.Errorf("determining review scope: %w", err)
	}

	// Get diff
	diff, err := getGitDiffForReview(fromCommit)
	if err != nil {
		return fmt.Errorf("getting diff: %w", err)
	}

	if strings.TrimSpace(diff) == "" {
		fmt.Println("No changes to review since", shortCommit(fromCommit))
		return nil
	}

	// Dry-run: show scope and exit
	if reviewDryRun {
		diffStat, err := getGitDiffStatForReview(fromCommit)
		if err != nil {
			return fmt.Errorf("getting diff stat: %w", err)
		}
		fmt.Printf("Review scope: from commit %s to HEAD\n\n", shortCommit(fromCommit))
		fmt.Println(diffStat)
		return nil
	}

	// Interactive mode (default)
	if !reviewNonInteractive {
		return runReviewInteractive(cfg, fromCommit, diff)
	}

	// Non-interactive mode
	return runReviewNonInteractive(cfg, fromCommit, diff)
}

func determineReviewScope(cfg *config.Config) (string, error) {
	return determineReviewScopeWithClient(cfg, nil)
}

func determineReviewScopeWithClient(cfg *config.Config, beadsClient *bead.Client) (string, error) {
	// Validate mutual exclusivity of --epic, --spec, and --since
	if err := scope.ValidateFlags(reviewEpic, reviewSpec, reviewSince); err != nil {
		return "", err
	}

	// Priority: --since flag > --spec flag > --epic flag > state file
	if reviewSince != "" {
		return reviewSince, nil
	}

	if reviewSpec != "" {
		// Find the earliest commit from beads in this spec
		if beadsClient == nil {
			var err error
			beadsClient, err = bead.NewClient()
			if err != nil {
				return "", fmt.Errorf("creating bead client: %w", err)
			}
		}
		return getSpecBaseCommit(beadsClient, reviewSpec, cfg.Paths.Specs)
	}

	if reviewEpic != "" {
		// Find the earliest commit from beads in this epic
		gromitDir := resolveGromitDir(cfg)
		return getEpicBaseCommit(reviewEpic, cfg.Paths.Specs, gromitDir)
	}

	// Default: use last review commit from state
	gromitDir := resolveGromitDir(cfg)

	sf, err := state.NewInteractiveFile(gromitDir)
	if err != nil {
		return "", fmt.Errorf("creating state file: %w", err)
	}

	if err := sf.Load(); err != nil {
		return "", fmt.Errorf("loading state: %w", err)
	}

	fromCommit := sf.LastReviewCommit()
	if fromCommit == "" {
		return "", fmt.Errorf("no previous review found - use --since to specify a commit")
	}

	return fromCommit, nil
}

func getSpecBaseCommit(beadsClient *bead.Client, specName string, specsDir string) (string, error) {
	if beadsClient == nil {
		return "", fmt.Errorf("bead client is nil")
	}
	// Validate spec file exists before attempting to resolve
	if err := scope.ValidateSpec(specsDir, specName); err != nil {
		return "", err
	}

	// Get the spec label
	labels := scope.ResolveSpec(specName)
	if len(labels) == 0 {
		return "", fmt.Errorf("no label found for spec %q", specName)
	}

	// Get all beads with this label
	beadsWithLabel, err := beadsClient.ListWithLabel(labels[0])
	if err != nil {
		return "", fmt.Errorf("listing beads with label %q: %w", labels[0], err)
	}

	if len(beadsWithLabel) == 0 {
		return "", fmt.Errorf("no beads found for spec %q - try using --since to specify a commit", specName)
	}

	// Find the earliest commit from these beads
	earliestCommit := findEarliestCommitFromBeads(beadsWithLabel)
	if earliestCommit == "" {
		return "", fmt.Errorf("no commits found for spec %q - try using --since to specify a commit", specName)
	}

	return earliestCommit, nil
}

func getEpicBaseCommit(epicID, specsDir, gromitDir string) (string, error) {
	// Use scope.ResolveEpic to get spec labels for this epic
	specLabels, err := scope.ResolveEpic(epicID, specsDir)
	if err != nil {
		return "", fmt.Errorf("resolving epic %q: %w", epicID, err)
	}

	if len(specLabels) == 0 {
		return "", fmt.Errorf("no specs found for epic %q - try using --since to specify a commit", epicID)
	}

	// Get beads for each spec label
	beadsClient, err := bead.NewClient()
	if err != nil {
		return "", fmt.Errorf("creating bead client: %w", err)
	}

	allBeads := make([]*bead.Bead, 0)
	for _, label := range specLabels {
		beadsWithLabel, err := beadsClient.ListWithLabel(label)
		if err != nil {
			return "", fmt.Errorf("listing beads with label %q: %w", label, err)
		}
		allBeads = append(allBeads, beadsWithLabel...)
	}

	if len(allBeads) == 0 {
		return "", fmt.Errorf("no beads found for epic %q - try using --since to specify a commit", epicID)
	}

	// Find the earliest commit from these beads
	earliestCommit := findEarliestCommitFromBeads(allBeads)
	if earliestCommit == "" {
		return "", fmt.Errorf("no commits found for epic %q - try using --since to specify a commit", epicID)
	}

	return earliestCommit, nil
}

func findFirstCommitForBead(beadID string) (string, error) {
	cmd := exec.Command("git", "log", "--all", "--format=%H", "--grep", beadID)
	out, err := cmd.Output()
	if err != nil {
		return "", nil // No commits found - not an error, just no work yet
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > 0 && lines[0] != "" {
		// git log returns newest first, so the last line is the earliest commit
		return lines[len(lines)-1], nil
	}

	return "", nil
}

func isCommitEarlier(commit1, commit2 string) bool {
	ts1, err1 := getCommitTimestamp(commit1)
	ts2, err2 := getCommitTimestamp(commit2)
	if err1 != nil || err2 != nil {
		return false
	}
	return ts1 < ts2
}

func getCommitTimestamp(commit string) (int64, error) {
	cmd := exec.Command("git", "log", "-1", "--format=%at", commit)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
}

// findEarliestCommitFromBeads iterates through beads and returns the earliest commit found.
// Returns empty string if no commits are found.
func findEarliestCommitFromBeads(beads []*bead.Bead) string {
	var earliestCommit string

	for _, b := range beads {
		commit, err := findFirstCommitForBead(b.ID)
		if err != nil || commit == "" {
			continue // Skip beads without commits
		}

		if earliestCommit == "" || isCommitEarlier(commit, earliestCommit) {
			earliestCommit = commit
		}
	}

	return earliestCommit
}

func getGitDiffForReview(fromCommit string) (string, error) {
	cmd := exec.Command("git", "diff", fromCommit)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

func getGitDiffStatForReview(fromCommit string) (string, error) {
	cmd := exec.Command("git", "diff", "--stat", fromCommit)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff --stat: %w", err)
	}
	return string(out), nil
}

func runReviewInteractive(cfg *config.Config, fromCommit string, diff string) error {
	// Print status message
	fmt.Printf("Launching interactive review session (from commit %s)...\n", shortCommit(fromCommit))

	// Build pipeline and dependencies
	gromitDir := resolveGromitDir(cfg)

	renderer, err := prompt.NewRenderer(
		cfg.Paths.Templates,
		cfg.Paths.Specs,
		cfg.Paths.ProjectClaudeMD,
		gromitDir,
	)
	if err != nil {
		return fmt.Errorf("creating renderer: %w", err)
	}

	// Create agent resolver adapter
	agentResolver := &cliAgentResolver{
		cfg: cfg,
	}

	// Create prompt renderer adapter that loads ClaudeMD and Rules
	promptRendererAdapter := &cliPromptRenderer{
		renderer: renderer,
	}

	deps := &pipeline.Deps{
		AgentResolver:  agentResolver,
		PromptRenderer: promptRendererAdapter,
	}

	paths := &pipeline.Paths{
		GromitDir: gromitDir,
	}

	p := pipeline.New(deps, paths)

	// Prepare input
	input := pipeline.ReviewInput{
		FromCommit: fromCommit,
		Diff:       diff,
		Model:      cfg.Review.Thorough.Model,
		AgentName:  reviewAgent,
	}

	// Call pipeline
	_, err = p.ReviewInteractive(context.Background(), input)
	if err != nil {
		return fmt.Errorf("review interactive: %w", err)
	}

	return nil
}

func runReviewNonInteractive(cfg *config.Config, fromCommit string, diff string) error {
	fmt.Printf("Running autonomous thorough review (from commit %s)...\n", shortCommit(fromCommit))

	// Build pipeline and dependencies
	gromitDir := resolveGromitDir(cfg)

	renderer, err := prompt.NewRenderer(
		cfg.Paths.Templates,
		cfg.Paths.Specs,
		cfg.Paths.ProjectClaudeMD,
		gromitDir,
	)
	if err != nil {
		return fmt.Errorf("creating renderer: %w", err)
	}

	claudeClient, err := claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, cfg.Claude.Timeout)
	if err != nil {
		return fmt.Errorf("creating claude client: %w", err)
	}

	beadsClient, err := bead.NewClient()
	if err != nil {
		return fmt.Errorf("creating bead client: %w", err)
	}

	// Create adapters
	promptRendererAdapter := &cliPromptRenderer{
		renderer: renderer,
	}

	claudeAdapter := &claudeClientAdapter{
		Client: claudeClient,
	}

	beadAdapter := &beadClientAdapter{
		Client: beadsClient,
	}

	backlogAdapter := &cliBacklogClient{
		beadClient: beadsClient,
	}

	learningsAdapter := &cliLearningsManager{
		gromitDir:    gromitDir,
		claudeClient: claudeClient,
	}

	logAdapter := &cliLogWriter{
		logsDir: cfg.Paths.Logs,
	}

	stateAdapter := &cliStateManager{
		gromitDir: gromitDir,
	}

	deps := &pipeline.Deps{
		PromptRenderer:   promptRendererAdapter,
		ClaudeClient:     claudeAdapter,
		BeadClient:       beadAdapter,
		BacklogClient:    backlogAdapter,
		LearningsManager: learningsAdapter,
		LogWriter:        logAdapter,
		StateManager:     stateAdapter,
	}

	paths := &pipeline.Paths{
		GromitDir: gromitDir,
	}

	p := pipeline.New(deps, paths)

	// Prepare input
	timeout := time.Duration(cfg.Review.Thorough.Timeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	input := pipeline.ReviewInput{
		FromCommit: fromCommit,
		Diff:       diff,
		Model:      cfg.Review.Thorough.Model,
		Timeout:    cfg.Review.Thorough.Timeout,
	}

	// Call pipeline
	result, err := p.ReviewNonInteractive(ctx, input)
	if err != nil {
		return fmt.Errorf("review non-interactive: %w", err)
	}

	// Display output
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("REVIEW SUMMARY")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println(result.Summary)
	fmt.Println()

	if result.FixesApplied > 0 {
		fmt.Printf("Fixes applied: %d\n", result.FixesApplied)
		fmt.Println()
	}

	if result.BeadsCreated > 0 {
		fmt.Printf("Created %d beads from review findings\n", result.BeadsCreated)
	}
	if result.BacklogCreated > 0 {
		fmt.Printf("Created %d backlog items\n", result.BacklogCreated)
	}

	fmt.Println("\nReview complete!")
	return nil
}

func getGitHeadForReview() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// validateCommitRef rejects refs that look like git flags (start with "-") or are empty.
func validateCommitRef(ref string) error {
	if ref == "" {
		return fmt.Errorf("commit ref must not be empty")
	}
	if strings.HasPrefix(ref, "-") {
		return fmt.Errorf("invalid commit ref %q: must not start with '-'", ref)
	}
	return nil
}

func shortCommit(commit string) string {
	if len(commit) > 8 {
		return commit[:8]
	}
	return commit
}

// cliAgentResolver adapts agent.Resolve to the pipeline.AgentResolver interface
type cliAgentResolver struct {
	cfg *config.Config
}

func (r *cliAgentResolver) Resolve(phase string, flagOverride string, choosePicker bool) (pipeline.Agent, error) {
	// Use the CLI's agent.Resolve which handles stdin/stdout
	return agent.Resolve(r.cfg, phase, flagOverride, choosePicker, os.Stdin, os.Stdout)
}

// cliPromptRenderer adapts prompt.Renderer to pipeline.PromptRenderer interface
// It loads ClaudeMD and Rules before rendering
type cliPromptRenderer struct {
	renderer *prompt.Renderer
}

func (r *cliPromptRenderer) RenderRefine(input *pipeline.RefinePromptInput) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (r *cliPromptRenderer) RenderPlan(input *pipeline.PlanPromptInput) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (r *cliPromptRenderer) RenderDecompose(input *pipeline.DecomposePromptInput) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (r *cliPromptRenderer) RenderThoroughReview(input *pipeline.ThoroughReviewPromptInput) (string, error) {
	// Build ThoroughReviewContext from pipeline input
	reviewCtx := &prompt.ThoroughReviewContext{
		Diff: input.Diff,
	}

	// Load ClaudeMD and Rules (warnings only)
	var err error
	reviewCtx.ClaudeMD, err = r.renderer.LoadClaudeMD()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load CLAUDE.md: %v\n", err)
	}
	reviewCtx.Rules, err = r.renderer.LoadRules()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load rules: %v\n", err)
	}

	return r.renderer.RenderThoroughReview(reviewCtx)
}

func (r *cliPromptRenderer) RenderExplore(input *pipeline.ExplorePromptInput) (string, error) {
	return "", fmt.Errorf("not implemented")
}

// cliBacklogClient adapts bead operations to pipeline.BacklogClient interface
type cliBacklogClient struct {
	beadClient *bead.Client
}

func (c *cliBacklogClient) List() ([]*pipeline.Idea, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *cliBacklogClient) Get(id string) (*pipeline.Idea, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *cliBacklogClient) Add(item *pipeline.Idea) error {
	// Create a backlog bead with P2 priority and backlog label
	labels := []string{"from-review", "backlog"}
	_, err := c.beadClient.Create(item.Text, 2, labels, nil)
	return err
}

func (c *cliBacklogClient) Update(id string, fn func(*pipeline.Idea)) error {
	return fmt.Errorf("not implemented")
}

// cliLearningsManager adapts learnings operations to pipeline.LearningsManager interface
type cliLearningsManager struct {
	gromitDir    string
	claudeClient *claude.Client
}

func (m *cliLearningsManager) Add(content string) error {
	learningsFile, err := learnings.NewFile(m.gromitDir)
	if err != nil {
		return err
	}

	// Wire filter into learnings file
	if m.claudeClient != nil {
		claudeRunner := &cliClaudeRunner{client: m.claudeClient}
		learningsFile.SetFilter(learnings.NewLLMFilter(claudeRunner, "gromit", learnings.ProjectDescriptions.Gromit))
	}

	if err := learningsFile.Load(); err != nil {
		return err
	}

	learningsFile.Add("review", content, learnings.CategoryPatterns)
	return nil
}

// cliClaudeRunner implements learnings.ClaudeRunner interface for the Claude client
type cliClaudeRunner struct {
	client *claude.Client
}

func (r *cliClaudeRunner) Run(ctx context.Context, prompt string, model string) (*learnings.Result, error) {
	if r.client == nil {
		return nil, fmt.Errorf("claude client is nil")
	}

	claudeResult, err := r.client.Run(ctx, prompt, model)
	if err != nil {
		return nil, err
	}

	if claudeResult == nil {
		return nil, fmt.Errorf("claude returned nil result")
	}

	// Convert claude.Result to learnings.Result
	return &learnings.Result{
		Success: claudeResult.Success,
		Output:  claudeResult.Output,
	}, nil
}

// cliLogWriter adapts logger operations to pipeline.LogWriter interface
type cliLogWriter struct {
	logsDir string
}

func (w *cliLogWriter) Write(entry any) error {
	log, err := logger.NewLogger(w.logsDir)
	if err != nil {
		return err
	}
	defer log.Close()

	logEntry, ok := entry.(*pipeline.LogEntry)
	if !ok {
		return fmt.Errorf("unexpected entry type")
	}

	// Use model from entry or default to opus
	model := logEntry.Model
	if model == "" {
		model = "opus" // Default for thorough reviews
	}

	reviewLog := &logger.ReviewLog{
		Timestamp:      time.Now(),
		Type:           "review",
		ReviewType:     "thorough-cli",
		Iteration:      0,
		Model:          model,
		Passed:         logEntry.Passed,
		FixesApplied:   logEntry.FixesApplied,
		BeadsCreated:   logEntry.BeadsCreated,
		BacklogCreated: logEntry.BacklogCreated,
		DurationMs:     0, // Duration tracked by caller if needed
	}

	return log.LogReview(reviewLog)
}

// cliStateManager adapts state operations to pipeline.StateManager interface
type cliStateManager struct {
	gromitDir string
}

func (m *cliStateManager) GetLastReviewCommit() (string, error) {
	sf, err := state.NewInteractiveFile(m.gromitDir)
	if err != nil {
		return "", err
	}

	if err := sf.Load(); err != nil {
		return "", err
	}

	return sf.LastReviewCommit(), nil
}

func (m *cliStateManager) SetLastReviewCommit(commit string) error {
	sf, err := state.NewInteractiveFile(m.gromitDir)
	if err != nil {
		return err
	}

	if err := sf.Load(); err != nil {
		return err
	}

	// Get current HEAD commit
	currentCommit, err := getGitHeadForReview()
	if err != nil {
		currentCommit = commit // Fallback to input commit
	}

	return sf.RecordReview(currentCommit, 0)
}
