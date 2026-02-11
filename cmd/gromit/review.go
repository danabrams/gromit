package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/review"
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

Interactive mode (default): Launches a Claude Code session for collaborative review.
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
		return getSpecBaseCommit(reviewSpec, cfg.Paths.Specs)
	}

	if reviewEpic != "" {
		// Find the earliest commit from beads in this epic
		gromitDir := resolveGromitDir(cfg)
		return getEpicBaseCommit(reviewEpic, cfg.Paths.Specs, gromitDir)
	}

	// Default: use last review commit from state
	gromitDir := resolveGromitDir(cfg)

	sf, err := state.NewFile(gromitDir)
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

func getSpecBaseCommit(specName string, specsDir string) (string, error) {
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
	beadsClient, err := bead.NewClient()
	if err != nil {
		return "", fmt.Errorf("creating bead client: %w", err)
	}

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
	// Search git log for the bead ID in commit messages
	cmd := exec.Command("git", "log", "--all", "--format=%H", "--grep", beadID)
	out, err := cmd.Output()
	if err != nil {
		return "", nil // No commits found
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > 0 && lines[0] != "" {
		// git log returns newest first, so the last line is the earliest commit
		return lines[len(lines)-1], nil
	}

	return "", nil
}

func isCommitEarlier(commit1, commit2 string) bool {
	// Compare commit timestamps to determine which is earlier
	cmd1 := exec.Command("git", "log", "-1", "--format=%at", commit1)
	out1, err1 := cmd1.Output()
	if err1 != nil {
		return false
	}

	cmd2 := exec.Command("git", "log", "-1", "--format=%at", commit2)
	out2, err2 := cmd2.Output()
	if err2 != nil {
		return false
	}

	timestamp1Str := strings.TrimSpace(string(out1))
	timestamp2Str := strings.TrimSpace(string(out2))

	timestamp1, err := strconv.ParseInt(timestamp1Str, 10, 64)
	if err != nil {
		return false
	}

	timestamp2, err := strconv.ParseInt(timestamp2Str, 10, 64)
	if err != nil {
		return false
	}

	return timestamp1 < timestamp2
}

// findEarliestCommitFromBeads iterates through beads and returns the earliest commit found.
// Returns empty string if no commits are found.
func findEarliestCommitFromBeads(beads []*bead.Bead) string {
	earliestCommit := ""
	for _, b := range beads {
		commit, err := findFirstCommitForBead(b.ID)
		if err != nil {
			// Skip beads that don't have commits
			continue
		}
		if commit != "" {
			if earliestCommit == "" {
				earliestCommit = commit
			} else {
				if isCommitEarlier(commit, earliestCommit) {
					earliestCommit = commit
				}
			}
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
	// Build and render prompt
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

	reviewCtx := &prompt.ThoroughReviewContext{
		Diff:  diff,
		Model: cfg.Review.Thorough.Model,
	}
	reviewCtx.ClaudeMD, err = renderer.LoadClaudeMD()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load CLAUDE.md: %v\n", err)
	}
	reviewCtx.Rules, err = renderer.LoadRules()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load rules: %v\n", err)
	}

	renderedPrompt, err := renderer.RenderThoroughReview(reviewCtx)
	if err != nil {
		return fmt.Errorf("rendering review prompt: %w", err)
	}

	// Write prompt to a temp file to avoid "argument list too long" errors
	// when the diff is large
	tmpDir := filepath.Join(gromitDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return fmt.Errorf("creating tmp dir: %w", err)
	}

	promptFile, err := os.CreateTemp(tmpDir, "review-prompt-*.md")
	if err != nil {
		return fmt.Errorf("creating temp prompt file: %w", err)
	}
	promptPath := promptFile.Name()
	defer os.Remove(promptPath)

	if _, err := promptFile.WriteString(renderedPrompt); err != nil {
		promptFile.Close()
		return fmt.Errorf("writing prompt file: %w", err)
	}
	promptFile.Close()

	// Launch interactive review session with agent selection
	fmt.Printf("Launching interactive review session (from commit %s)...\n", shortCommit(fromCommit))

	// Resolve which agent to use
	selectedAgent, err := agent.Resolve(cfg, "review", reviewAgent, reviewChooseAgent, os.Stdin, os.Stdout)
	if err != nil {
		return fmt.Errorf("resolving agent: %w", err)
	}

	// Launch the agent with the prompt file
	if err := selectedAgent.Launch(promptPath); err != nil {
		return fmt.Errorf("launching agent: %w", err)
	}

	return nil
}

func runReviewNonInteractive(cfg *config.Config, fromCommit string, diff string) error {
	fmt.Printf("Running autonomous thorough review (from commit %s)...\n", shortCommit(fromCommit))

	// Build context
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

	reviewCtx := &prompt.ThoroughReviewContext{
		Diff:  diff,
		Model: cfg.Review.Thorough.Model,
	}
	reviewCtx.ClaudeMD, err = renderer.LoadClaudeMD()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load CLAUDE.md: %v\n", err)
	}
	reviewCtx.Rules, err = renderer.LoadRules()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load rules: %v\n", err)
	}

	renderedPrompt, err := renderer.RenderThoroughReview(reviewCtx)
	if err != nil {
		return fmt.Errorf("rendering review prompt: %w", err)
	}

	// Call Claude
	claudeClient, err := claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, cfg.Claude.Timeout)
	if err != nil {
		return fmt.Errorf("creating claude client: %w", err)
	}

	timeout := time.Duration(cfg.Review.Thorough.Timeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	model := cfg.Review.Thorough.Model
	claudeResult, err := claudeClient.Run(ctx, renderedPrompt, model)
	if err != nil {
		return fmt.Errorf("review invocation: %w", err)
	}
	if claudeResult == nil {
		return fmt.Errorf("review returned nil result")
	}

	// Parse result
	result, err := review.ParseReviewResult(claudeResult.Output)
	if err != nil {
		return fmt.Errorf("parsing review result: %w", err)
	}

	// Display summary
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("REVIEW SUMMARY")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println(result.Summary)
	fmt.Println()

	if len(result.FixesApplied) > 0 {
		fmt.Printf("Fixes applied: %d\n", len(result.FixesApplied))
		for _, fix := range result.FixesApplied {
			fmt.Printf("  - %s\n", fix)
		}
		fmt.Println()
	}

	// Apply results (create beads)
	beadsCreated, backlogCreated := applyReviewResultCLI(result)

	if beadsCreated > 0 {
		fmt.Printf("Created %d beads from review findings\n", beadsCreated)
	}
	if backlogCreated > 0 {
		fmt.Printf("Created %d backlog items\n", backlogCreated)
	}

	// Persist learnings
	persistReviewLearnings(gromitDir, result.Learnings, claudeClient)

	// Log the review
	log, err := logger.NewLogger(cfg.Paths.Logs)
	if err == nil {
		log.LogReview(&logger.ReviewLog{
			Timestamp:      time.Now(),
			Type:           "review",
			ReviewType:     "thorough-cli",
			Iteration:      0,
			Model:          model,
			Passed:         result.Passed,
			FixesApplied:   len(result.FixesApplied),
			BeadsCreated:   beadsCreated,
			BacklogCreated: backlogCreated,
			DurationMs:     int64(claudeResult.Duration.Milliseconds()),
		})
		log.Close()
	}

	// Update state
	sf, err := state.NewFile(gromitDir)
	if err == nil {
		sf.Load()
		currentCommit, err := getGitHeadForReview()
		if err == nil {
			sf.RecordReview(currentCommit, 0)
		}
	}

	fmt.Println("\nReview complete!")
	return nil
}

