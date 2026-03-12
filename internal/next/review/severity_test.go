package review

import "testing"

func TestSeverity_Ordering(t *testing.T) {
	if SeverityError.Rank() <= SeverityWarning.Rank() {
		t.Error("error should rank higher than warning")
	}
	if SeverityWarning.Rank() <= SeveritySuggestion.Rank() {
		t.Error("warning should rank higher than suggestion")
	}
	if SeveritySuggestion.Rank() <= SeverityInfo.Rank() {
		t.Error("suggestion should rank higher than info")
	}
}

func TestSeverity_String(t *testing.T) {
	tests := []struct {
		sev  Severity
		want string
	}{
		{SeverityError, "error"},
		{SeverityWarning, "warning"},
		{SeveritySuggestion, "suggestion"},
		{SeverityInfo, "info"},
	}
	for _, tt := range tests {
		if got := tt.sev.String(); got != tt.want {
			t.Errorf("Severity.String() = %q, want %q", got, tt.want)
		}
	}
}

func TestParseSeverity_Valid(t *testing.T) {
	for _, name := range []string{"error", "warning", "suggestion", "info"} {
		sev, err := ParseSeverity(name)
		if err != nil {
			t.Errorf("ParseSeverity(%q) error: %v", name, err)
		}
		if sev.String() != name {
			t.Errorf("ParseSeverity(%q).String() = %q", name, sev.String())
		}
	}
}

func TestParseSeverity_Invalid(t *testing.T) {
	_, err := ParseSeverity("critical")
	if err == nil {
		t.Error("ParseSeverity(\"critical\") should return error")
	}
}
