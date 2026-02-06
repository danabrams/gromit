package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// State represents persistent state stored in .ralph/state.json
type State struct {
	LastRetro time.Time `json:"last_retro,omitempty"`
}

// File manages the state.json file
type File struct {
	path  string
	state State
}

// NewFile creates a new state file manager
func NewFile(ralphDir string) (*File, error) {
	return &File{
		path: filepath.Join(ralphDir, "state.json"),
	}, nil
}

// Load reads the state from disk
func (f *File) Load() error {
	if f == nil {
		return fmt.Errorf("state file is nil")
	}
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No state yet
		}
		return fmt.Errorf("reading state file: %w", err)
	}

	if err := json.Unmarshal(data, &f.state); err != nil {
		return fmt.Errorf("parsing state file: %w", err)
	}

	return nil
}

// Save writes the state to disk
func (f *File) Save() error {
	if f == nil {
		return fmt.Errorf("state file is nil")
	}
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	data, err := json.MarshalIndent(f.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling state: %w", err)
	}

	return os.WriteFile(f.path, data, 0644)
}

// LastRetro returns the time of the last retrospective
func (f *File) LastRetro() time.Time {
	if f == nil {
		return time.Time{}
	}
	return f.state.LastRetro
}

// RecordRetro updates the last retro time to now
func (f *File) RecordRetro() error {
	if f == nil {
		return fmt.Errorf("state file is nil")
	}
	f.state.LastRetro = time.Now()
	return f.Save()
}
