package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/worktree"
)

type planLaunchTestAgent struct {
	launchInDirFn func(promptPath, dir string) error
}

func (a *planLaunchTestAgent) Name() string { return "plan-test-agent" }

func (a *planLaunchTestAgent) Launch(promptPath string) error {
	return a.LaunchInDir(promptPath, "")
}

func (a *planLaunchTestAgent) LaunchInDir(promptPath, dir string) error {
	if a != nil && a.launchInDirFn != nil {
		return a.launchInDirFn(promptPath, dir)
	}
	return nil
}

func (a *planLaunchTestAgent) Command(promptPath string) (*exec.Cmd, error) {
	return nil, errors.New("not implemented")
}

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

func TestLaunchPlanSession_UsesSessionLauncherWhenEnabled(t *testing.T) {
	origLauncher := planSessionLauncherFn
	t.Cleanup(func() { planSessionLauncherFn = origLauncher })

	sessionDir := t.TempDir()
	var launchedDir string
	launcherCalled := false

	planSessionLauncherFn = func(
		gromitDir string,
		command string,
		conflictSettings sessionConflictSettings,
		callback func(sessionDir string) error,
	) (*worktree.SessionWorktree, error) {
		launcherCalled = true
		if command != planSessionCommand {
			t.Fatalf("command = %q, want %q", command, planSessionCommand)
		}
		if err := callback(sessionDir); err != nil {
			return nil, err
		}
		return &worktree.SessionWorktree{BranchName: "gromit/plan-test", WorktreeDir: sessionDir}, nil
	}

	agent := &planLaunchTestAgent{
		launchInDirFn: func(promptPath, dir string) error {
			launchedDir = dir
			return nil
		},
	}

	if err := launchPlanSession(&config.Config{}, ".gromit", agent, "prompt.md"); err != nil {
		t.Fatalf("launchPlanSession() error = %v", err)
	}
	if !launcherCalled {
		t.Fatal("expected session launcher to be called")
	}
	if launchedDir != sessionDir {
		t.Fatalf("launch dir = %q, want %q", launchedDir, sessionDir)
	}
}

func TestLaunchPlanSession_WorktreeDisabledUsesInPlaceLaunch(t *testing.T) {
	origLauncher := planSessionLauncherFn
	t.Cleanup(func() { planSessionLauncherFn = origLauncher })

	enabled := false
	cfg := &config.Config{}
	cfg.Worktree.Enabled = &enabled

	launcherCalled := false
	planSessionLauncherFn = func(
		gromitDir string,
		command string,
		conflictSettings sessionConflictSettings,
		callback func(sessionDir string) error,
	) (*worktree.SessionWorktree, error) {
		launcherCalled = true
		return nil, nil
	}

	var launchedDir string
	agent := &planLaunchTestAgent{
		launchInDirFn: func(promptPath, dir string) error {
			launchedDir = dir
			return nil
		},
	}

	if err := launchPlanSession(cfg, ".gromit", agent, "prompt.md"); err != nil {
		t.Fatalf("launchPlanSession() error = %v", err)
	}
	if launcherCalled {
		t.Fatal("session launcher should not be called when worktree is disabled")
	}
	if launchedDir != "" {
		t.Fatalf("launch dir = %q, want empty string", launchedDir)
	}
}
