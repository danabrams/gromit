package coverage

import (
	"encoding/json"
	"fmt"
	"strings"
)

type SelfReport struct {
	Targeting int   `json:"targeting"`
	Remaining []int `json:"remaining"`
}

type ValidationResponse struct {
	Covers bool   `json:"covers"`
	Reason string `json:"reason"`
}

func (s *SelfReport) normalizeNilFields() {
	if s.Remaining == nil {
		s.Remaining = []int{}
	}
}

func parseEmbeddedJSON(output string, label string, dest any) error {
	if output == "" {
		return fmt.Errorf("%s output is empty", label)
	}

	searchFrom := 0
	var lastErr error

	for {
		start := strings.Index(output[searchFrom:], "{")
		if start == -1 {
			break
		}
		start += searchFrom

		jsonBlock := extractBracketedJSON(output[start:])
		if jsonBlock == "" {
			searchFrom = start + 1
			continue
		}

		if err := json.Unmarshal([]byte(jsonBlock), dest); err == nil {
			return nil
		} else {
			lastErr = err
			searchFrom = start + 1
		}
	}

	if lastErr != nil {
		return fmt.Errorf("parsing %s JSON: %w", label, lastErr)
	}

	return fmt.Errorf("no %s JSON block found", label)
}

// extractBracketedJSON finds the first balanced JSON object starting at the
// beginning of text and returns the full object including braces.
func extractBracketedJSON(text string) string {
	if len(text) == 0 || text[0] != '{' {
		return ""
	}

	depth := 0
	inString := false
	escapeNext := false

	for i := 0; i < len(text); i++ {
		char := text[i]

		if escapeNext {
			escapeNext = false
			continue
		}

		if char == '\\' {
			escapeNext = true
			continue
		}

		if char == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if char == '{' {
			depth++
			continue
		}

		if char == '}' {
			depth--
			if depth == 0 {
				return text[:i+1]
			}
		}
	}

	return ""
}

func ParseSelfReport(output string) (*SelfReport, error) {
	var report SelfReport
	if err := parseEmbeddedJSON(output, "self report", &report); err != nil {
		return nil, err
	}

	report.normalizeNilFields()

	return &report, nil
}

func ParseValidationResponse(output string) (*ValidationResponse, error) {
	var resp ValidationResponse
	if err := parseEmbeddedJSON(output, "validation response", &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
