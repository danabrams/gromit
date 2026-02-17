package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/state"
)

var requiredQualityGateCommands = []string{"go test", "go vet", "go build"}

// checkRetroSuggestion checks if a retro should be suggested and prints a message
func (r *Runner) checkRetroSuggestion() {
	if r.cfg == nil {
		return
	}
	// Load learnings
	lf, err := learnings.NewFile(r.gromitDir)
	if err != nil {
		return // Silently skip if learnings can't be created
	}
	if err := lf.Load(); err != nil {
		return // Silently skip if learnings can't be loaded
	}

	// Load interactive state for last retro time
	interactiveFile, err := state.NewInteractiveFile(r.gromitDir)
	if err != nil {
		return // Silently skip if interactive state can't be created
	}
	if err := interactiveFile.Load(); err != nil {
		return // Silently skip if interactive state can't be loaded
	}

	// Compute failure rate from logs
	stats, err := logger.ReadAllLogs(r.cfg.Paths.Logs)
	if err != nil {
		stats = logger.RunStats{} // Use zero stats on error
	}

	should, reason := lf.ShouldSuggestRetro(interactiveFile.LastRetro(), stats.FailureRate())
	if !should {
		return
	}

	confirmedCount, provisionalCount := lf.Stats()
	r.log("\nRetro suggested: %d provisional learnings, %d confirmed patterns (%s). Run: gromit retro",
		provisionalCount, confirmedCount, reason)
}

// isStuckBeadWithStats checks if a bead has failed too many times across runs
// using pre-loaded bead statistics (call ReadPerBeadStats once before the loop for efficiency)
func (r *Runner) isStuckBeadWithStats(b *bead.Bead, beadStats map[string]logger.BeadStats) bool {
	if r == nil || b == nil || r.cfg == nil {
		return false
	}

	// If threshold is 0 or negative, stuck-bead detection is disabled
	if r.cfg.Loop.StuckBeadThreshold <= 0 {
		return false
	}

	stats, exists := beadStats[b.ID]
	if !exists {
		// No history for this bead, not stuck
		return false
	}

	// Mark as stuck if failures >= threshold
	return stats.Failures >= r.cfg.Loop.StuckBeadThreshold
}

// Status returns the current queue status
func (r *Runner) Status() error {
	if r == nil {
		return fmt.Errorf("runner is nil")
	}

	// Read status.json
	status, err := ReadStatus(r.gromitDir)
	if err != nil {
		return fmt.Errorf("reading status: %w", err)
	}

	// Check if status is stale (process not alive)
	if status != nil && status.Running && !IsProcessAlive(status.PID) {
		elapsed := time.Since(status.StartedAt)
		r.log("Warning: stale run detected from %s (%s ago)",
			status.StartedAt.Format(time.RFC3339),
			elapsed.Round(time.Second))
		r.log("  Bead: %s - %s", status.BeadID, status.BeadTitle)
		r.log("  Removing stale status file")

		// Delete the stale file
		sw, err := NewStatusWriter(r.gromitDir)
		if err == nil {
			_ = sw.Delete() // Ignore error - we'll proceed anyway
		}
		r.log("")
		status = nil // Treat as if no status exists
	}

	// Read pipeline status
	// Pass startedAt from status.json if available for "closed this run" count
	var startedAt *time.Time
	if status != nil && !status.StartedAt.IsZero() {
		startedAt = &status.StartedAt
	}
	pipelineStatus, err := pipeline.ReadStatus(r.gromitDir, r.cfg.Paths.Specs, r.cfg.Paths.Plans, startedAt)
	if err != nil {
		return fmt.Errorf("reading pipeline status: %w", err)
	}

	// Load state file for health data
	stateFile, err := state.NewFile(r.gromitDir)
	if err != nil {
		return fmt.Errorf("creating state file: %w", err)
	}
	if err := stateFile.Load(); err != nil {
		return fmt.Errorf("loading state file: %w", err)
	}

	// Load interactive state file for last retro data
	interactiveFile, err := state.NewInteractiveFile(r.gromitDir)
	if err != nil {
		return fmt.Errorf("creating interactive state file: %w", err)
	}
	if err := interactiveFile.Load(); err != nil {
		return fmt.Errorf("loading interactive state file: %w", err)
	}

	// Read model performance stats
	modelStats, err := logger.ReadModelStats(r.cfg.Paths.Logs)
	if err != nil {
		// Log warning but continue - model stats are informational only
		r.log("Warning: could not read model stats: %v", err)
		modelStats = make(map[string]logger.ModelStats)
	}

	// Format and print all sections
	r.log("%s", formatPipeline(pipelineStatus))
	r.log("")
	r.log("%s", formatRun(status))
	r.log("")
	r.log("%s", formatHealth(interactiveFile.LastRetro(), stateFile.IterationsSinceReview()))
	r.log("")
	r.log("%s", formatModelPerformance(modelStats))
	r.log("")
	r.log("%s", formatRecommendation(pipelineStatus.Recommendation))

	return nil
}

