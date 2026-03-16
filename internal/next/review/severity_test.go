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

func TestParseSeverity_CaseInsensitive(t *testing.T) {
	cases := []struct {
		input string
		want  Severity
	}{
		{"Error", SeverityError},
		{"ERROR", SeverityError},
		{"Warning", SeverityWarning},
		{"WARNING", SeverityWarning},
		{"Suggestion", SeveritySuggestion},
		{"SUGGESTION", SeveritySuggestion},
		{"Info", SeverityInfo},
		{"INFO", SeverityInfo},
		{"eRrOr", SeverityError},
	}
	for _, tc := range cases {
		sev, err := ParseSeverity(tc.input)
		if err != nil {
			t.Errorf("ParseSeverity(%q) error: %v", tc.input, err)
			continue
		}
		if sev != tc.want {
			t.Errorf("ParseSeverity(%q) = %v, want %v", tc.input, sev, tc.want)
		}
	}
}

func TestParseSeverity_CriticalAlias(t *testing.T) {
	// "critical" is the spec-defined alias for SeverityError
	for _, input := range []string{"critical", "Critical", "CRITICAL"} {
		sev, err := ParseSeverity(input)
		if err != nil {
			t.Errorf("ParseSeverity(%q) error: %v", input, err)
			continue
		}
		if sev != SeverityError {
			t.Errorf("ParseSeverity(%q) = %v, want SeverityError", input, sev)
		}
	}
}

func TestParseSeverity_LLMAliases(t *testing.T) {
	// LLMs sometimes return severity values outside the canonical set.
	// These common aliases must be accepted and mapped to the nearest canonical severity.
	cases := []struct {
		input string
		want  Severity
	}{
		{"high", SeverityError},
		{"HIGH", SeverityError},
		{"medium", SeverityWarning},
		{"MEDIUM", SeverityWarning},
		{"low", SeveritySuggestion},
		{"LOW", SeveritySuggestion},
	}
	for _, tc := range cases {
		sev, err := ParseSeverity(tc.input)
		if err != nil {
			t.Errorf("ParseSeverity(%q) error: %v", tc.input, err)
			continue
		}
		if sev != tc.want {
			t.Errorf("ParseSeverity(%q) = %v, want %v", tc.input, sev, tc.want)
		}
	}
}

func TestParseSeverity_Invalid(t *testing.T) {
	_, err := ParseSeverity("bogus")
	if err == nil {
		t.Error("ParseSeverity(\"bogus\") should return error")
	}
}
