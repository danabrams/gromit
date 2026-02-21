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

func TestParseSelfReport_SkipsUnrelatedJSON(t *testing.T) {
	output := `prefix {"foo":"bar"} middle {"targeting": 1, "remaining": [2, 3]} suffix`

	report, err := ParseSelfReport(output)
	if err != nil {
		t.Fatalf("ParseSelfReport() error: %v", err)
	}

	if report.Targeting != 1 {
		t.Fatalf("Targeting = %d, want 1", report.Targeting)
	}

	if len(report.Remaining) != 2 || report.Remaining[0] != 2 || report.Remaining[1] != 3 {
		t.Fatalf("Remaining = %v, want [2 3]", report.Remaining)
	}
}

func TestParseSelfReport_NormalizesMissingRemaining(t *testing.T) {
	output := `{"targeting": 4, "remaining": null}`

	report, err := ParseSelfReport(output)
	if err != nil {
		t.Fatalf("ParseSelfReport() error: %v", err)
	}

	if report.Remaining == nil {
		t.Fatal("Remaining = nil, want empty slice")
	}

	if len(report.Remaining) != 0 {
		t.Fatalf("Remaining len = %d, want 0", len(report.Remaining))
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

func TestParseValidationResponse_MalformedJSON(t *testing.T) {
	output := `{"covers": tru, "reason": "typo"}`

	_, err := ParseValidationResponse(output)
	if err == nil {
		t.Fatal("ParseValidationResponse() expected error, got nil")
	}
}

func TestParseValidationResponse_SkipsUnrelatedJSON(t *testing.T) {
	output := `start {"targeting":1,"remaining":[2]} then {"covers": true, "reason": "ok"}`

	resp, err := ParseValidationResponse(output)
	if err != nil {
		t.Fatalf("ParseValidationResponse() error: %v", err)
	}

	if !resp.Covers {
		t.Fatal("Covers = false, want true")
	}

	if resp.Reason != "ok" {
		t.Fatalf("Reason = %q, want %q", resp.Reason, "ok")
	}
}

func TestParseValidationResponse_EmbeddedJSONWithEscapes(t *testing.T) {
	output := `prefix {"covers": true, "reason": "C:\\temp\\\"quoted\\\" {ok}"} suffix`

	resp, err := ParseValidationResponse(output)
	if err != nil {
		t.Fatalf("ParseValidationResponse() error: %v", err)
	}

	if !resp.Covers {
		t.Fatal("Covers = false, want true")
	}

	if resp.Reason != `C:\temp\"quoted\" {ok}` {
		t.Fatalf("Reason = %q, want %q", resp.Reason, `C:\temp\"quoted\" {ok}`)
	}
}

func TestParseSelfReport_UnbalancedBracketedJSON(t *testing.T) {
	output := `noise {"targeting": 1, "remaining": [2, 3]`

	_, err := ParseSelfReport(output)
	if err == nil {
		t.Fatal("ParseSelfReport() expected error, got nil")
	}
}
