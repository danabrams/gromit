package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
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

// reviewGitCommandFn is the injectable function for constructing git subcommands
// used by review helpers. Tests may replace this to avoid real subprocess calls.
var reviewGitCommandFn = exec.Command

// reviewGitOutputFn is the injectable function for executing a command and
// capturing its stdout. Tests may replace this to return synthetic output.
var reviewGitOutputFn = func(cmd *exec.Cmd) ([]byte, error) {
	return cmd.Output()
}

const defaultThoroughReviewTimeoutSeconds = 900
const (
	reviewSessionCommand       = "review"
	reviewGitDiffErrPrefix     = "git diff"
	reviewGitDiffStatErrPrefix = "git diff --stat"
	reviewGitHeadErrPrefix     = "git rev-parse HEAD"
	reviewGitHashFormatArg     = "--format=%H"
	reviewSummaryBannerWidth   = 80
	reviewSummaryTitle         = "REVIEW SUMMARY"
	reviewCompletionMessage    = "Review complete!"
	reviewLogType              = "review"
	reviewLogReviewType        = "thorough-cli"
	reviewDefaultModel         = "opus"
)

func runReviewGitOutput(args ...string) ([]byte, error) {
	cmd := reviewGitCommandFn("git", args...)
	return reviewGitOutputFn(cmd)
}

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

var reviewInteractiveSessionLauncherFn = runWithSessionWorktreeWithConflictSettings
var reviewInteractiveRunnerFn = runReviewInteractiveInDir
var recordInteractiveReviewCompletionFn = recordInteractiveReviewCompletion

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

// createReviewPipeline creates a minimal pipeline instance for scope resolution.
// This pipeline only needs StateManager for fallback to state file.
var createReviewPipeline = func(cfg *config.Config, gromitDir string) (ReviewScopeResolver, error) {
	deps, err := NewPipelineDeps(cfg, gromitDir)
	if err != nil {
		return nil, fmt.Errorf("constructing pipeline deps: %w", err)
	}

	paths := &pipeline.Paths{
		GromitDir: gromitDir,
	}

	return pipeline.New(deps, paths), nil
}

// ReviewScopeResolver abstracts the pipeline's scope resolution capability
type ReviewScopeResolver interface {
	ResolveReviewScope(ctx context.Context, spec string, epic string, since string) (string, error)
}

func resolveReviewScopeWithPipeline(resolver ReviewScopeResolver, cfg *config.Config) (string, error) {
	ctx := context.Background()

	// First try Pipeline.ResolveReviewScope for --since flag
	commit, err := resolver.ResolveReviewScope(ctx, reviewSpec, reviewEpic, reviewSince)
	if err != nil {
		return "", err
	}

	return commit, nil
}

func validateReviewFlags() error {
	return scope.ValidateFlags(reviewEpic, reviewSpec, reviewSince)
}

