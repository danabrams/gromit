package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/spf13/cobra"
)

// unwrapAll recursively unwraps an error chain to find the root cause.
func unwrapAll(err error) error {
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return err
		}
		err = unwrapped
	}
}

// newExecShowCmd creates the `exec show` command.
func newExecShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [run-id]",
		Short: "Show a human-readable summary of a run",
		Long: `Show a human-readable summary of a run. If no run-id is given,
the latest run is shown (by modification time of .gromit-next/runs/).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			full, _ := cmd.Flags().GetBool("full")
			storeDir, _ := cmd.Flags().GetString("store-dir")
			if storeDir == "" {
				storeDir = ".gromit-next"
			}

			store := runstore.NewStore(storeDir)

			var runID string
			if len(args) == 0 || args[0] == "latest" {
				id, err := findLatestRunID(storeDir, project, store)
				if err != nil {
					return err
				}
				runID = id
			} else {
				runID = args[0]
			}

			output, err := execShow(runID, store, full)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), output)
			return nil
		},
	}
	cmd.Flags().String("project", "", "Project name (used with 'latest' to filter)")
	cmd.Flags().Bool("full", false, "Print complete evidence bundle contents")
	cmd.Flags().String("store-dir", "", "Override store directory (for testing)")
	return cmd
}

// findLatestRunID finds the most recent run. If projectID is set, it uses the
// store's List method. Otherwise, it lists .gromit-next/runs/ sorted by mtime.
func findLatestRunID(storeDir string, projectID string, store *runstore.Store) (string, error) {
	if projectID != "" {
		return resolveRunID("latest", projectID, store)
	}
	// List all run directories sorted by modification time (most recent first).
	runsDir := filepath.Join(storeDir, "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return "", fmt.Errorf("list runs directory: %w", err)
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("no runs found in %s", runsDir)
	}
	type dirEntry struct {
		name    string
		modTime time.Time
	}
	var dirs []dirEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, infoErr := e.Info()
		if infoErr != nil {
			continue
		}
		dirs = append(dirs, dirEntry{name: e.Name(), modTime: info.ModTime()})
	}
	if len(dirs) == 0 {
		return "", fmt.Errorf("no run directories found in %s", runsDir)
	}
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].modTime.After(dirs[j].modTime)
	})
	return dirs[0].name, nil
}

// resolveRunID resolves "latest" to the most recent run ID for a project.
func resolveRunID(id string, projectID string, store *runstore.Store) (string, error) {
	if id != "latest" {
		return id, nil
	}
	if projectID == "" {
		return "", fmt.Errorf("--project is required when using 'latest'")
	}
	runs, err := store.List(projectID)
	if err != nil {
		return "", fmt.Errorf("list runs: %w", err)
	}
	if len(runs) == 0 {
		return "", fmt.Errorf("no runs found for project %q", projectID)
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})
	return runs[0].RunID, nil
}

// execShow formats run details as a human-readable summary string.
func execShow(runID string, store *runstore.Store, full bool) (string, error) {
	rs, err := store.Get(runID)
	if err != nil {
		if os.IsNotExist(unwrapAll(err)) {
			return "", fmt.Errorf("run %q not found", runID)
		}
		return "", err
	}

	var b strings.Builder

	// --- Header ---
	fmt.Fprintf(&b, "Run: %s\n", rs.RunID)
	fmt.Fprintf(&b, "Spec: %s\n", rs.SpecID)
	if rs.ProjectID != "" {
		fmt.Fprintf(&b, "Project: %s\n", rs.ProjectID)
	}
	statusLine := rs.Status
	if rs.TerminalReason != "" {
		statusLine += " (" + rs.TerminalReason + ")"
	}
	fmt.Fprintf(&b, "Status: %s\n", statusLine)

	// Cost | Cycles | Duration
	var metaParts []string
	metaParts = append(metaParts, fmt.Sprintf("$%.2f", rs.AccumulatedCost))
	metaParts = append(metaParts, fmt.Sprintf("Cycles: %d", rs.Cycle))
	if !rs.EndedAt.IsZero() {
		dur := rs.EndedAt.Sub(rs.StartedAt)
		metaParts = append(metaParts, fmt.Sprintf("Duration: %s", formatDuration(dur)))
	}
	fmt.Fprintf(&b, "Cost: %s\n", strings.Join(metaParts, " | "))

	if rs.BlockerSummary != "" {
		fmt.Fprintf(&b, "Blocker: %s\n", rs.BlockerSummary)
	}

	// --- Tasks ---
	if len(rs.Tasks) > 0 {
		doneTasks := 0
		failedTasks := 0
		for _, t := range rs.Tasks {
			switch t.Status {
			case "done":
				doneTasks++
			case "failed":
				failedTasks++
			}
		}
		fmt.Fprintf(&b, "\nTasks (%d):\n", len(rs.Tasks))
		for _, t := range rs.Tasks {
			icon := statusIcon(t.Status)
			label := truncateObjective(t.Objective, 60)
			detail := formatTaskDetail(t)
			fmt.Fprintf(&b, "  %s %s: %s%s\n", icon, t.TaskID, label, detail)
		}
	}

	// --- Quality Gates ---
	fmt.Fprintf(&b, "\nQuality Gates:\n")
	fmt.Fprintf(&b, "  Validation: %s %s\n", passFailIcon(rs.FinalValidationPassed), passFailLabel(rs.FinalValidationPassed))
	fmt.Fprintf(&b, "  Review: %s %s\n", passFailIcon(rs.FinalReviewPassed), formatReviewGate(rs))
	fmt.Fprintf(&b, "  Acceptance: %s %s\n", passFailIcon(rs.FinalAcceptancePassed), formatAcceptanceGate(rs))

	// --- Review Findings ---
	if len(rs.ReviewFindings) > 0 {
		fmt.Fprintf(&b, "\nReview Findings:\n")
		for _, f := range rs.ReviewFindings {
			fmt.Fprintf(&b, "  %s\n", f)
		}
	}

	// --- Acceptance Results ---
	if len(rs.AcceptanceResults) > 0 {
		fmt.Fprintf(&b, "\nAcceptance Results:\n")
		for _, a := range rs.AcceptanceResults {
			fmt.Fprintf(&b, "  %s\n", a)
		}
	}

	// --- Suggested Actions ---
	actions := suggestedActions(rs)
	if len(actions) > 0 {
		fmt.Fprintf(&b, "\nSuggested Actions:\n")
		for _, a := range actions {
			fmt.Fprintf(&b, "  * %s\n", a)
		}
	}

	// --- Invocation count + paths ---
	if n := readInvocationCount(store.RunEvidenceDir(runID)); n >= 0 {
		fmt.Fprintf(&b, "\nInvocations: %d\n", n)
	}
	if rs.WorktreePath != "" {
		fmt.Fprintf(&b, "Worktree: %s\n", rs.WorktreePath)
	}
	fmt.Fprintf(&b, "Evidence: %s\n", store.RunEvidenceDir(runID))

	// --- Full evidence dump ---
	if full {
		evidenceDir := store.RunEvidenceDir(runID)
		entries, readErr := os.ReadDir(evidenceDir)
		if readErr == nil && len(entries) > 0 {
			fmt.Fprintf(&b, "\n--- Evidence ---\n")
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				path := filepath.Join(evidenceDir, entry.Name())
				data, fErr := os.ReadFile(path)
				if fErr != nil {
					continue
				}
				fmt.Fprintf(&b, "\n=== %s ===\n%s\n", entry.Name(), string(data))
			}
		}
	}

	return b.String(), nil
}

// formatDuration produces a human-friendly duration like "27m" or "1h12m".
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%dm", h, m)
}

// statusIcon returns a check or cross icon for a task status.
func statusIcon(status string) string {
	switch status {
	case "done":
		return "\u2713" // checkmark
	case "failed":
		return "\u2717" // cross
	default:
		return "-"
	}
}

// passFailIcon returns a check or cross icon for a boolean gate.
func passFailIcon(passed bool) string {
	if passed {
		return "\u2713"
	}
	return "\u2717"
}

// passFailLabel returns "passed" or "failed".
func passFailLabel(passed bool) string {
	if passed {
		return "passed"
	}
	return "failed"
}

// truncateObjective truncates a task objective to maxLen characters.
func truncateObjective(obj string, maxLen int) string {
	// Take first sentence or up to maxLen.
	if idx := strings.Index(obj, ". "); idx > 0 && idx < maxLen {
		return obj[:idx]
	}
	if len(obj) <= maxLen {
		return obj
	}
	return obj[:maxLen-3] + "..."
}

// formatTaskDetail builds the parenthetical detail for a task line.
func formatTaskDetail(t runstore.Task) string {
	var parts []string
	if t.Status == "failed" && t.Attempts > 1 {
		parts = append(parts, fmt.Sprintf("%d attempts", t.Attempts))
		parts = append(parts, "failed")
	} else if t.Status == "done" {
		if t.DurationMs > 0 {
			dur := time.Duration(t.DurationMs) * time.Millisecond
			parts = append(parts, formatDuration(dur))
		}
		if t.TokensUsed > 0 {
			parts = append(parts, fmt.Sprintf("%d tok", t.TokensUsed))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

// formatReviewGate formats the review gate line with finding counts.
func formatReviewGate(rs *runstore.RunState) string {
	if rs.FinalReviewPassed {
		return "passed"
	}
	label := "failed"
	if len(rs.ReviewFindings) > 0 {
		errorCount := 0
		warnCount := 0
		for _, f := range rs.ReviewFindings {
			fl := strings.ToLower(f)
			if strings.Contains(fl, "[error]") {
				errorCount++
			} else if strings.Contains(fl, "[warning]") || strings.Contains(fl, "[warn]") {
				warnCount++
			}
		}
		var countParts []string
		if errorCount > 0 {
			countParts = append(countParts, fmt.Sprintf("%d errors", errorCount))
		}
		if warnCount > 0 {
			countParts = append(countParts, fmt.Sprintf("%d warnings", warnCount))
		}
		if len(countParts) > 0 {
			label += " (" + strings.Join(countParts, ", ") + ")"
		}
	}
	return label
}

// formatAcceptanceGate formats the acceptance gate line with pass/fail counts.
func formatAcceptanceGate(rs *runstore.RunState) string {
	if rs.FinalAcceptancePassed {
		return "passed"
	}
	if len(rs.AcceptanceResults) == 0 {
		return "failed"
	}
	failCount := 0
	for _, a := range rs.AcceptanceResults {
		al := strings.ToLower(a)
		if strings.Contains(al, "fail") || strings.HasPrefix(a, "\u2717") {
			failCount++
		}
	}
	if failCount > 0 {
		return fmt.Sprintf("failed (%d/%d criteria failed)", failCount, len(rs.AcceptanceResults))
	}
	return "failed"
}

// suggestedActions generates actionable suggestions based on run state.
func suggestedActions(rs *runstore.RunState) []string {
	var actions []string

	// Check for potential false positives in review findings.
	if !rs.FinalReviewPassed && len(rs.ReviewFindings) > 0 {
		for _, f := range rs.ReviewFindings {
			fl := strings.ToLower(f)
			if strings.Contains(fl, "false positive") || strings.Contains(fl, "does not exist") {
				actions = append(actions, "Review failed with findings that may be false positives (diff visibility issue)")
				break
			}
		}
	}

	// Failed tasks.
	for _, t := range rs.Tasks {
		if t.Status == "failed" {
			actions = append(actions, fmt.Sprintf("Task %s (%s) failed after %d attempts",
				t.TaskID, truncateObjective(t.Objective, 40), t.Attempts))
		}
	}

	// Worktree hint.
	if rs.WorktreePath != "" {
		actions = append(actions, fmt.Sprintf("Run `gromit-next exec show %s --full` to inspect evidence", rs.RunID))
	}

	return actions
}

// readInvocationCount reads metrics.json from the evidence dir and returns the
// number of invocation records, or -1 if the file is absent or unreadable.
func readInvocationCount(evidenceDir string) int {
	data, err := os.ReadFile(filepath.Join(evidenceDir, "metrics.json"))
	if err != nil {
		return -1
	}
	var m struct {
		Invocations []json.RawMessage `json:"invocations"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return -1
	}
	return len(m.Invocations)
}
