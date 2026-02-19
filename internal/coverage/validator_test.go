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

func TestParseValidationResponse_ValidJSON(t *testing.T) {
	output := `{"covers": true, "reason": "matches criterion"}`

	resp, err := ParseValidationResponse(output)
	if err != nil {
		t.Fatalf("ParseValidationResponse() error: %v", err)
	}

	if !resp.Covers {
		t.Fatalf("Covers = false, want true")
	}

	if resp.Reason != "matches criterion" {
		t.Fatalf("Reason = %q, want %q", resp.Reason, "matches criterion")
	}
}
