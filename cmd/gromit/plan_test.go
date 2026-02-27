package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/worktree"
	"github.com/spf13/cobra"
)

// planLaunchTestAgent is now defined in session_test_agent_helper.go
// using the shared sessionTestAgent test helper
type planLaunchTestAgent = sessionTestAgent

func TestFilterUnplannedSpecs(t *testing.T) {
	t.Parallel()
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
			t.Parallel(
			// Create temporary directories
			)

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

func TestLaunchPlanSession_ConvertsPromptPathToAbsolute(t *testing.T) {

	origLauncher := planSessionLauncherFn
	t.Cleanup(func() { planSessionLauncherFn = origLauncher })

	enabled := false
	cfg := &config.Config{}
	cfg.Worktree.Enabled = &enabled

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() failed: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) failed: %v", tmpDir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	relativePromptPath := filepath.Join(".gromit", "tmp", "plan-prompt.md")
	if err := os.MkdirAll(filepath.Dir(relativePromptPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}
	if err := os.WriteFile(relativePromptPath, []byte("prompt"), 0o644); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	capturedPromptPath := ""
	agent := &planLaunchTestAgent{
		launchInDirFn: func(promptPath, dir string) error {
			capturedPromptPath = promptPath
			return nil
		},
	}

	if err := launchPlanSession(cfg, ".gromit", agent, relativePromptPath); err != nil {
		t.Fatalf("launchPlanSession() error = %v", err)
	}

	if !filepath.IsAbs(capturedPromptPath) {
		t.Fatalf("prompt path = %q, want absolute path", capturedPromptPath)
	}
}

func TestRunPlanDelegatesToPipelineAndReportsSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd(): %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) failed: %v", tmpDir, err)
	}

	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("creating specs dir: %v", err)
	}
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatalf("creating plans dir: %v", err)
	}

	specName := "sample-spec"
	specPath := filepath.Join(specsDir, specName+".md")
	if err := os.WriteFile(specPath, []byte("# Sample spec"), 0644); err != nil {
		t.Fatalf("writing spec file: %v", err)
	}

	configDst := filepath.Join(tmpDir, "gromit.yaml")
	if err := copyFileWithSuffix(origWD, "gromit.yaml", configDst, "\nworktree:\n  enabled: false\n"); err != nil {
		t.Fatalf("preparing config: %v", err)
	}
	origConfigPath := configPath
	configPath = configDst
	t.Cleanup(func() { configPath = origConfigPath })

	origPlanForce := planForce
	planForce = true
	t.Cleanup(func() { planForce = origPlanForce })

	origPlanNoChain := planNoChain
	planNoChain = true
	t.Cleanup(func() { planNoChain = origPlanNoChain })

	stub := &capturingPlanExecutor{
		t:        t,
		plansDir: plansDir,
	}
	origFactory := createPlanPipelineFn
	createPlanPipelineFn = func(cfg *config.Config, gDir, specs string, plans string) (planExecutor, error) {
		return stub, nil
	}
	t.Cleanup(func() { createPlanPipelineFn = origFactory })

	cmd := &cobra.Command{}
	cmd.Flags().String("agent", "", "")
	cmd.Flags().Bool("choose-agent", false, "")
	if err := cmd.Flags().Set("agent", "cli-agent"); err != nil {
		t.Fatalf("setting agent flag: %v", err)
	}
	if err := cmd.Flags().Set("choose-agent", "true"); err != nil {
		t.Fatalf("setting choose-agent flag: %v", err)
	}

	output := captureRunPlanStdout(t, func() {
		if err := runPlan(cmd, []string{specName}); err != nil {
			t.Fatalf("runPlan failed: %v", err)
		}
	})

	planPath := filepath.Join(plansDir, specName+".md")
	if !strings.Contains(output, planPath) {
		t.Fatalf("expected output to mention %q, got:\n%s", planPath, output)
	}

	if stub.input.SpecName != specName {
		t.Fatalf("PlanInput.SpecName = %q, want %q", stub.input.SpecName, specName)
	}
	if !stub.input.Force {
		t.Fatalf("PlanInput.Force = false, want true")
	}
	if stub.input.AgentName != "cli-agent" {
		t.Fatalf("PlanInput.AgentName = %q, want %q", stub.input.AgentName, "cli-agent")
	}
	if !stub.input.ChooseAgent {
		t.Fatalf("PlanInput.ChooseAgent = false, want true")
	}
	if stub.input.LaunchDir != "" {
		t.Fatalf("PlanInput.LaunchDir = %q, want empty", stub.input.LaunchDir)
	}
}

type capturingPlanExecutor struct {
	t        *testing.T
	plansDir string
	input    pipeline.PlanInput
}

func (c *capturingPlanExecutor) Plan(ctx context.Context, input pipeline.PlanInput) (*pipeline.PlanSession, error) {
	c.input = input
	planPath := filepath.Join(c.plansDir, input.SpecName+".md")
	if err := os.WriteFile(planPath, []byte("# plan"), 0644); err != nil {
		c.t.Fatalf("writing plan file: %v", err)
	}
	return pipeline.NewPlanSession(func() {}), nil
}

type captureResult struct {
	output string
	err    error
}

func captureRunPlanStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdout pipe: %v", err)
	}

	origStdout := os.Stdout
	os.Stdout = w
	defer func() {
		os.Stdout = origStdout
	}()

	done := make(chan captureResult, 1)
	go func() {
		var buf strings.Builder
		_, copyErr := io.Copy(&buf, r)
		done <- captureResult{output: buf.String(), err: copyErr}
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("closing pipe writer: %v", err)
	}

	result := <-done
	if result.err != nil {
		t.Fatalf("capturing stdout: %v", result.err)
	}

	return result.output
}

func copyFileWithSuffix(srcDir, srcName, dstPath, suffix string) error {
	srcPath := filepath.Join(srcDir, srcName)
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	if suffix != "" {
		data = append(data, []byte(suffix)...)
	}
	return os.WriteFile(dstPath, data, 0644)
}
