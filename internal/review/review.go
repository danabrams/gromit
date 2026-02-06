package review

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ReviewResult represents the structured output from a code review
type ReviewResult struct {
	Passed        bool           `json:"passed"`
	FixesApplied  []string       `json:"fixes_applied"`
	BeadsToCreate []BeadProposal `json:"beads_to_create"`
	BacklogItems  []BacklogItem  `json:"backlog_items"`
	Summary       string         `json:"summary"`
}

// BeadProposal represents a new bead that should be created based on review findings
type BeadProposal struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Priority    int      `json:"priority"`
	Labels      []string `json:"labels"`
}

// BacklogItem represents a lower-priority item that should be tracked but not immediately worked on
type BacklogItem struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Reason      string `json:"reason"`
}

// normalizeNilFields ensures nil slices are replaced with empty slices.
// This prevents issues with downstream code that may range over nil slices
// vs code that checks len() or marshals to JSON (nil → "null" vs [] → "[]").
func (r *ReviewResult) normalizeNilFields() {
	if r == nil {
		return
	}
	if r.FixesApplied == nil {
		r.FixesApplied = []string{}
	}
	if r.BeadsToCreate == nil {
		r.BeadsToCreate = []BeadProposal{}
	}
	if r.BacklogItems == nil {
		r.BacklogItems = []BacklogItem{}
	}
	// Normalize labels within each bead proposal
	for i := range r.BeadsToCreate {
		if r.BeadsToCreate[i].Labels == nil {
			r.BeadsToCreate[i].Labels = []string{}
		}
	}
}

// ParseReviewResult extracts a ReviewResult from Claude's output.
// It handles surrounding text by finding the JSON object boundaries.
func ParseReviewResult(output string) (*ReviewResult, error) {
	if output == "" {
		return nil, fmt.Errorf("review output is empty")
	}

	output = strings.TrimSpace(output)

	// Find JSON boundaries
	start := strings.Index(output, "{")
	end := strings.LastIndex(output, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON found in review output")
	}

	jsonStr := output[start : end+1]

	var result ReviewResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("parsing review JSON: %w", err)
	}

	result.normalizeNilFields()
	return &result, nil
}
