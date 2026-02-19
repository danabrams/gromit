package coverage

import "testing"

func TestParseSelfReport_ValidJSON(t *testing.T) {
	output := `{"targeting": 3, "remaining": [5, 7]}`

	report, err := ParseSelfReport(output)
	if err != nil {
		t.Fatalf("ParseSelfReport() error: %v", err)
	}

	if report.Targeting != 3 {
		t.Fatalf("Targeting = %d, want 3", report.Targeting)
	}

	if len(report.Remaining) != 2 || report.Remaining[0] != 5 || report.Remaining[1] != 7 {
		t.Fatalf("Remaining = %v, want [5 7]", report.Remaining)
	}
}
