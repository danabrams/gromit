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
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/worktree"
)

// setupExploreTest creates a temp directory with the standard explore test
// structure: .gromit/ with templates/, CLAUDE.md, and config.
//
// Note: After extracting explore logic to internal/pipeline, this helper
// is preserved for CLI adapter tests that need to verify command setup.
func setupExploreTest(t *testing.T) (*config.Config, string) {
	t.Helper()

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	templatesDir := filepath.Join(gromitDir, "templates")

	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	if err := os.WriteFile(claudeMDPath, []byte("# Project\n\nThis is project context."), 0644); err != nil {
		t.Fatalf("failed to create CLAUDE.md: %v", err)
	}

	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       templatesDir,
			ProjectClaudeMD: claudeMDPath,
			GromitDir:       gromitDir,
		},
	}

	return cfg, gromitDir
}

func TestBuildExplorePipeline_NilConfigUsesResolvedDefaults(t *testing.T) {
	t.Chdir(t.TempDir())

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("buildExplorePipeline(nil) panicked: %v", r)
		}
	}()

	p, err := buildExplorePipeline(nil)
	if err != nil {
		t.Fatalf("buildExplorePipeline(nil) returned error: %v", err)
	}
	if p == nil {
		t.Fatal("buildExplorePipeline(nil) returned nil pipeline")
	}
}

func TestExplorePromptRenderer_RenderExploreBuildsPromptDiagnostics(t *testing.T) {
	cfg, gromitDir := setupExploreTest(t)

	if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte("## Process\n- Keep it simple\n"), 0o644); err != nil {
		t.Fatalf("failed to create RULES.md: %v", err)
	}

	renderer, err := prompt.NewRenderer(
		cfg.Paths.Templates,
		filepath.Join(gromitDir, "specs"),
		cfg.Paths.ProjectClaudeMD,
		gromitDir,
	)
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}

	adapter := &explorePromptRenderer{renderer: renderer}
	const topic = "Improve onboarding flow"
	if _, err := adapter.RenderExplore(&pipeline.ExplorePromptInput{Query: topic}); err != nil {
		t.Fatalf("RenderExplore() error = %v", err)
	}

	diagnostics := adapter.LastDiagnostics()
	if diagnostics == nil {
		t.Fatal("LastDiagnostics() = nil, want non-nil")
	}
	if diagnostics.PromptType != "explore" {
		t.Fatalf("PromptType = %q, want %q", diagnostics.PromptType, "explore")
	}

	for _, key := range []string{
		exploreSectionTopic,
		prompt.SectionClaudeMD,
		prompt.SectionRules,
		exploreSectionLearnings,
		exploreSectionInstructions,
	} {
		if _, ok := diagnostics.SectionTokens[key]; !ok {
			t.Fatalf("SectionTokens missing key %q", key)
		}
	}
}

type exploreSessionTestAgent struct {
	launchInDirFn func(promptPath, dir string) error
}

func (a *exploreSessionTestAgent) Name() string { return "explore-test-agent" }

func (a *exploreSessionTestAgent) Launch(promptPath string) error {
	return a.LaunchInDir(promptPath, "")
}

func (a *exploreSessionTestAgent) LaunchInDir(promptPath, dir string) error {
	if a != nil && a.launchInDirFn != nil {
		return a.launchInDirFn(promptPath, dir)
	}
	return nil
}

type exploreSessionTestResolver struct {
	agent pipeline.Agent
}

func (r *exploreSessionTestResolver) Resolve(phase string, flagOverride string, choosePicker bool) (pipeline.Agent, error) {
	return r.agent, nil
}

type exploreSessionTestRenderer struct{}

func (r *exploreSessionTestRenderer) RenderExplore(input *pipeline.ExplorePromptInput) (string, error) {
	return "explore prompt", nil
}

type exploreSessionTestBacklog struct{}

func (b *exploreSessionTestBacklog) List() ([]*pipeline.Idea, error) { return []*pipeline.Idea{}, nil }
func (b *exploreSessionTestBacklog) Get(id string) (*pipeline.Idea, error) {
	return nil, nil
}
func (b *exploreSessionTestBacklog) Add(item *pipeline.Idea) error { return nil }
func (b *exploreSessionTestBacklog) Update(id string, fn func(*pipeline.Idea)) error {
	return nil
}

