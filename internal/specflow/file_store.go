package specflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// NewFileStore creates a file-backed SpecStore inside the given gromit directory.
func NewFileStore(gromitDir string) (SpecStore, error) {
	gromitDir = strings.TrimSpace(gromitDir)
	if gromitDir == "" {
		gromitDir = ".gromit"
	}
	store := &fileStore{
		path:      filepath.Join(gromitDir, "specflow.json"),
		stages:    make(map[string]Stage),
		readFile:  os.ReadFile,
		writeFile: os.WriteFile,
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

type fileStore struct {
	mu        sync.Mutex
	path      string
	stages    map[string]Stage
	readFile  func(string) ([]byte, error)
	writeFile func(string, []byte, os.FileMode) error
}

func (f *fileStore) Stage(_ context.Context, specID string) (Stage, error) {
	if f == nil {
		return "", fmt.Errorf("specflow file store is nil")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	stage, ok := f.stages[specID]
	if !ok {
		return "", ErrStageNotFound
	}
	return stage, nil
}

func (f *fileStore) StoreStage(_ context.Context, specID string, stage Stage) error {
	if f == nil {
		return fmt.Errorf("specflow file store is nil")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stages[specID] = stage
	return f.saveLocked()
}

func (f *fileStore) load() error {
	if f == nil {
		return fmt.Errorf("specflow file store is nil")
	}
	read := os.ReadFile
	if f.readFile != nil {
		read = f.readFile
	}
	data, err := read(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading specflow store: %w", err)
	}
	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parsing specflow store: %w", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for specID, value := range raw {
		f.stages[specID] = Stage(value)
	}
	return nil
}

func (f *fileStore) saveLocked() error {
	data := make(map[string]string, len(f.stages))
	for specID, stage := range f.stages {
		data[specID] = string(stage)
	}
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling specflow store: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return fmt.Errorf("creating specflow store dir: %w", err)
	}
	write := os.WriteFile
	if f.writeFile != nil {
		write = f.writeFile
	}
	if err := write(f.path, jsonData, 0644); err != nil {
		return fmt.Errorf("writing specflow store: %w", err)
	}
	return nil
}
