package review

import "testing"

func TestThreshold_IsBlocking(t *testing.T) {
	tests := []struct {
		name      string
		threshold Severity
		finding   Severity
		want      bool
	}{
		{"error blocks at error threshold", SeverityError, SeverityError, true},
		{"warning does not block at error threshold", SeverityError, SeverityWarning, false},
		{"suggestion does not block at error threshold", SeverityError, SeveritySuggestion, false},
		{"info never blocks", SeverityError, SeverityInfo, false},
		{"warning blocks at warning threshold", SeverityWarning, SeverityWarning, true},
		{"error blocks at warning threshold", SeverityWarning, SeverityError, true},
		{"suggestion does not block at warning threshold", SeverityWarning, SeveritySuggestion, false},
		{"suggestion blocks at suggestion threshold", SeveritySuggestion, SeveritySuggestion, true},
		{"error blocks at suggestion threshold", SeveritySuggestion, SeverityError, true},
		{"info never blocks at any threshold", SeveritySuggestion, SeverityInfo, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBlocking(tt.threshold, tt.finding)
			if got != tt.want {
				t.Errorf("IsBlocking(%v, %v) = %v, want %v", tt.threshold, tt.finding, got, tt.want)
			}
		})
	}
}

func TestFilterBlockingFindings(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityError, Description: "error finding"},
		{Severity: SeverityWarning, Description: "warning finding"},
		{Severity: SeveritySuggestion, Description: "suggestion finding"},
		{Severity: SeverityInfo, Description: "info finding"},
	}

	blocking := FilterBlockingFindings(findings, SeverityWarning)
	if len(blocking) != 2 {
		t.Fatalf("expected 2 blocking findings at warning threshold, got %d", len(blocking))
	}
	if blocking[0].Description != "error finding" {
		t.Errorf("first blocking should be error, got %q", blocking[0].Description)
	}
	if blocking[1].Description != "warning finding" {
		t.Errorf("second blocking should be warning, got %q", blocking[1].Description)
	}
}

func TestFilterBlockingFindings_AllBelowThreshold(t *testing.T) {
	findings := []Finding{
		{Severity: SeveritySuggestion, Description: "suggestion finding"},
		{Severity: SeverityInfo, Description: "info finding"},
	}

	blocking := FilterBlockingFindings(findings, SeverityWarning)
	if len(blocking) != 0 {
		t.Errorf("expected 0 blocking findings when all below threshold, got %d", len(blocking))
	}
}

func TestFilterBlockingFindings_EmptyInput(t *testing.T) {
	blocking := FilterBlockingFindings([]Finding{}, SeverityWarning)
	if blocking == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(blocking) != 0 {
		t.Errorf("expected 0 blocking findings for empty input, got %d", len(blocking))
	}
}

func TestFilterBlockingFindings_InfoThresholdNeverBlocks(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityInfo, Description: "info finding 1"},
		{Severity: SeverityInfo, Description: "info finding 2"},
	}

	blocking := FilterBlockingFindings(findings, SeverityInfo)
	if len(blocking) != 0 {
		t.Errorf("info findings should never block regardless of threshold, got %d", len(blocking))
	}
}
