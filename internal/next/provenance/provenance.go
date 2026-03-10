package provenance

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Record struct {
	FactID    string    `json:"fact_id"`
	Artifact  string    `json:"artifact"`
	Category  string    `json:"category"`
	GitSHA    string    `json:"git_sha"`
	Timestamp time.Time `json:"timestamp"`
	Extractor string    `json:"extractor"`
	InputHash string    `json:"input_hash"`
}

type Tracker interface {
	Record(rec Record) error
	Check(artifactName string) (Record, error)
	IsFresh(artifactName string, currentSHA string) (bool, error)
}

type FSTracker struct {
	path string
}

func NewFSTracker(path string) *FSTracker {
	return &FSTracker{path: path}
}

func (t *FSTracker) Record(rec Record) error {
	records, err := t.load()
	if err != nil {
		return fmt.Errorf("load provenance: %w", err)
	}
	records[rec.Artifact] = rec
	return t.save(records)
}

func (t *FSTracker) Check(artifactName string) (Record, error) {
	records, err := t.load()
	if err != nil {
		return Record{}, err
	}
	rec, ok := records[artifactName]
	if !ok {
		return Record{}, fmt.Errorf("no provenance for artifact %q", artifactName)
	}
	return rec, nil
}

func (t *FSTracker) IsFresh(artifactName string, currentSHA string) (bool, error) {
	rec, err := t.Check(artifactName)
	if err != nil {
		return false, err
	}
	return rec.GitSHA == currentSHA, nil
}

func (t *FSTracker) load() (map[string]Record, error) {
	data, err := os.ReadFile(t.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make(map[string]Record), nil
		}
		return nil, err
	}
	var records map[string]Record
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}

func (t *FSTracker) save(records map[string]Record) error {
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(t.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(t.path, data, 0o644)
}
