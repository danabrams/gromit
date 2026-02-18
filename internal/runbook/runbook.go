package runbook

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// Append writes a runbook entry as a JSONL line.
func Append(gromitDir string, entry Entry) error {
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		return fmt.Errorf("creating runbook directory: %w", err)
	}
	path := filepath.Join(gromitDir, "runbooks.jsonl")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening runbook file: %w", err)
	}
	defer file.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling runbook entry: %w", err)
	}

	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("writing runbook entry: %w", err)
	}

	return nil
}

// List returns runbook entries filtered by TTL.
func List(gromitDir string, ttlDays int) ([]Entry, error) {
	path := filepath.Join(gromitDir, "runbooks.jsonl")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil
		}
		return nil, fmt.Errorf("opening runbook file: %w", err)
	}
	defer file.Close()

	cutoff := time.Now().AddDate(0, 0, -ttlDays)
	entries := []Entry{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("parsing runbook line: %w", err)
		}
		if ttlDays > 0 && entry.Timestamp.Before(cutoff) {
			continue
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading runbook file: %w", err)
	}

	return entries, nil
}

// Cleanup rewrites the runbook file without expired entries.
func Cleanup(gromitDir string, ttlDays int) error {
	entries, err := List(gromitDir, ttlDays)
	if err != nil {
		return fmt.Errorf("loading runbook entries: %w", err)
	}

	path := filepath.Join(gromitDir, "runbooks.jsonl")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("opening runbook file: %w", err)
	}
	defer file.Close()

	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("marshaling runbook entry: %w", err)
		}
		if _, err := file.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("writing runbook entry: %w", err)
		}
	}

	return nil
}
