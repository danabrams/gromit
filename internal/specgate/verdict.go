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

// ParseVerdict unmarshals a JSON-encoded GateVerdict.
func ParseVerdict(data []byte) (*GateVerdict, error) {
	var v GateVerdict
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return &v, nil
}
