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
