package enrich

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const factsFileName = "facts.json"

// FactStore manages persisting and merging inferred facts on disk.
// It is stateless — all state lives in the filesystem.
type FactStore struct{}

// NewFactStore returns a new FactStore.
func NewFactStore() *FactStore {
	return &FactStore{}
}

// SaveFacts writes facts to inferred/facts.json under cellPath.
// It creates the inferred/ directory if it does not exist.
func (s *FactStore) SaveFacts(cellPath string, facts []InferredFact) error {
	dir := filepath.Join(cellPath, "inferred")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating inferred dir: %w", err)
	}

	// Normalize nil fields before serialization.
	for i := range facts {
		facts[i].NormalizeNilFields()
	}

	data, err := json.MarshalIndent(facts, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling facts: %w", err)
	}

	path := filepath.Join(dir, factsFileName)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing facts file: %w", err)
	}
	return nil
}

// LoadFacts reads facts from inferred/facts.json under cellPath.
// If the file does not exist, it returns an empty slice and no error.
func (s *FactStore) LoadFacts(cellPath string) ([]InferredFact, error) {
	path := filepath.Join(cellPath, "inferred", factsFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []InferredFact{}, nil
		}
		return nil, fmt.Errorf("reading facts file: %w", err)
	}

	var facts []InferredFact
	if err := json.Unmarshal(data, &facts); err != nil {
		return nil, fmt.Errorf("unmarshaling facts: %w", err)
	}
	return facts, nil
}

// MergeWithExisting combines existing on-disk facts with newly inferred incoming
// facts. The merge rules are:
//   - If an existing accepted fact's ID appears in incoming, its accepted status
//     is preserved (only accepted is sticky).
//   - If an existing rejected fact's ID re-appears in incoming, it is re-proposed.
//   - If an existing fact's ID does NOT appear in incoming, it is marked superseded.
//   - Incoming facts with no matching existing fact are kept as-is (proposed).
func (s *FactStore) MergeWithExisting(existing, incoming []InferredFact) []InferredFact {
	existingByID := make(map[string]InferredFact, len(existing))
	for _, f := range existing {
		existingByID[f.FactID] = f
	}

	incomingByID := make(map[string]struct{}, len(incoming))
	for _, f := range incoming {
		incomingByID[f.FactID] = struct{}{}
	}

	merged := make([]InferredFact, 0, len(existing)+len(incoming))

	// Process incoming facts, preserving existing statuses where applicable.
	for _, inc := range incoming {
		if ex, ok := existingByID[inc.FactID]; ok {
			// Only accepted is sticky; rejected facts that re-appear are re-proposed.
			if ex.Status == StatusAccepted {
				inc.Status = ex.Status
			}
		}
		merged = append(merged, inc)
	}

	// Mark existing facts not present in incoming as superseded.
	for _, ex := range existing {
		if _, ok := incomingByID[ex.FactID]; !ok {
			ex.Status = StatusSuperseded
			merged = append(merged, ex)
		}
	}

	return merged
}

// UpdateStatus loads facts from cellPath, updates the status of the fact with
// the given factID, and saves the modified list back to disk.
func (s *FactStore) UpdateStatus(cellPath string, factID string, status Status) error {
	facts, err := s.LoadFacts(cellPath)
	if err != nil {
		return err
	}

	found := false
	for i := range facts {
		if facts[i].FactID == factID {
			facts[i].Status = status
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("fact %q not found", factID)
	}

	return s.SaveFacts(cellPath, facts)
}
