package coverage

import (
	"fmt"

	"github.com/danabrams/gromit/internal/jsonutil"
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

	if err := jsonutil.ExtractObject(output, dest); err != nil {
		return fmt.Errorf("parsing %s JSON: %w", label, err)
	}

	return nil
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
