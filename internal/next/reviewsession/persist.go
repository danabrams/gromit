package reviewsession

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteOutcome writes ReviewOutcome as JSON to review-outcome.json in the evidence directory.
func WriteOutcome(evidenceDir string, outcome *ReviewOutcome) error {
	if outcome == nil {
		return fmt.Errorf("outcome cannot be nil")
	}

	// Normalize nil fields to empty slices
	outcome.NormalizeNilFields()

	// Create evidence directory if it doesn't exist
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		return fmt.Errorf("create evidence directory: %w", err)
	}

	// Marshal to JSON with indent
	data, err := json.MarshalIndent(outcome, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal outcome: %w", err)
	}

	// Write to file
	filePath := filepath.Join(evidenceDir, "review-outcome.json")
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("write outcome file: %w", err)
	}

	return nil
}

// ReadOutcome reads ReviewOutcome from review-outcome.json in the evidence directory.
func ReadOutcome(evidenceDir string) (*ReviewOutcome, error) {
	filePath := filepath.Join(evidenceDir, "review-outcome.json")

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read outcome file: %w", err)
	}

	// Unmarshal from JSON
	var outcome ReviewOutcome
	if err := json.Unmarshal(data, &outcome); err != nil {
		return nil, fmt.Errorf("unmarshal outcome: %w", err)
	}

	// Normalize nil fields to empty slices
	outcome.NormalizeNilFields()

	return &outcome, nil
}
