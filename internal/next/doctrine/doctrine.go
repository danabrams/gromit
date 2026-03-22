// Package doctrine manages project-level coding standards, conventions,
// and rules that guide agent behavior.
//
// Doctrine is the successor to legacy RULES.md / LEARNINGS.md. It provides
// structured, queryable conventions rather than flat markdown.
package doctrine

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Rule is a single convention or standard.
type Rule struct {
	// ID is a stable identifier for this rule.
	ID string `json:"id"`

	// Summary is a short human-readable description.
	Summary string `json:"summary"`

	// Scope limits where this rule applies (e.g. "tests", "api", "*").
	Scope string `json:"scope"`

	// Source indicates how this rule was introduced.
	Source string `json:"source"`

	// CreatedAt is when this rule was created.
	CreatedAt time.Time `json:"created_at"`

	// Status indicates the rule's lifecycle state (active, superseded).
	Status string `json:"status"`

	// SupersededBy holds the ID of the decision that superseded this rule.
	SupersededBy string `json:"superseded_by"`
}

// NewRule creates a Rule with Source set to "declared" and CreatedAt set to now.
func NewRule(id string, summary string, scope string) Rule {
	return Rule{
		ID:        id,
		Summary:   summary,
		Scope:     scope,
		Source:    "declared",
		CreatedAt: time.Now(),
	}
}

// Doctrine represents the full set of rules and conventions for a project.
type Doctrine struct {
	// Rules is the list of coding rules and conventions.
	Rules []Rule `json:"rules"`
}

// NormalizeNilFields maps nil slices/maps to empty values.
// See CLAUDE.md nil-field normalization visibility convention.
func (d *Doctrine) NormalizeNilFields() {
	if d.Rules == nil {
		d.Rules = []Rule{}
	}
}

// Store is the interface for persisting and loading doctrine.
type Store interface {
	Save(doctrineDir string, d Doctrine) error
	Load(doctrineDir string) (Doctrine, error)
}

// FSStore implements Store using the local filesystem.
type FSStore struct{}

// NewFSStore returns a new filesystem-backed doctrine store.
func NewFSStore() *FSStore {
	return &FSStore{}
}

// Save writes the doctrine to doctrineDir/rules.json.
func (s *FSStore) Save(doctrineDir string, d Doctrine) error {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(doctrineDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(doctrineDir, "rules.json"), data, 0o644)
}

// Load reads the doctrine from doctrineDir/rules.json. Returns an empty
// Doctrine if the file does not exist.
func (s *FSStore) Load(doctrineDir string) (Doctrine, error) {
	data, err := os.ReadFile(filepath.Join(doctrineDir, "rules.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Doctrine{Rules: []Rule{}}, nil
		}
		return Doctrine{}, err
	}
	var d Doctrine
	if err := json.Unmarshal(data, &d); err != nil {
		return Doctrine{}, err
	}
	return d, nil
}