func runReview(cmd *cobra.Command, args []string) error {
	if err := validateReviewFlags(); err != nil {
		return err
	}

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Determine scope (from commit) using Pipeline
	gromitDir := resolveGromitDir(cfg)
	p, err := createReviewPipeline(cfg, gromitDir)
	if err != nil {
		return fmt.Errorf("creating pipeline for scope resolution: %w", err)
	}

	fromCommit, err := resolveReviewScopeWithPipeline(p, cfg)
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

func resolveReviewRendererPaths(cfg *config.Config) (string, string, string) {
	return resolveTemplatesDir(cfg), resolveSpecsDir(cfg), resolveProjectClaudeMD(cfg)
}

func resolveReviewNonInteractiveTimeout(cfg *config.Config) int {
	if cfg == nil {
		return defaultThoroughReviewTimeoutSeconds
	}
	timeout := cfg.Review.Thorough.Timeout
	if timeout <= 0 {
		return defaultThoroughReviewTimeoutSeconds
	}
	return timeout
}

func getGitDiffForReview(fromCommit string) (string, error) {
	return runGitDiffForReview(fromCommit, reviewGitDiffErrPrefix)
}

func getGitDiffStatForReview(fromCommit string) (string, error) {
	return runGitDiffForReview(fromCommit, reviewGitDiffStatErrPrefix, "--stat")
}

func runGitDiffForReview(fromCommit string, errPrefix string, args ...string) (string, error) {
	if err := validateCommitRef(fromCommit); err != nil {
		return "", err
	}
	diffArgs := append([]string{"diff"}, args...)
	diffArgs = append(diffArgs, fromCommit, "--")
	out, err := runReviewGitOutput(diffArgs...)
	if err != nil {
		return "", fmt.Errorf("%s: %w", errPrefix, err)
	}
	return string(out), nil
}

func runReviewInteractive(cfg *config.Config, fromCommit string, diff string) error {
	// Print status message
	fmt.Printf("Launching interactive review session (from commit %s)...\n", shortCommit(fromCommit))

	gromitDir := resolveGromitDir(cfg)

	if err := launchInSessionIfEnabled(cfg, gromitDir, reviewSessionCommand, reviewInteractiveSessionLauncherFn, func(sessionDir string) error {
		return reviewInteractiveRunnerFn(cfg, fromCommit, diff, sessionDir)
	}, func() error {
		return reviewInteractiveRunnerFn(cfg, fromCommit, diff, "")
	}); err != nil {
		return err
	}

	if err := recordInteractiveReviewCompletionFn(gromitDir, fromCommit); err != nil {
		return err
	}

	return nil
}

func runReviewInteractiveInDir(cfg *config.Config, fromCommit string, diff string, launchDir string) error {
	gromitDir := resolveGromitDir(cfg)

	// Construct pipeline dependencies using dependency injection
	deps, err := NewPipelineDeps(cfg, gromitDir)
	if err != nil {
		return fmt.Errorf("constructing pipeline deps: %w", err)
	}

	// Create prompt renderer for interactive review
	templatesDir, specsDir, claudeMDPath := resolveReviewRendererPaths(cfg)
	renderer, err := prompt.NewRenderer(templatesDir, specsDir, claudeMDPath, gromitDir)
	if err != nil {
		return fmt.Errorf("creating renderer: %w", err)
	}

	// Override review-specific renderer
	promptRendererAdapter := &cliPromptRenderer{
		renderer: renderer,
	}
	deps.ReviewRenderer = promptRendererAdapter

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
		LaunchDir:  launchDir,
	}

	// Call pipeline
	// ReviewInteractive returns a session that owns the temp prompt file cleanup.
	// For synchronous mode, we clean up after LaunchInDir completes.
	// For async mode, cleanup will be handled by the session owner (e.g., worktree manager).
	session, err := p.ReviewInteractive(context.Background(), input)
	if err != nil {
		return fmt.Errorf("review interactive: %w", err)
	}

	// Clean up temp file now that LaunchInDir has completed (synchronous mode)
	// In async mode, the session owner would manage this lifecycle
	if session != nil {
		session.Cleanup()
	}

	return nil
}

