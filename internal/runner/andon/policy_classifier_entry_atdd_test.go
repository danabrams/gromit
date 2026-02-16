package andon

import "testing"

// Expected failure: PolicyClassification and ClassifyFailureEntry do not exist yet.
func TestClassifyFailureEntry_ReturnsOnlyCanonicalClasses(t *testing.T) {
	allowed := map[FailureClass]struct{}{
		FailureClassTransient: {},
		FailureClassWorkflow:  {},
		FailureClassQuality:   {},
		FailureClassIntent:    {},
		FailureClassData:      {},
	}

	cases := []FailureSignal{
		{Kind: FailureKindTimeout, Output: "context deadline exceeded"},
		{Kind: FailureKindWorkflow, Output: "missing bd sync before close"},
		{Kind: FailureKindQualityGate, Output: "go test ./... failed"},
		{Kind: FailureKindAmbiguousIntent, Output: "spec leaves behavior undefined"},
		{Kind: FailureKindIntegrity, Output: "state divergence detected"},
	}

	for _, signal := range cases {
		classified := ClassifyFailureEntry(signal)
		if _, ok := allowed[classified.Class]; !ok {
			t.Fatalf("ClassifyFailureEntry(%+v).Class = %q, want one of canonical classes", signal, classified.Class)
		}
	}

	var _ PolicyClassification
}
