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
