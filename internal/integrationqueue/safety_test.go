package integrationqueue

import "testing"

func TestValidateSafety_DetectsProhibitedArtifacts(t *testing.T) {
	tests := []struct {
		name          string
		changedFiles  []string
		wantViolation bool
	}{
		{
			name:          "dolt directory",
			changedFiles:  []string{"cmd/main.go", ".dolt/config"},
			wantViolation: true,
		},
		{
			name:          "doltcfg directory",
			changedFiles:  []string{".doltcfg/local_config"},
			wantViolation: true,
		},
		{
			name:          "beads_gromit directory",
			changedFiles:  []string{"beads_gromit/some_file"},
			wantViolation: true,
		},
		{
			name:          "gromit state.json",
			changedFiles:  []string{".gromit/state.json"},
			wantViolation: true,
		},
		{
			name:          "gromit stats.json",
			changedFiles:  []string{".gromit/stats.json"},
			wantViolation: true,
		},
		{
			name:          "gromit interactive-state.json",
			changedFiles:  []string{".gromit/interactive-state.json"},
			wantViolation: true,
		},
		{
			name:          "lock file",
			changedFiles:  []string{"package-lock.json"},
			wantViolation: true,
		},
		{
			name:          "normal source files",
			changedFiles:  []string{"cmd/main.go", "internal/foo/bar.go"},
			wantViolation: false,
		},
		{
			name:          "approved fixture path",
			changedFiles:  []string{"test/fixtures/foo.json"},
			wantViolation: false,
		},
		{
			name:          "approved curated path",
			changedFiles:  []string{".gromit/reports/curated/report.json"},
			wantViolation: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diag := ValidateSafety(tt.changedFiles)
			hasViolation := diag != nil
			if hasViolation != tt.wantViolation {
				t.Errorf("ValidateSafety() violation = %v, want %v", hasViolation, tt.wantViolation)
			}
		})
	}
}