func runReviewNonInteractive(cfg *config.Config, fromCommit string, diff string) error {
	fmt.Printf("Running autonomous thorough review (from commit %s)...\n", shortCommit(fromCommit))

	// Build pipeline and dependencies
	gromitDir := resolveGromitDir(cfg)

	// Construct pipeline dependencies using dependency injection
	deps, err := NewPipelineDeps(cfg, gromitDir)
	if err != nil {
		return fmt.Errorf("constructing pipeline deps: %w", err)
	}

	// Override the LLM client to use review-specific configuration (providers, timeout)
	llmClient, err := buildReviewNonInteractiveClient(cfg)
	if err != nil {
		return fmt.Errorf("creating review invoker: %w", err)
	}
	deps.LLMClient = llmClient
	deps.ReviewInvoker = llmClient

	// Create prompt renderer for diagnostics
	templatesDir, specsDir, claudeMDPath := resolveReviewRendererPaths(cfg)
	renderer, err := prompt.NewRenderer(templatesDir, specsDir, claudeMDPath, gromitDir)
	if err != nil {
		return fmt.Errorf("creating renderer: %w", err)
	}

	// Override review-specific renderers and adapters
	reviewRendererAdapter := &cliPromptRenderer{
		renderer: renderer,
	}
	deps.ReviewRenderer = reviewRendererAdapter

	// Inject diagnostics provider from renderer
	SetDepsPromptDiagnosticsProvider(deps, func() *prompt.PromptDiagnostics {
		return renderer.LastDiagnostics()
	})

	paths := &pipeline.Paths{
		GromitDir: gromitDir,
	}

	p := pipeline.New(deps, paths)

	// Prepare input
	timeoutSeconds := resolveReviewNonInteractiveTimeout(cfg)
	timeout := time.Duration(timeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	input := pipeline.ReviewInput{
		FromCommit: fromCommit,
		Diff:       diff,
		Model:      cfg.Review.Thorough.Model,
		Timeout:    timeoutSeconds,
	}

	// Call pipeline
	result, err := p.ReviewNonInteractive(ctx, input)
	if err != nil {
		return fmt.Errorf("review non-interactive: %w", err)
	}

	// Display output
	fmt.Println("\n" + strings.Repeat("=", reviewSummaryBannerWidth))
	fmt.Println(reviewSummaryTitle)
	fmt.Println(strings.Repeat("=", reviewSummaryBannerWidth))
	fmt.Println(result.Summary)
	fmt.Println()
	printReviewSummaryCounts(os.Stdout, result)

	fmt.Println("\n" + reviewCompletionMessage)
	return nil
}

func buildReviewNonInteractiveClient(cfg *config.Config) (pipeline.LLMClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	timeout := time.Duration(cfg.Claude.PipelineTimeout) * time.Second
	if cfg.HasProviders() {
		router, err := provider.BuildRouterFromConfig(cfg)
		if err != nil {
			return nil, err
		}
		return &llmRouterClientAdapter{
			Router:  router,
			Timeout: timeout,
			Phase:   reviewSessionCommand,
		}, nil
	}

	claudeClient, err := claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, cfg.Claude.Timeout)
	if err != nil {
		return nil, err
	}

	return &claudeClientAdapter{
		Client:  claudeClient,
		Timeout: timeout,
	}, nil
}

func printReviewSummaryCounts(w io.Writer, result *pipeline.ReviewResult) {
	if w == nil || result == nil {
		return
	}
	if result.FixesApplied > 0 {
		fmt.Fprintf(w, "Fixes applied: %d\n", result.FixesApplied)
	}
	if result.BeadsCreated > 0 {
		fmt.Fprintf(w, "Created %d beads from review findings\n", result.BeadsCreated)
	}
	if result.BacklogCreated > 0 {
		fmt.Fprintf(w, "Created %d backlog items\n", result.BacklogCreated)
	}
}

func getGitHeadForReview() (string, error) {
	out, err := runReviewGitOutput("rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("%s: %w", reviewGitHeadErrPrefix, err)
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

func recordInteractiveReviewCompletion(gromitDir, fromCommit string) error {
	sf, err := state.NewInteractiveFile(gromitDir)
	if err != nil {
		return fmt.Errorf("creating state file: %w", err)
	}
	if err := sf.Load(); err != nil {
		return fmt.Errorf("loading state file: %w", err)
	}

	stateAdapter := &cliStateManager{
		stateFile: sf,
	}
	if err := stateAdapter.SetLastReviewCommit(fromCommit); err != nil {
		return fmt.Errorf("recording review completion: %w", err)
	}
	return nil
}

func reviewRepoDirFromGromitDir(gromitDir string) string {
	clean := filepath.Clean(gromitDir)
	if filepath.Base(clean) == ".gromit" {
		return filepath.Dir(clean)
	}
	return clean
}
