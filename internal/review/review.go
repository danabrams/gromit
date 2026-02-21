package review

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	fixCategoryNilChecks     = "nil_checks"
	fixCategoryTestQuality   = "test_quality"
	fixCategoryErrorHandling = "error_handling"
)

// ReviewResult represents the structured output from a code review
type ReviewResult struct {
	Passed        bool           `json:"passed"`
	FixesApplied  []string       `json:"fixes_applied"`
	FixCategories []string       `json:"fix_categories"`
	BeadsToCreate []BeadProposal `json:"beads_to_create"`
	BacklogItems  []BacklogItem  `json:"backlog_items"`
	Summary       string         `json:"summary"`
	Learnings     []string       `json:"learnings,omitempty"`
}

// BeadProposal represents a new bead that should be created based on review findings
type BeadProposal struct {
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Priority        int      `json:"priority"`
	Labels          []string `json:"labels"`
	ExpectedOutputs []string `json:"expected_outputs,omitempty"`
}

// BacklogItem represents a lower-priority item that should be tracked but not immediately worked on
type BacklogItem struct {
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Reason          string   `json:"reason"`
	ExpectedOutputs []string `json:"expected_outputs,omitempty"`
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
	if r.FixCategories == nil {
		r.FixCategories = []string{}
	}
	if r.BeadsToCreate == nil {
		r.BeadsToCreate = []BeadProposal{}
	}
	if r.BacklogItems == nil {
		r.BacklogItems = []BacklogItem{}
	}
	if r.Learnings == nil {
		r.Learnings = []string{}
	}
	// Normalize labels within each bead proposal
	for i := range r.BeadsToCreate {
		if r.BeadsToCreate[i].Labels == nil {
			r.BeadsToCreate[i].Labels = []string{}
		}
		if r.BeadsToCreate[i].ExpectedOutputs == nil {
			r.BeadsToCreate[i].ExpectedOutputs = []string{}
		}
	}
	for i := range r.BacklogItems {
		if r.BacklogItems[i].ExpectedOutputs == nil {
			r.BacklogItems[i].ExpectedOutputs = []string{}
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

	if len(result.FixCategories) == 0 {
		result.FixCategories = CategorizeFixes(result.FixesApplied)
	}
	result.normalizeNilFields()
	return &result, nil
}

// CategorizeFixes maps review fix descriptions to stable category labels.
func CategorizeFixes(fixes []string) []string {
	if len(fixes) == 0 {
		return []string{}
	}

	categories := make(map[string]struct{})
	for _, fix := range fixes {
		label := categorizeFix(fix)
		if label == "" {
			continue
		}
		categories[label] = struct{}{}
	}

	if len(categories) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(categories))
	for category := range categories {
		result = append(result, category)
	}
	sort.Strings(result)
	return result
}

func categorizeFix(fix string) string {
	text := strings.ToLower(strings.TrimSpace(fix))
	if text == "" {
		return ""
	}

	if containsAny(text, "nil") {
		return fixCategoryNilChecks
	}

	if containsAny(text, "assert", "test", "coverage") {
		return fixCategoryTestQuality
	}

	if containsAny(text, "error", "panic", "recover") {
		return fixCategoryErrorHandling
	}

	return ""
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}
