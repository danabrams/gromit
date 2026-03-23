package proposaltriage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadDecisions reads decisions from the proposal-decisions.json file in the given directory.
// Returns an empty slice if the file doesn't exist or if dir is empty.
func LoadDecisions(dir string) ([]Decision, error) {
	if dir == "" {
		return []Decision{}, nil
	}
	filePath := filepath.Join(dir, "proposal-decisions.json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Decision{}, nil
		}
		return nil, err
	}

	var decisions []Decision
	if err := json.Unmarshal(data, &decisions); err != nil {
		return nil, err
	}

	return decisions, nil
}

// SaveDecisions writes decisions to the proposal-decisions.json file in the given directory.
// It overwrites existing decisions for the same proposal IDs and preserves others.
// Creates the directory if it doesn't exist.
// Returns an error if dir is empty to prevent accidental writes to the current working directory.
func SaveDecisions(dir string, newDecisions []Decision) error {
	if dir == "" {
		return fmt.Errorf("SaveDecisions: evidenceDir must not be empty")
	}
	// Load existing decisions
	existing, err := LoadDecisions(dir)
	if err != nil {
		return err
	}

	// Build a map of existing decisions by proposal ID
	existingMap := make(map[string]Decision)
	for _, d := range existing {
		existingMap[d.ProposalID] = d
	}

	// Update with new decisions (overwriting by proposal ID)
	for _, d := range newDecisions {
		d.NormalizeNilFields()
		existingMap[d.ProposalID] = d
	}

	// Reconstruct the slice from the map (preserves order by iterating through new decisions first, then existing)
	var result []Decision
	seenProposalIDs := make(map[string]bool)

	// First, add new decisions in order
	for _, d := range newDecisions {
		result = append(result, existingMap[d.ProposalID])
		seenProposalIDs[d.ProposalID] = true
	}

	// Then, add any existing decisions that weren't updated
	for _, d := range existing {
		if !seenProposalIDs[d.ProposalID] {
			result = append(result, d)
		}
	}

	// Write to file
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(dir, "proposal-decisions.json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return err
	}

	return nil
}

// SaveDecision saves a single decision to the proposal-decisions.json file in the given directory.
// It updates existing decisions for the same proposal ID and preserves others.
func SaveDecision(dir string, decision Decision) error {
	return SaveDecisions(dir, []Decision{decision})
}

// IsTerminalDecision checks if a Decision has a terminal action (dismissed).
func IsTerminalDecision(d Decision) bool {
	return d.Action == "dismissed"
}

// FindExistingDecision searches for a decision with the given proposal ID in the slice.
// Returns the matching Decision and true if found, or a zero Decision and false if not found.
func FindExistingDecision(proposalID string, decisions []Decision) (Decision, bool) {
	for _, d := range decisions {
		if d.ProposalID == proposalID {
			return d, true
		}
	}
	return Decision{}, false
}

// ValidateTerminalState checks if a proposal ID already has a terminal decision.
// Returns an error if a dismissed decision exists for that proposal.
func ValidateTerminalState(proposalID string, decisions []Decision) error {
	decision, found := FindExistingDecision(proposalID, decisions)
	if found && IsTerminalDecision(decision) {
		return fmt.Errorf("proposal %q cannot be re-decided: it has been dismissed", proposalID)
	}
	return nil
}
