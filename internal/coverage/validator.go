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

func parseEmbeddedJSON(output string, label string, expectedKeys []string, dest any) error {
	if strings.TrimSpace(output) == "" {
		return fmt.Errorf("%s output is empty", label)
	}

	searchFrom := 0
	var lastErr error

	for {
		start := strings.IndexByte(output[searchFrom:], '{')
		if start == -1 {
			break
		}
		start += searchFrom

		jsonBlock := extractBracketedJSON(output[start:])
		if jsonBlock == "" {
			searchFrom = start + 1
			continue
		}

		if !hasExpectedKeys(jsonBlock, expectedKeys) {
			searchFrom = start + 1
			continue
		}

		err := json.Unmarshal([]byte(jsonBlock), dest)
		if err == nil {
			return nil
		}
		lastErr = err
		searchFrom = start + 1
	}

	if lastErr != nil {
		return fmt.Errorf("parsing %s JSON: %w", label, lastErr)
	}

	return fmt.Errorf("no %s JSON block found", label)
}

func hasExpectedKeys(jsonBlock string, expectedKeys []string) bool {
	if len(expectedKeys) == 0 {
		return true
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonBlock), &object); err != nil {
		return false
	}

	for _, key := range expectedKeys {
		if _, ok := object[key]; !ok {
			return false
		}
	}

	return true
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
	if err := parseEmbeddedJSON(output, "self report", []string{"targeting", "remaining"}, &report); err != nil {
		return nil, err
	}

	report.normalizeNilFields()

	return &report, nil
}

func ParseValidationResponse(output string) (*ValidationResponse, error) {
	var resp ValidationResponse
	if err := parseEmbeddedJSON(output, "validation response", []string{"covers", "reason"}, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
