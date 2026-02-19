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

func TestParseSelfReport_EmbeddedJSON(t *testing.T) {
	output := "noise before {\"targeting\": 2, \"remaining\": [4]} trailing text"

	report, err := ParseSelfReport(output)
	if err != nil {
		t.Fatalf("ParseSelfReport() error: %v", err)
	}

	if report.Targeting != 2 {
		t.Fatalf("Targeting = %d, want 2", report.Targeting)
	}

	if len(report.Remaining) != 1 || report.Remaining[0] != 4 {
		t.Fatalf("Remaining = %v, want [4]", report.Remaining)
	}
}

func TestParseSelfReport_MalformedJSON(t *testing.T) {
	output := `{"targeting": 1, "remaining": [2, }`

	_, err := ParseSelfReport(output)
	if err == nil {
		t.Fatal("ParseSelfReport() expected error, got nil")
	}
}

func TestParseValidationResponse_EmbeddedJSON(t *testing.T) {
	output := "prefix {\"covers\": false, \"reason\": \"missing assertion\"} suffix"

	resp, err := ParseValidationResponse(output)
	if err != nil {
		t.Fatalf("ParseValidationResponse() error: %v", err)
	}

	if resp.Covers {
		t.Fatalf("Covers = true, want false")
	}

	if resp.Reason != "missing assertion" {
		t.Fatalf("Reason = %q, want %q", resp.Reason, "missing assertion")
	}
}
