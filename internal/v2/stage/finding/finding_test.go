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
