package display

import (
	"fmt"
	"strings"
)

// IntegrationQueueStatus holds queue summary for display purposes.
type IntegrationQueueStatus struct {
	QueueLength      int                         `json:"queue_length"`
	ReadyCount       int                         `json:"ready_count"`
	IntegratingCount int                         `json:"integrating_count"`
	BlockedCount     int                         `json:"blocked_count"`
	MergedCount      int                         `json:"merged_count"`
	Entries          []*IntegrationQueueEntryView `json:"entries"`
}

// IntegrationQueueEntryView captures the display fields for a queue entry.
type IntegrationQueueEntryView struct {
	Branch           string `json:"branch"`
	State            string `json:"state"`
	Lane             string `json:"lane"`
	ReadyPosition    int    `json:"ready_position,omitempty"`
	FifoSequence     int    `json:"fifo_seq,omitempty"`
	RetryAttempt     int    `json:"retry_attempt,omitempty"`
	FailureReason    string `json:"failure_reason,omitempty"`
	LastErrorCode    string `json:"last_error_code,omitempty"`
	LastErrorMessage string `json:"last_error_message,omitempty"`
}

var integrationQueueRecoveryInstructions = map[string]string{
	"conflict":       `Resolve merge conflicts on the branch (e.g., git checkout <branch> && git rebase origin/main) and allow the coordinator to retry.`,
	"failed_gates":   `Fix gate failures (see .gromit/logs) and rerun validation before the queue retries.`,
	"lane_violation": `Adjust lane policy compliance (safe_lane vs code_lane) and requeue the session branch once aligned.`,
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

	components := buildIntegrationQueueEntryComponents(entry)
	if len(components) == 0 {
		return ""
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
	return integrationQueueRecoveryInstructions[state]
}

// FormatIntegrationQueueError renders a short error block when the queue
// cannot be loaded.
func FormatIntegrationQueueError(code, detail string) string {
	lines := []string{
		"Integration Queue:",
		fmt.Sprintf("  Error: %s", code),
	}
	if detail != "" {
		lines = append(lines, fmt.Sprintf("  Details: %s", detail))
	}
	return strings.Join(lines, "\n")
}
