package runbook

import (
	"fmt"
	"time"
)

// Entry represents a runbook entry captured after a failed bead.
type Entry struct {
	ID                 string    `json:"id"`
	Timestamp          time.Time `json:"timestamp"`
	BeadID             string    `json:"bead_id"`
	BeadTitle          string    `json:"bead_title"`
	SpecID             string    `json:"spec_id"`
	StartCommit        string    `json:"start_commit"`
	FailureCommit      string    `json:"failure_commit"`
	Prompt             string    `json:"prompt"`
	ValidationCommands []string  `json:"validation_commands"`
	FailureOutput      string    `json:"failure_output"`
	FailureCategory    string    `json:"failure_category"`
	EscalationChain    []string  `json:"escalation_chain"`
	Env                Env       `json:"env"`
}

// Env captures the environment metadata for a runbook entry.
type Env struct {
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// NewEntry constructs a runbook entry with an ID and timestamp.
func NewEntry(beadID string, now time.Time) Entry {
	return Entry{
		ID:        fmt.Sprintf("rb-%d-%s", now.Unix(), beadID),
		Timestamp: now.UTC(),
		BeadID:    beadID,
	}
}

func truncateOutput(output string) string {
	const maxBytes = 5 * 1024
	if len(output) <= maxBytes {
		return output
	}
	return output[len(output)-maxBytes:]
}
