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

func ParseSelfReport(output string) (*SelfReport, error) {
	if output == "" {
		return nil, fmt.Errorf("self report output is empty")
	}

	var report SelfReport
	if err := jsonutil.ExtractObject(output, &report); err != nil {
		return nil, fmt.Errorf("parsing self report JSON: %w", err)
	}

	report.normalizeNilFields()

	return &report, nil
}

func ParseValidationResponse(output string) (*ValidationResponse, error) {
	if output == "" {
		return nil, fmt.Errorf("validation response output is empty")
	}

	var resp ValidationResponse
	if err := jsonutil.ExtractObject(output, &resp); err != nil {
		return nil, fmt.Errorf("parsing validation response JSON: %w", err)
	}

	return &resp, nil
}