// runGitAutoPush pushes the current branch to its upstream tracking ref after bead completion.
// If auto_push is disabled, does nothing. If push fails and push_failure is "warn", logs a warning
// and continues. If push fails and push_failure is "stop", returns an error to halt the loop.
func (r *Runner) runGitAutoPush() error {
	if r == nil || r.cfg == nil {
		return nil
	}
	if !r.cfg.Git.IsAutoPushEnabled() {
		return nil
	}

	r.log("Pushing to remote...")
	_, stderr, exitCode, err := r.runCmd(context.Background(), "git push", "")
	if err != nil {
		if r.cfg.Git.PushFailure == "stop" {
			return fmt.Errorf("git push failed: %w", err)
		}
		r.log("Warning: git push failed: %v", err)
		return nil
	}
	if exitCode != 0 {
		if r.cfg.Git.PushFailure == "stop" {
			return fmt.Errorf("git push failed (exit %d): %s", exitCode, stderr)
		}
		r.log("Warning: git push failed (exit %d): %s", exitCode, stderr)
		return nil
	}

	return nil
}

func (r *Runner) mergeInteractiveBranches() error {
	if r == nil || r.cfg == nil {
		return nil
	}
	if !r.cfg.Worktree.IsEnabled() || !r.cfg.Worktree.IsAutoMergeEnabled() {
		return nil
	}
	if r.worktreeManager == nil {
		return nil
	}

	branches, err := r.worktreeManager.PendingBranches()
	if err != nil {
		return r.handleMergeFailure(fmt.Errorf("list pending branches: %w", err))
	}

	for _, branch := range branches {
		if err := r.worktreeManager.MergeBack(branch); err != nil {
			if err := r.handleMergeFailure(fmt.Errorf("merge back %s: %w", branch, err)); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *Runner) handleMergeFailure(err error) error {
	if r == nil || r.cfg == nil {
		return err
	}
	if strings.EqualFold(r.cfg.Worktree.MergeFailure, "stop") {
		return err
	}
	r.log("Warning: merge back failed: %v", err)
	return nil
}

// runBetweenIterationsCommand runs the user-configured command between iterations.
// If the command is empty, does nothing. If the command fails, logs a warning but does not
// stop the loop or return an error.
func (r *Runner) runBetweenIterationsCommand() {
	if r == nil || r.cfg == nil {
		return
	}
	command := r.cfg.Loop.BetweenIterationsCommand
	if command == "" {
		return
	}

	r.log("Running between-iterations command: %s", command)
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = r.output
	cmd.Stderr = r.output

	if err := cmd.Run(); err != nil {
		r.log("Warning: between-iterations command failed: %v", err)
	}
}

// SetLabelFilters sets optional spec labels to filter beads by
func (r *Runner) SetLabelFilters(labels []string) {
	r.labelFilters = labels
}

// getNextBead gets the next bead to process, optionally filtering by labels
func (r *Runner) getNextBead() (*bead.Bead, error) {
	if r == nil || r.beads == nil {
		return nil, fmt.Errorf("runner or beads client is nil")
	}

	// If no label filters, use Ready() as before
	if len(r.labelFilters) == 0 {
		return r.beads.Ready()
	}

	// Collect beads from all labels
	var candidates []*bead.Bead
	for _, label := range r.labelFilters {
		b, err := r.beads.ReadyWithLabel(label)
		if err != nil {
			return nil, fmt.Errorf("getting bead with label %s: %w", label, err)
		}
		if b != nil {
			candidates = append(candidates, b)
		}
	}

	// If no beads found for any label
	if len(candidates) == 0 {
		return nil, nil
	}

	// Return the highest priority bead (lowest priority number)
	highestPriority := candidates[0]
	for _, b := range candidates[1:] {
		if b.Priority < highestPriority.Priority {
			highestPriority = b
		}
	}

	return highestPriority, nil
}

// updateGlobalStats reads current run's model stats and merges them into ~/.gromit/stats.json
func (r *Runner) updateGlobalStats() {
	// Get current run ID from logger
	runID := r.logger.RunID()
	if runID == "" {
		r.log("Warning: could not determine run ID for global stats update")
		return
	}

	// Read model stats for current run
	runStats, err := logger.ReadRunModelStats(r.cfg.Paths.Logs, runID)
	if err != nil {
		r.log("Warning: could not read run model stats for global stats update: %v", err)
		return
	}

	// Resolve global stats path using user home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		r.log("Warning: could not determine user home directory for global stats: %v", err)
		return
	}
	globalStatsPath := filepath.Join(homeDir, ".gromit", "stats.json")

	// Update global stats
	if err := logger.UpdateGlobalStats(globalStatsPath, runStats); err != nil {
		r.log("Warning: could not update global stats: %v", err)
		return
	}
}

func (r *Runner) enforceMandatoryQualityGateCoverage(gateName string, commands []string) error {
	if r == nil || r.cfg == nil || !r.cfg.Validation.Enabled {
		return nil
	}

	missing := make([]string, 0, len(requiredQualityGateCommands))
	for _, required := range requiredQualityGateCommands {
		if !hasQualityGateCommand(commands, required) {
			missing = append(missing, required)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	return fmt.Errorf("%s quality gate missing mandatory commands: %s", gateName, strings.Join(missing, ", "))
}

func hasQualityGateCommand(commands []string, requiredPrefix string) bool {
	for _, cmd := range commands {
		if strings.HasPrefix(strings.TrimSpace(cmd), requiredPrefix) {
			return true
		}
	}
	return false
}
