package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/worktree"
	"github.com/spf13/cobra"
)

func TestShowRefinePickerEOFSelectsNew(t *testing.T) {
	t.Parallel()
	unrefined := []*backlog.Idea{
		{ID: "idea-1", Text: "First", Type: "task"},
	}

	choice := showRefinePicker(unrefined, strings.NewReader(""))

	if choice != len(unrefined) {
		t.Fatalf("expected choice %d for EOF input, got %d", len(unrefined), choice)
	}
}

func TestShowRefinePickerNonNumericInputSelectsNew(t *testing.T) {
	t.Parallel()
	unrefined := []*backlog.Idea{
		{ID: "idea-1", Text: "First", Type: "task"},
		{ID: "idea-2", Text: "Second", Type: "feature"},
	}

	choice := showRefinePicker(unrefined, strings.NewReader("abc\n"))

	if choice != len(unrefined) {
		t.Fatalf("expected choice %d for non-numeric input, got %d", len(unrefined), choice)
	}
}

func TestShowRefinePickerZeroInputSelectsNew(t *testing.T) {
	t.Parallel()
	unrefined := []*backlog.Idea{
		{ID: "idea-1", Text: "First", Type: "task"},
	}

	choice := showRefinePicker(unrefined, strings.NewReader("0\n"))

	if choice != len(unrefined) {
		t.Fatalf("expected choice %d for zero input, got %d", len(unrefined), choice)
	}
}

func TestShowRefinePickerOutOfBoundsHighSelectsNew(t *testing.T) {
	t.Parallel()
	unrefined := []*backlog.Idea{
		{ID: "idea-1", Text: "First", Type: "task"},
		{ID: "idea-2", Text: "Second", Type: "feature"},
	}

	// Max valid choice is 3 (len+1 = "something new"), so 99 is out of bounds
	choice := showRefinePicker(unrefined, strings.NewReader("99\n"))

	if choice != len(unrefined) {
		t.Fatalf("expected choice %d for out-of-bounds input, got %d", len(unrefined), choice)
	}
}

func TestShowRefinePickerValidInputSelectsCorrectItem(t *testing.T) {
	t.Parallel()
	unrefined := []*backlog.Idea{
		{ID: "idea-1", Text: "First", Type: "task"},
		{ID: "idea-2", Text: "Second", Type: "feature"},
		{ID: "idea-3", Text: "Third", Type: "bug"},
	}

	// Selecting "2" should return index 1 (Second item)
	choice := showRefinePicker(unrefined, strings.NewReader("2\n"))

	if choice != 1 {
		t.Fatalf("expected choice 1 for input '2', got %d", choice)
	}
}

func TestShowRefinePickerSomethingNewOptionSelectsCorrectly(t *testing.T) {
	t.Parallel()
	unrefined := []*backlog.Idea{
		{ID: "idea-1", Text: "First", Type: "task"},
		{ID: "idea-2", Text: "Second", Type: "feature"},
	}

	// Selecting "3" (len+1) is the "Something new..." option
	choice := showRefinePicker(unrefined, strings.NewReader("3\n"))

	if choice != len(unrefined) {
		t.Fatalf("expected choice %d for 'something new' input, got %d", len(unrefined), choice)
	}
}

func TestShowRefinePickerNegativeInputSelectsNew(t *testing.T) {
	t.Parallel()
	unrefined := []*backlog.Idea{
		{ID: "idea-1", Text: "First", Type: "task"},
	}

	choice := showRefinePicker(unrefined, strings.NewReader("-1\n"))

	if choice != len(unrefined) {
		t.Fatalf("expected choice %d for negative input, got %d", len(unrefined), choice)
	}
}

