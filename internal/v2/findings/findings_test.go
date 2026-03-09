package findings

import "testing"

func TestDeriveVerdictCritical(t *testing.T) {
	verdict := DeriveVerdict([]Finding{{Severity: SeverityCritical}})
	if verdict != VerdictFail {
		t.Fatalf("expected fail verdict for critical finding, got %s", verdict)
	}
}