func TestRunExploreInSession_UsesSessionLauncherWhenEnabled(t *testing.T) {
	origLauncher := exploreSessionLauncherFn
	origRunInDir := exploreRunInDirFn
	t.Cleanup(func() {
		exploreSessionLauncherFn = origLauncher
		exploreRunInDirFn = origRunInDir
	})

	baseDir := t.TempDir()
	t.Chdir(baseDir)
	gromitDir := filepath.Join(baseDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	epicsDir := filepath.Join(gromitDir, "epics")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}
	if err := os.MkdirAll(epicsDir, 0o755); err != nil {
		t.Fatalf("mkdir epics: %v", err)
	}

	sessionDir := t.TempDir()
	launcherCalled := false
	runInDirArg := ""
	agentWD := ""

	exploreSessionLauncherFn = func(
		gromitDir string,
		command string,
		conflictSettings sessionConflictSettings,
		callback func(sessionDir string) error,
	) (*worktree.SessionWorktree, error) {
		launcherCalled = true
		if command != exploreSessionCommand {
			t.Fatalf("command = %q, want %q", command, exploreSessionCommand)
		}
		if err := callback(sessionDir); err != nil {
			return nil, err
		}
		return &worktree.SessionWorktree{BranchName: "gromit/explore-test", WorktreeDir: sessionDir}, nil
	}
	exploreRunInDirFn = func(dir string, fn func() error) error {
		runInDirArg = dir
		return runInDir(dir, fn)
	}

	agent := &exploreSessionTestAgent{
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
		AgentResolver:   &exploreSessionTestResolver{agent: agent},
		ExploreRenderer: &exploreSessionTestRenderer{},
		BacklogClient:   &exploreSessionTestBacklog{},
	}, &pipeline.Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		EpicsDir:  epicsDir,
	})

	result, err := runExploreInSession(context.Background(), &config.Config{}, gromitDir, p, pipeline.ExploreInput{Topic: "topic"})
	if err != nil {
		t.Fatalf("runExploreInSession() error = %v", err)
	}
	if result == nil {
		t.Fatal("runExploreInSession() returned nil result")
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

func TestRunExploreInSession_WorktreeDisabledSkipsSessionLauncher(t *testing.T) {
	origLauncher := exploreSessionLauncherFn
	t.Cleanup(func() { exploreSessionLauncherFn = origLauncher })

	baseDir := t.TempDir()
	t.Chdir(baseDir)
	gromitDir := filepath.Join(baseDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	epicsDir := filepath.Join(gromitDir, "epics")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}
	if err := os.MkdirAll(epicsDir, 0o755); err != nil {
		t.Fatalf("mkdir epics: %v", err)
	}

	launcherCalled := false
	exploreSessionLauncherFn = func(
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

	p := pipeline.New(&pipeline.Deps{
		AgentResolver:   &exploreSessionTestResolver{agent: &exploreSessionTestAgent{}},
		ExploreRenderer: &exploreSessionTestRenderer{},
		BacklogClient:   &exploreSessionTestBacklog{},
	}, &pipeline.Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		EpicsDir:  epicsDir,
	})

	if _, err := runExploreInSession(context.Background(), cfg, gromitDir, p, pipeline.ExploreInput{Topic: "topic"}); err != nil {
		t.Fatalf("runExploreInSession() error = %v", err)
	}
	if launcherCalled {
		t.Fatal("session launcher should not be called when worktree is disabled")
	}
}

func TestHandleExploreOutput_RendersCreatedArtifacts(t *testing.T) {
	result := &pipeline.ExploreResult{
		CreatedEpics:        []string{"epic1.md", "epic2.md"},
		CreatedSpecs:        []string{"spec1.md"},
		CreatedBacklogItems: []string{"id1", "id2", "id3"},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	handleExploreOutput(result)

	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	data, _ := io.ReadAll(r)
	output := string(data)

	// Verify output contains expected content
	if !strings.Contains(output, "Epics created") {
		t.Fatalf("output missing 'Epics created', got: %q", output)
	}
	if !strings.Contains(output, "epic1.md") {
		t.Fatalf("output missing 'epic1.md', got: %q", output)
	}
	if !strings.Contains(output, "Specs created") {
		t.Fatalf("output missing 'Specs created', got: %q", output)
	}
	if !strings.Contains(output, "spec1.md") {
		t.Fatalf("output missing 'spec1.md', got: %q", output)
	}
	if !strings.Contains(output, "Backlog items created: 3") {
		t.Fatalf("output missing 'Backlog items created: 3', got: %q", output)
	}
}
