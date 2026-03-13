package review

import (
	"encoding/json"
	"fmt"
)

const (
	DispositionNew         = "new"
	DispositionPreExisting = "pre-existing"
)

// Finding represents a single review finding from a facet.
type Finding struct {
	Facet        string   `json:"facet"`
	Severity     Severity `json:"severity"`
	File         string   `json:"file"`
	Line         int      `json:"line"`
	Description  string   `json:"description"`
	SuggestedFix string   `json:"suggested_fix,omitempty"`
	Cycle        int      `json:"cycle"`
	Disposition  string   `json:"disposition,omitempty"`
}

// See CLAUDE.md nil-field normalization visibility convention:
// exported — cross-package boundary type
// NormalizeNilFields satisfies the codebase convention. Finding has no slice fields.
func (f *Finding) NormalizeNilFields() {}

// severityJSON handles JSON marshal/unmarshal for Finding using string severity.
type findingJSON struct {
	Facet        string `json:"facet"`
	Severity     string `json:"severity"`
	File         string `json:"file"`
	Line         int    `json:"line"`
	Description  string `json:"description"`
	SuggestedFix string `json:"suggested_fix,omitempty"`
	Cycle        int    `json:"cycle"`
	Disposition  string `json:"disposition,omitempty"`
}

func (f Finding) MarshalJSON() ([]byte, error) {
	return json.Marshal(findingJSON{
		Facet:        f.Facet,
		Severity:     f.Severity.String(),
		File:         f.File,
		Line:         f.Line,
		Description:  f.Description,
		SuggestedFix: f.SuggestedFix,
		Cycle:        f.Cycle,
		Disposition:  f.Disposition,
	})
}

func (f *Finding) UnmarshalJSON(data []byte) error {
	var raw findingJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	sev, err := ParseSeverity(raw.Severity)
	if err != nil {
		return fmt.Errorf("parsing finding severity: %w", err)
	}
	f.Facet = raw.Facet
	f.Severity = sev
	f.File = raw.File
	f.Line = raw.Line
	f.Description = raw.Description
	f.SuggestedFix = raw.SuggestedFix
	f.Cycle = raw.Cycle
	f.Disposition = raw.Disposition
	return nil
}

// FindingSet groups findings by facet.
type FindingSet struct {
	Facet    string    `json:"facet"`
	Findings []Finding `json:"findings"`
}

// See CLAUDE.md nil-field normalization visibility convention:
// exported — cross-package boundary type
// NormalizeNilFields maps nil Findings slice to empty.
func (fs *FindingSet) NormalizeNilFields() {
	if fs.Findings == nil {
		fs.Findings = []Finding{}
	}
}
