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
