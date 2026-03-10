package review

import "testing"

func TestComputeVerdictCriticalFindings(t *testing.T) {
	t.Parallel()

	findings := []Finding{
		{Severity: SeverityCritical},
	}

	verdict := ComputeVerdict(findings)
	if verdict != VerdictFail {
		t.Fatalf("verdict = %q, want %q", verdict, VerdictFail)
	}
}

func TestComputeVerdictPassesWhenNoCritical(t *testing.T) {
	t.Parallel()

	findings := []Finding{
		{Severity: SeverityWarning},
		{Severity: SeveritySuggestion},
	}

	verdict := ComputeVerdict(findings)
	if verdict != VerdictPass {
		t.Fatalf("verdict = %q, want %q", verdict, VerdictPass)
	}
}

func TestHasFindings(t *testing.T) {
	t.Parallel()

	if HasFindings(nil) {
		t.Fatal("expected nil slice to mean no findings")
	}
	if HasFindings([]Finding{}) {
		t.Fatal("expected empty slice to mean no findings")
	}
	if !HasFindings([]Finding{{Description: "something"}}) {
		t.Fatal("expected non-empty slice to report findings")
	}
}