func TestRunRefineReturnsErrorWhenPipelineCreationFails(t *testing.T) {

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(configFile, []byte("loop:\n  max_iterations: 1\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	origConfigPath := configPath
	configPath = configFile
	t.Cleanup(func() {
		configPath = origConfigPath
	})

	origFactory := createRefinePipelineFn
	createRefinePipelineFn = func(_ *config.Config, _, _, _ string) (*pipeline.Pipeline, error) {
		return nil, errors.New("factory failed")
	}
	t.Cleanup(func() {
		createRefinePipelineFn = origFactory
	})

	cmd := &cobra.Command{}
	cmd.Flags().String("agent", "", "")

	err := runRefine(cmd, []string{"ad-hoc idea"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "factory failed") {
		t.Fatalf("expected error to include factory failure, got %v", err)
	}
}

// refineSessionTestAgent is now defined in session_test_agent_helper.go
// using the shared sessionTestAgent test helper
type refineSessionTestAgent = sessionTestAgent

type refineSessionTestResolver struct {
	agent pipeline.Agent
}

func (r *refineSessionTestResolver) Resolve(phase string, flagOverride string, choosePicker bool) (pipeline.Agent, error) {
	return r.agent, nil
}

func TestRunRefineInSession_UsesSessionLauncherWhenEnabled(t *testing.T) {

	origLauncher := refineSessionLauncherFn
	origRunInDir := refineRunInDirFn
	t.Cleanup(func() {
		refineSessionLauncherFn = origLauncher
		refineRunInDirFn = origRunInDir
	})

	baseDir := t.TempDir()
	t.Chdir(baseDir)
	gromitDir := filepath.Join(baseDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}

	sessionDir := t.TempDir()
	launcherCalled := false
	runInDirArg := ""
	agentWD := ""

	refineSessionLauncherFn = func(
		gromitDir string,
		command string,
		conflictSettings sessionConflictSettings,
		callback func(sessionDir string) error,
	) (*worktree.SessionWorktree, error) {
		launcherCalled = true
		if command != refineSessionCommand {
			t.Fatalf("command = %q, want %q", command, refineSessionCommand)
		}
		if err := callback(sessionDir); err != nil {
			return nil, err
		}
		return &worktree.SessionWorktree{BranchName: "gromit/refine-test", WorktreeDir: sessionDir}, nil
	}
	refineRunInDirFn = func(dir string, fn func() error) error {
		runInDirArg = dir
		return runInDir(dir, fn)
	}

	agent := &refineSessionTestAgent{
		launchInDirFn: func(promptPath, dir string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			agentWD = wd
			return nil
		},
	}
	p := pipeline.New(&pipeline.Deps{
		AgentResolver: &refineSessionTestResolver{agent: agent},
	}, &pipeline.Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
	})

	result, err := runRefineInSession(context.Background(), &config.Config{}, gromitDir, p, pipeline.RefineInput{IdeaText: "idea"})
	if err != nil {
		t.Fatalf("runRefineInSession() error = %v", err)
	}
	if result == nil {
		t.Fatal("runRefineInSession() returned nil result")
	}
	if !launcherCalled {
		t.Fatal("expected session launcher to be called")
	}
	if runInDirArg != sessionDir {
		t.Fatalf("runInDir called with %q, want %q", runInDirArg, sessionDir)
	}
	if agentWD != sessionDir {
		t.Fatalf("agent launched from %q, want %q", agentWD, sessionDir)
	}
}

func TestRunRefineInSession_WorktreeDisabledSkipsSessionLauncher(t *testing.T) {

	origLauncher := refineSessionLauncherFn
	t.Cleanup(func() { refineSessionLauncherFn = origLauncher })

	baseDir := t.TempDir()
	t.Chdir(baseDir)
	gromitDir := filepath.Join(baseDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}

	launcherCalled := false
	refineSessionLauncherFn = func(
		gromitDir string,
		command string,
		conflictSettings sessionConflictSettings,
		callback func(sessionDir string) error,
	) (*worktree.SessionWorktree, error) {
		launcherCalled = true
		return nil, nil
	}

	enabled := false
	cfg := &config.Config{}
	cfg.Worktree.Enabled = &enabled

	agent := &refineSessionTestAgent{}
	p := pipeline.New(&pipeline.Deps{
		AgentResolver: &refineSessionTestResolver{agent: agent},
	}, &pipeline.Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
	})

	if _, err := runRefineInSession(context.Background(), cfg, gromitDir, p, pipeline.RefineInput{IdeaText: "idea"}); err != nil {
		t.Fatalf("runRefineInSession() error = %v", err)
	}
	if launcherCalled {
		t.Fatal("session launcher should not be called when worktree is disabled")
	}
}

func TestDetermineRefineInput_ChooseAgentWithIdeaID(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("mkdir gromitDir: %v", err)
	}

	// Create an empty backlog file so the picker logic doesn't try to read it
	backlogFile := filepath.Join(gromitDir, "backlog.json")
	if err := os.WriteFile(backlogFile, []byte("[]"), 0644); err != nil {
		t.Fatalf("write backlog file: %v", err)
	}

	cmd := &cobra.Command{}
	cmd.Flags().String("agent", "", "")
	cmd.Flags().Bool("choose-agent", false, "")

	// Set flags
	cmd.Flag("choose-agent").Value.Set("true")

	// Test with backlog ID argument
	input, err := determineRefineInput(cmd, []string{"idea-123"}, gromitDir, filepath.Join(gromitDir, "specs"))
	if err != nil {
		t.Fatalf("determineRefineInput() error = %v", err)
	}

	if !input.ChooseAgent {
		t.Errorf("ChooseAgent = %v, want true when --choose-agent flag is set", input.ChooseAgent)
	}

	if input.IdeaID != "idea-123" {
		t.Errorf("IdeaID = %q, want %q", input.IdeaID, "idea-123")
	}
}

