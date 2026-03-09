package findings

import "testing"

func TestDeriveVerdictCritical(t *testing.T) {
	verdict := DeriveVerdict([]Finding{{Severity: SeverityCritical}})
	if verdict != VerdictFail {
		t.Fatalf("expected fail verdict for critical finding, got %s", verdict)
	}
}

func TestDeriveVerdictPassWhenNoCritical(t *testing.T) {
	verdict := DeriveVerdict([]Finding{{Severity: SeverityWarning}})
	if verdict != VerdictPass {
		t.Fatalf("expected pass verdict without critical findings, got %s", verdict)
	}
}
