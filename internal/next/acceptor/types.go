package acceptor

const (
	StatusPass    = "pass"
	StatusFail    = "fail"
	StatusUnclear = "unclear"
)

// CriterionResult holds the evaluation result for a single acceptance criterion.
type CriterionResult struct {
	Criterion    string   `json:"criterion"`
	Status       string   `json:"status"`
	Rationale    string   `json:"rationale"`
	EvidenceRefs []string `json:"evidence_refs"`
}

// NormalizeNilFields maps nil slices to empty values.
func (cr *CriterionResult) NormalizeNilFields() {
	if cr.EvidenceRefs == nil {
		cr.EvidenceRefs = []string{}
	}
}

// AcceptanceResult holds the aggregate result of evaluating all acceptance criteria.
type AcceptanceResult struct {
	Results          []CriterionResult `json:"results"`
	AllPass          bool              `json:"all_pass"`
	HasFailOrUnclear bool             `json:"has_fail_or_unclear"`
}

// NormalizeNilFields maps nil slices to empty values.
func (ar *AcceptanceResult) NormalizeNilFields() {
	if ar.Results == nil {
		ar.Results = []CriterionResult{}
	}
	for i := range ar.Results {
		ar.Results[i].NormalizeNilFields()
	}
}