func TestDetermineRefineInput_ChooseAgentWithIdeaText(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{}
	cmd.Flags().String("agent", "", "")
	cmd.Flags().Bool("choose-agent", false, "")

	if err := cmd.Flags().Set("choose-agent", "true"); err != nil {
		t.Fatalf("set choose-agent flag: %v", err)
	}

	input, err := determineRefineInput(cmd, []string{"an ad-hoc idea"}, "", "")
	if err != nil {
		t.Fatalf("determineRefineInput() error = %v", err)
	}

	if !input.ChooseAgent {
		t.Errorf("ChooseAgent = %v, want true when --choose-agent flag is set", input.ChooseAgent)
	}

	if input.IdeaText != "an ad-hoc idea" {
		t.Errorf("IdeaText = %q, want %q", input.IdeaText, "an ad-hoc idea")
	}
}

func TestFilterRefineCandidates_ExcludesTerminalAndSpecLinkedIdeas(t *testing.T) {
	t.Parallel()

	ideas := []*backlog.Idea{
		{ID: "idea-open", Text: "open"},
		{ID: "idea-refined", Text: "refined", Status: "refined"},
		{ID: "idea-closed", Text: "closed", Status: "closed"},
		{ID: "idea-rejected", Text: "rejected", Status: "rejected"},
		{ID: "idea-specname", Text: "specname", SpecName: "already-linked"},
		{ID: "idea-source", Text: "source-linked"},
		nil,
	}

	refinedBySpec := map[string]struct{}{
		"idea-source": {},
	}

	got := filterRefineCandidates(ideas, refinedBySpec)
	if len(got) != 1 {
		t.Fatalf("len(filterRefineCandidates()) = %d, want 1", len(got))
	}
	if got[0].ID != "idea-open" {
		t.Fatalf("filterRefineCandidates()[0].ID = %q, want %q", got[0].ID, "idea-open")
	}
}

func TestLoadRefinedIdeaIDsFromSpecs_ParsesSourceIdeasFrontmatter(t *testing.T) {
	t.Parallel()

	specsDir := t.TempDir()
	spec1 := filepath.Join(specsDir, "one.md")
	spec2 := filepath.Join(specsDir, "two.md")
	spec3 := filepath.Join(specsDir, "three.md")

	spec1Content := `---
id: one
source_ideas:
  - idea-100
  - idea-200
---
# One
`
	spec2Content := `---
id: two
source_ideas: idea-300
---
# Two
`
	spec3Content := `---
id: three
source_ideas:
  - 123
---
# Three
`

	if err := os.WriteFile(spec1, []byte(spec1Content), 0o644); err != nil {
		t.Fatalf("write spec1: %v", err)
	}
	if err := os.WriteFile(spec2, []byte(spec2Content), 0o644); err != nil {
		t.Fatalf("write spec2: %v", err)
	}
	if err := os.WriteFile(spec3, []byte(spec3Content), 0o644); err != nil {
		t.Fatalf("write spec3: %v", err)
	}

	ids, err := loadRefinedIdeaIDsFromSpecs(specsDir)
	if err != nil {
		t.Fatalf("loadRefinedIdeaIDsFromSpecs() error = %v", err)
	}

	for _, id := range []string{"idea-100", "idea-200", "idea-300"} {
		if _, ok := ids[id]; !ok {
			t.Fatalf("expected %q in result map", id)
		}
	}
	for _, id := range []string{"123", "idea-123"} {
		if _, ok := ids[id]; !ok {
			t.Fatalf("expected %q in result map for numeric legacy source_ideas", id)
		}
	}
}
