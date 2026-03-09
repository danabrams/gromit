package finding

import "testing"

func TestHasCritical(t *testing.T) {
	cases := []struct {
		name     string
		findings []Finding
		want     bool
	}{
		{
			name: "contains critical",
			findings: []Finding{
				{Severity: SeverityWarning},
				{Severity: SeverityCritical},
			},
			want: true,
		},
		{
			name: "no critical",
			findings: []Finding{
				{Severity: SeverityWarning},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := HasCritical(tc.findings); got != tc.want {
				t.Fatalf("HasCritical(%v) = %v, want %v", tc.findings, got, tc.want)
			}
		})
	}
}

func TestFindingNormalizeNilFields(t *testing.T) {
	f := &Finding{}
	f.NormalizeNilFields()
	if f.AffectedFiles == nil {
		t.Fatalf("AffectedFiles should be non-nil after NormalizeNilFields")
	}
	if len(f.AffectedFiles) != 0 {
		t.Fatalf("AffectedFiles length = %d, want 0", len(f.AffectedFiles))
	}

	var nilFinding *Finding
	nilFinding.NormalizeNilFields() // should not panic
}

func TestFindingCategoryAndFields(t *testing.T) {
	f := Finding{
		Category:    CategoryArchitecture,
		Scope:       "stage",
		Description: "desc",
	}
	if f.Category != CategoryArchitecture {
		t.Fatalf("Category field mismatch: got %v", f.Category)
	}
	if f.Scope != "stage" {
		t.Fatalf("Scope field mismatch: got %v", f.Scope)
	}
	if f.Description != "desc" {
		t.Fatalf("Description field mismatch: got %v", f.Description)
	}

	expected := map[Category]string{
		CategoryBug:          "bug",
		CategorySecurity:     "security",
		CategoryQuality:      "quality",
		CategoryTestGap:      "test_gap",
		CategoryArchitecture: "architecture",
		CategoryAcceptance:   "acceptance",
	}
	for cat, want := range expected {
		if string(cat) != want {
			t.Fatalf("Category %v has value %q, want %q", cat, string(cat), want)
		}
	}
}
