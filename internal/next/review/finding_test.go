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
