package specgate

import "encoding/json"

// CriterionResult holds the evaluation result for a single spec criterion.
type CriterionResult struct {
	Criterion string `json:"criterion"`
	Passed    bool   `json:"passed"`
	Evidence  string `json:"evidence"`
}

// GateVerdict is the overall verdict from a spec gate evaluation.
type GateVerdict struct {
	Passed  bool              `json:"passed"`
	Results []CriterionResult `json:"results"`
}

// FailedCriteria returns the subset of Results where Passed is false.
func (v *GateVerdict) FailedCriteria() []CriterionResult {
	var failed []CriterionResult
	for _, r := range v.Results {
		if !r.Passed {
			failed = append(failed, r)
		}
	}
	return failed
}

// ParseVerdict unmarshals a JSON-encoded GateVerdict.
func ParseVerdict(data []byte) (*GateVerdict, error) {
	var v GateVerdict
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return &v, nil
}
