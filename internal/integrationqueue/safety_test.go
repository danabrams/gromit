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

func TestValidateSafety_ViolationDetailsCorrect(t *testing.T) {
	tests := []struct {
		name             string
		changedFiles     []string
		wantViolatedFile string
		wantViolationType string
	}{
		{
			name:             "dolt violation",
			changedFiles:     []string{".dolt/config"},
			wantViolatedFile: ".dolt/config",
			wantViolationType: "dolt_artifact",
		},
		{
			name:             "beads_gromit violation",
			changedFiles:     []string{"beads_gromit/file.json"},
			wantViolatedFile: "beads_gromit/file.json",
			wantViolationType: "beads_gromit_artifact",
		},
		{
			name:             "lock file violation",
			changedFiles:     []string{"package-lock.json"},
			wantViolatedFile: "package-lock.json",
			wantViolationType: "lock_file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diag := ValidateSafety(tt.changedFiles)
			if diag == nil {
				t.Fatalf("expected violation, got nil")
			}
			if diag.ViolatedFile != tt.wantViolatedFile {
				t.Errorf("ViolatedFile = %q, want %q", diag.ViolatedFile, tt.wantViolatedFile)
			}
			if diag.ViolationType != tt.wantViolationType {
				t.Errorf("ViolationType = %q, want %q", diag.ViolationType, tt.wantViolationType)
			}
		})
	}
}
