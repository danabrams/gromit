package coverage

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/jsonutil"
)

const (
	selfReportLabel         = "self report"
	validationResponseLabel = "validation response"
)

var (
	selfReportKeys         = []string{"targeting", "remaining"}
	validationResponseKeys = []string{"covers", "reason"}
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
	if s == nil {
		return
	}
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

	// Scan for candidate JSON objects and parse the first one that contains the expected keys.
	for {
		start := strings.IndexByte(output[searchFrom:], '{')
		if start == -1 {
			break
		}
		start += searchFrom

		jsonBlock := jsonutil.ExtractBracketedObject(output[start:])
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

func ParseSelfReport(output string) (*SelfReport, error) {
	var report SelfReport
	if err := parseEmbeddedJSON(output, selfReportLabel, selfReportKeys, &report); err != nil {
		return nil, err
	}

	report.normalizeNilFields()

	return &report, nil
}

func ParseValidationResponse(output string) (*ValidationResponse, error) {
	var resp ValidationResponse
	if err := parseEmbeddedJSON(output, validationResponseLabel, validationResponseKeys, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
