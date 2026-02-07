package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilterUnplannedSpecs(t *testing.T) {
	tests := []struct {
		name         string
		specs        []string
		planFiles    []string
		wantFiltered []string
	}{
		{
			name:         "empty input",
			specs:        []string{},
			planFiles:    []string{},
			wantFiltered: []string{},
		},
		{
			name:         "no plans exist",
			specs:        []string{"spec1.md", "spec2.md", "spec3.md"},
			planFiles:    []string{},
			wantFiltered: []string{"spec1.md", "spec2.md", "spec3.md"},
		},
		{
			name:         "all plans exist",
			specs:        []string{"spec1.md", "spec2.md", "spec3.md"},
			planFiles:    []string{"spec1.md", "spec2.md", "spec3.md"},
			wantFiltered: []string{},
		},
		{
			name:         "mixed - some plans exist",
			specs:        []string{"spec1.md", "spec2.md", "spec3.md", "spec4.md"},
			planFiles:    []string{"spec1.md", "spec3.md"},
			wantFiltered: []string{"spec2.md", "spec4.md"},
		},
		{
			name:         "single spec with plan",
			specs:        []string{"feature-x.md"},
			planFiles:    []string{"feature-x.md"},
			wantFiltered: []string{},
		},
		{
			name:         "single spec without plan",
			specs:        []string{"feature-y.md"},
			planFiles:    []string{},
			wantFiltered: []string{"feature-y.md"},
		},
		{
			name:         "plans exist but not for these specs",
			specs:        []string{"spec-a.md", "spec-b.md"},
			planFiles:    []string{"other-plan.md", "unrelated.md"},
			wantFiltered: []string{"spec-a.md", "spec-b.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directories
			specsDir := t.TempDir()
			plansDir := t.TempDir()

			// Create spec files and build full paths
			fullSpecPaths := []string{}
			for _, specFile := range tt.specs {
				specPath := filepath.Join(specsDir, specFile)
				if err := os.WriteFile(specPath, []byte("spec content"), 0644); err != nil {
					t.Fatalf("failed to create spec file %s: %v", specFile, err)
				}
				fullSpecPaths = append(fullSpecPaths, specPath)
			}

			// Create plan files
			for _, planFile := range tt.planFiles {
				planPath := filepath.Join(plansDir, planFile)
				if err := os.WriteFile(planPath, []byte("plan content"), 0644); err != nil {
					t.Fatalf("failed to create plan file %s: %v", planFile, err)
				}
			}

			// Run the filter
			got := filterUnplannedSpecs(fullSpecPaths, plansDir)

			// Build expected full paths
			wantFullPaths := []string{}
			for _, wantFile := range tt.wantFiltered {
				wantFullPaths = append(wantFullPaths, filepath.Join(specsDir, wantFile))
			}

			// Compare results
			if len(got) != len(wantFullPaths) {
				t.Errorf("filterUnplannedSpecs() returned %d specs, want %d\ngot:  %v\nwant: %v",
					len(got), len(wantFullPaths), got, wantFullPaths)
				return
			}

			// Check each result
			for i, gotPath := range got {
				if gotPath != wantFullPaths[i] {
					t.Errorf("filterUnplannedSpecs()[%d] = %v, want %v", i, gotPath, wantFullPaths[i])
				}
			}
		})
	}
}
