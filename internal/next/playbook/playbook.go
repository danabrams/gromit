package playbook

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Entry represents a single playbook entry.
type Entry struct {
	ID               string    `json:"id"`
	Type             string    `json:"type"`
	Title            string    `json:"title"`
	Content          string    `json:"content"`
	Rationale        string    `json:"rationale"`
	Status           string    `json:"status"`
	SourceProposalID string    `json:"source_proposal_id"`
	SourceRunID      string    `json:"source_run_id"`
	SourceSpecID     string    `json:"source_spec_id"`
	CreatedAt        time.Time `json:"created_at"`
	SupersededBy     string    `json:"superseded_by"`
}

// See CLAUDE.md nil-field normalization visibility convention: exported — cross-package boundary type
// NormalizeNilFields normalizes nil fields to empty values.
func (e *Entry) NormalizeNilFields() {
	// Entry struct has no slice or map fields, so nothing to normalize.
	// This method exists for consistency with project conventions.
}

// Store persists playbook entries to disk.
type Store struct {
	Dir string
}

// Load reads entries from the entries.json file.
// Returns empty slice if file doesn't exist.
func (s *Store) Load() ([]Entry, error) {
	filePath := filepath.Join(s.Dir, "entries.json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil
		}
		return nil, err
	}

	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}

	return entries, nil
}

// Save writes entries to the entries.json file.
// Creates the directory if it doesn't exist.
func (s *Store) Save(entries []Entry) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(s.Dir, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(s.Dir, "entries.json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return err
	}

	return nil
}

// ComputeID generates a playbook entry ID from type and content.
// Uses SHA-256 hash of normalized content, takes first 8 hex chars,
// and prefixes with "pb-".
func ComputeID(typ, content string) string {
	// Normalize whitespace: collapse multiple spaces/newlines/tabs to single space
	normalized := regexp.MustCompile(`\s+`).ReplaceAllString(content, " ")
	normalized = strings.TrimSpace(normalized)

	// Hash type and content together
	hashInput := fmt.Sprintf("%s:%s", typ, normalized)
	hash := sha256.Sum256([]byte(hashInput))

	// Take first 8 hex characters
	hexStr := fmt.Sprintf("%x", hash)
	return "pb-" + hexStr[:8]
}

// ActiveEntries returns only entries with status="active" and excludes superseded entries.
func ActiveEntries(entries []Entry) []Entry {
	var active []Entry
	for _, e := range entries {
		if e.Status == "active" && e.SupersededBy == "" {
			active = append(active, e)
		}
	}
	return active
}
