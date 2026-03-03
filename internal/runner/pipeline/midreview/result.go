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
	input := []byte(raw)
	if err := json.Unmarshal(input, &result); err == nil {
		normalizeMidBuildReviewResult(&result)
		return &result, nil
	}

	var legacy []string
	if err := json.Unmarshal(input, &legacy); err == nil {
		result.Findings = make([]Finding, len(legacy))
		for i, msg := range legacy {
			result.Findings[i] = Finding{Message: msg}
		}
		normalizeMidBuildReviewResult(&result)
		return &result, nil
	} else {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
}

func normalizeMidBuildReviewResult(result *MidBuildReviewResult) {
	if result == nil {
		return
	}

	result.Summary = strings.TrimSpace(result.Summary)
	for i := range result.Findings {
		result.Findings[i].Category = strings.TrimSpace(result.Findings[i].Category)
		result.Findings[i].Message = strings.TrimSpace(result.Findings[i].Message)
	}
}