func applyReviewResultCLI(result *review.ReviewResult) (beadsCreated int, backlogCreated int) {
	if result == nil {
		return 0, 0
	}

	beadsClient, err := bead.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create bead client: %v\n", err)
		return 0, 0
	}

	for _, bp := range result.BeadsToCreate {
		labels := buildReviewBeadLabelsCLI(bp.Labels)
		_, err := beadsClient.Create(bp.Title, bp.Priority, labels, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create review bead: %v\n", err)
			continue
		}
		beadsCreated++
		fmt.Printf("Created bead: %s (P%d)\n", bp.Title, bp.Priority)
	}

	for _, bi := range result.BacklogItems {
		labels := buildBacklogLabelsCLI()
		_, err := beadsClient.Create(bi.Title, 2, labels, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create backlog bead: %v\n", err)
			continue
		}
		backlogCreated++
		fmt.Printf("Created backlog: %s (reason: %s)\n", bi.Title, bi.Reason)
	}

	// Persist learnings
	if len(result.Learnings) > 0 {
		cfg, err := loadConfig()
		if err == nil {
			gromitDir := resolveGromitDir(cfg)
			claudeClient, err := claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, cfg.Claude.Timeout)
			if err == nil {
				persistReviewLearnings(gromitDir, result.Learnings, claudeClient)
			}
		}
	}

	return beadsCreated, backlogCreated
}

// persistReviewLearnings adds review-discovered learnings to the learnings file.
func persistReviewLearnings(gromitDir string, reviewLearnings []string, claudeClient *claude.Client) {
	if len(reviewLearnings) == 0 {
		return
	}

	learningsFile, err := learnings.NewFile(gromitDir)
	if err != nil {
		return
	}

	// Wire filter into learnings file
	if claudeClient != nil {
		claudeRunnerAdapter := learnings.NewClaudeRunnerAdapter(claudeClient)
		learningsFile.SetFilter(learnings.NewLLMFilter(claudeRunnerAdapter, "gromit", learnings.ProjectDescriptions.Gromit))
	}

	if err := learningsFile.Load(); err != nil {
		return
	}

	for _, learning := range reviewLearnings {
		learningsFile.Add("review", learning, learnings.CategoryPatterns)
	}
}

func buildReviewBeadLabelsCLI(proposalLabels []string) []string {
	labels := []string{"from-review"}
	for _, l := range proposalLabels {
		if l != "from-review" {
			labels = append(labels, l)
		}
	}
	return labels
}

func buildBacklogLabelsCLI() []string {
	return []string{"from-review", "backlog"}
}

func getGitHeadForReview() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func shortCommit(commit string) string {
	if len(commit) > 8 {
		return commit[:8]
	}
	return commit
}
