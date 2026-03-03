package midreview

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Finding represents a single finding from the mid-build review.
type Finding struct {
	Category string `json:"category"`
	Message  string `json:"message"`
}

// MidBuildReviewResult represents the aggregated result of a mid-build review.
type MidBuildReviewResult struct {
	Findings []Finding `json:"findings"`
	Summary  string    `json:"summary"`
}

// ParseMidBuildReviewResult parses raw JSON input into a MidBuildReviewResult.
// It returns an error if the input is malformed or empty.
func ParseMidBuildReviewResult(raw string) (*MidBuildReviewResult, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty input")
	}

	var result MidBuildReviewResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &result, nil
}
