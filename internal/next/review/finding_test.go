package review

import (
	"encoding/json"
	"testing"
)

func TestFinding_JSONRoundTrip(t *testing.T) {
	f := Finding{
		Facet:        "code_quality",
		Severity:     SeverityWarning,
		File:         "internal/handler.go",
		Line:         42,
		Description:  "nil pointer if commands list is empty",
		SuggestedFix: "add empty check before iteration",
		Cycle:        1,
		Disposition:  DispositionNew,
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Finding
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Facet != f.Facet {
		t.Errorf("Facet = %q, want %q", got.Facet, f.Facet)
	}
	if got.Severity != f.Severity {
		t.Errorf("Severity = %v, want %v", got.Severity, f.Severity)
	}
	if got.File != f.File {
		t.Errorf("File = %q, want %q", got.File, f.File)
	}
	if got.Line != f.Line {
		t.Errorf("Line = %d, want %d", got.Line, f.Line)
	}
	if got.Disposition != DispositionNew {
		t.Errorf("Disposition = %q, want %q", got.Disposition, DispositionNew)
	}
}

func TestFinding_NormalizeNilFields(t *testing.T) {
	f := Finding{}
	f.NormalizeNilFields()
	// No slices to normalize in Finding, but method should exist for convention
}

func TestFindingSet_NormalizeNilFields(t *testing.T) {
	fs := FindingSet{}
	fs.NormalizeNilFields()
	if fs.Findings == nil {
		t.Error("NormalizeNilFields should set nil Findings to empty slice")
	}
}

// --- Parsing boundary tests ---

func TestFinding_JSONRoundTrip_AllSeverities(t *testing.T) {
	severities := []struct {
		sev  Severity
		name string
	}{
		{SeverityInfo, "info"},
		{SeveritySuggestion, "suggestion"},
		{SeverityWarning, "warning"},
		{SeverityError, "error"},
	}

	for _, tc := range severities {
		t.Run(tc.name, func(t *testing.T) {
			f := Finding{
				Facet:       "test_facet",
				Severity:    tc.sev,
				File:        "main.go",
				Line:        10,
				Description: "test finding",
				Cycle:       1,
			}
			data, err := json.Marshal(f)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var got Finding
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got.Severity != tc.sev {
				t.Errorf("Severity = %v, want %v", got.Severity, tc.sev)
			}
		})
	}
}

func TestFinding_EmptyFindingsList_MarshalNotNull(t *testing.T) {
	fs := FindingSet{
		Facet:    "code_quality",
		Findings: []Finding{},
	}
	data, err := json.Marshal(fs)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	if s == "" {
		t.Fatal("expected non-empty JSON")
	}
	// Ensure findings is [] not null
	var got FindingSet
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Findings == nil {
		t.Error("empty findings should unmarshal as empty slice, not nil")
	}
	if len(got.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(got.Findings))
	}
}

func TestFinding_NilFindingsList_MarshalNull(t *testing.T) {
	// When Findings is nil (not normalized), JSON produces null.
	// After NormalizeNilFields it should produce [].
	fs := FindingSet{Facet: "test"}
	fs.NormalizeNilFields()
	data, err := json.Marshal(fs)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got FindingSet
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Findings == nil {
		t.Error("normalized nil findings should marshal as [] and unmarshal as empty slice")
	}
}

func TestFinding_UnmarshalInvalidSeverity(t *testing.T) {
	raw := `{"facet":"f","severity":"critical","file":"x.go","line":1,"description":"d","cycle":1}`
	var f Finding
	err := json.Unmarshal([]byte(raw), &f)
	if err == nil {
		t.Fatal("expected error for invalid severity 'critical'")
	}
}

func TestFinding_UnmarshalEmptySeverity(t *testing.T) {
	raw := `{"facet":"f","severity":"","file":"x.go","line":1,"description":"d","cycle":1}`
	var f Finding
	err := json.Unmarshal([]byte(raw), &f)
	if err == nil {
		t.Fatal("expected error for empty severity")
	}
}

func TestFinding_UnmarshalMissingSeverity(t *testing.T) {
	raw := `{"facet":"f","file":"x.go","line":1,"description":"d","cycle":1}`
	var f Finding
	err := json.Unmarshal([]byte(raw), &f)
	if err == nil {
		t.Fatal("expected error for missing severity field")
	}
}

func TestFinding_UnmarshalMissingFile(t *testing.T) {
	// File is a string field with no custom validation, so missing file
	// results in zero value (empty string). This test documents the behavior.
	raw := `{"facet":"f","severity":"error","line":1,"description":"d","cycle":1}`
	var f Finding
	err := json.Unmarshal([]byte(raw), &f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.File != "" {
		t.Errorf("expected empty File, got %q", f.File)
	}
}

func TestFinding_UnmarshalMissingDescription(t *testing.T) {
	// Description is a string field with no custom validation, so missing
	// description results in zero value (empty string).
	raw := `{"facet":"f","severity":"warning","file":"x.go","line":1,"cycle":1}`
	var f Finding
	err := json.Unmarshal([]byte(raw), &f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Description != "" {
		t.Errorf("expected empty Description, got %q", f.Description)
	}
}

func TestFinding_UnmarshalMalformedJSON(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"truncated", `{"facet":"f","severity":"er`},
		{"not_json", `this is not json`},
		{"empty", ``},
		{"just_brace", `{`},
		{"wrong_type_line", `{"facet":"f","severity":"error","file":"x.go","line":"not_a_number","description":"d","cycle":1}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var f Finding
			err := json.Unmarshal([]byte(tc.raw), &f)
			if err == nil {
				t.Fatal("expected error for malformed JSON")
			}
		})
	}
}
