package display

import (
	"fmt"
	"strings"
)

// IntegrationQueueStatus holds queue summary for display purposes.
type IntegrationQueueStatus struct {
	QueueLength      int
	ReadyCount       int
	IntegratingCount int
	BlockedCount     int
	MergedCount      int
	Entries          []*IntegrationQueueEntryView
}

// IntegrationQueueEntryView captures the display fields for a queue entry.
type IntegrationQueueEntryView struct {
	Branch           string
	State            string
	Lane             string
	ReadyPosition    int
	LastErrorCode    string
	LastErrorMessage string
}

// FormatIntegrationQueue renders the queue summary and entries.
func FormatIntegrationQueue(status *IntegrationQueueStatus) string {
	if status == nil {
		return ""
	}

	lines := []string{
		"Integration Queue:",
		fmt.Sprintf("  Queue length: %d", status.QueueLength),
		fmt.Sprintf("  Ready: %d | Integrating: %d | Blocked: %d | Merged: %d",
			status.ReadyCount, status.IntegratingCount, status.BlockedCount, status.MergedCount),
	}

	if len(status.Entries) == 0 {
		return strings.Join(lines, "\n")
	}

	lines = append(lines, "  Entries:")
	for _, entry := range status.Entries {
		if entry == nil {
			continue
		}
		lines = append(lines, fmt.Sprintf("    %s", formatIntegrationQueueEntry(entry)))
	}

	return strings.Join(lines, "\n")
}

func formatIntegrationQueueEntry(entry *IntegrationQueueEntryView) string {
	if entry == nil {
		return ""
	}

	components := []string{fmt.Sprintf("%s (state: %s, lane: %s", entry.Branch, entry.State, entry.Lane)}
	if entry.ReadyPosition > 0 {
		components = append(components, fmt.Sprintf("position: %d", entry.ReadyPosition))
	}

	line := strings.Join(components, ", ") + ")"

	if entry.LastErrorCode != "" {
		errParts := []string{fmt.Sprintf("Error: %s", entry.LastErrorCode)}
		if entry.LastErrorMessage != "" {
			errParts = append(errParts, fmt.Sprintf("%s", entry.LastErrorMessage))
		}
		line += " — " + strings.Join(errParts, " ")
	}

	if inst := integrationQueueBlockedRecoveryInstruction(entry.State); inst != "" {
		line += " Recovery: " + inst
	}

	return line
}

func integrationQueueBlockedRecoveryInstruction(state string) string {
	switch state {
	case "conflict":
		return "Resolve merge conflicts on the branch (e.g., git checkout <branch> && git rebase origin/main) and allow the coordinator to retry."
	case "failed_gates":
		return "Fix gate failures (see .gromit/logs) and rerun validation before the queue retries."
	case "lane_violation":
		return "Adjust lane policy compliance (safe_lane vs code_lane) and requeue the session branch once aligned."
	default:
		return ""
	}
}
