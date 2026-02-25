package experiment

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func stateFilePath(stateDir, experimentID string) string {
	return filepath.Join(stateDir, experimentID+".json")
}

// LoadState reads the bandit state for the given experiment.
// Missing files return a fresh zero state without an error.
func LoadState(stateDir, experimentID string) (*BanditState, error) {
	if experimentID == "" {
		return nil, fmt.Errorf("experiment ID is empty")
	}

	path := stateFilePath(stateDir, experimentID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &BanditState{}, nil
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	var state BanditState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}

	return &state, nil
}

// SaveState persists the bandit state for the given experiment.
// Writes atomically by creating a temp file and renaming it into place.
func SaveState(stateDir, experimentID string, state *BanditState) error {
	if experimentID == "" {
		return fmt.Errorf("experiment ID is empty")
	}

	target := stateFilePath(stateDir, experimentID)
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	if state == nil {
		state = &BanditState{}
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling state: %w", err)
	}

	tmp, err := os.CreateTemp(dir, experimentID+"-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp state file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp state file: %w", err)
	}

	if err := os.Rename(tmpPath, target); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("moving state file into place: %w", err)
	}

	return nil
}

// InitializeState ensures a persisted state file exists and returns its decoded state.
// If the file is missing, it writes a fresh zero state before returning it.
func InitializeState(stateDir, experimentID string) (*BanditState, error) {
	target := stateFilePath(stateDir, experimentID)
	if _, err := os.Stat(target); err == nil {
		return LoadState(stateDir, experimentID)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("checking state file: %w", err)
	}

	state := &BanditState{}
	if err := SaveState(stateDir, experimentID, state); err != nil {
		return nil, err
	}

	return state, nil
}
