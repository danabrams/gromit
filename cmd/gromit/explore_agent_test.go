package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/worktree"
)

// TestExploreCommandHasAgentFlag verifies explore command has --agent flag.
func TestExploreCommandHasAgentFlag(t *testing.T) {
	// Expected failure: explore command does not define --agent flag or exploreAgentFlagName constant yet
	flag := exploreCmd.Flags().Lookup("agent")
	if flag == nil {
		t.Error("explore command missing --agent flag")
	}

	if flag != nil && flag.Value.Type() != "string" {
		t.Errorf("--agent flag type = %q, want %q", flag.Value.Type(), "string")
	}
}

// TestExploreCommandHasChooseAgentFlag verifies explore command has --choose-agent flag.
func TestExploreCommandHasChooseAgentFlag(t *testing.T) {
	// Expected failure: explore command does not define --choose-agent flag or exploreChooseAgentFlagName constant yet
	flag := exploreCmd.Flags().Lookup("choose-agent")
	if flag == nil {
		t.Error("explore command missing --choose-agent flag")
	}

	if flag != nil && flag.Value.Type() != "bool" {
		t.Errorf("--choose-agent flag type = %q, want %q", flag.Value.Type(), "bool")
	}
}

func TestRunExploreInSessionUsesSessionLauncher(t *testing.T) {
	origLauncher := exploreSessionLauncherFn
	origRunInDir := exploreRunInDirFn
	t.Cleanup(func() {
		exploreSessionLauncherFn = origLauncher
		exploreRunInDirFn = origRunInDir
	})

	sessionDir := filepath.Join(t.TempDir(), "session")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}

	targetGromit := filepath.Join(t.TempDir(), ".gromit")
	p, _ := newTestExplorePipeline(t, targetGromit)

	exploreSessionLauncherFn = func(gromitDir string, command string, conflict sessionConflictSettings, callback func(sessionDir string) error) (*worktree.SessionWorktree, error) {
		if command != exploreSessionCommand {
			t.Fatalf("command = %q, want %q", command, exploreSessionCommand)
		}
		if err := callback(sessionDir); err != nil {
			return nil, err
		}
		return &worktree.SessionWorktree{WorktreeDir: sessionDir}, nil
	}
	exploreRunInDirFn = func(dir string, fn func() error) error {
		if dir != sessionDir {
			t.Fatalf("runInDir called with %q, want %q", dir, sessionDir)
		}
		return fn()
	}

	result, err := runExploreInSession(context.Background(), &config.Config{}, targetGromit, p, pipeline.ExploreInput{Topic: "test"})
	if err != nil {
		t.Fatalf("runExploreInSession error = %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestRunExploreInSessionSkipsSessionLauncherWhenWorktreeDisabled(t *testing.T) {
	origLauncher := exploreSessionLauncherFn
	origRunInDir := exploreRunInDirFn
	t.Cleanup(func() {
		exploreSessionLauncherFn = origLauncher
		exploreRunInDirFn = origRunInDir
	})

	exploreSessionLauncherFn = func(gromitDir string, command string, conflict sessionConflictSettings, callback func(sessionDir string) error) (*worktree.SessionWorktree, error) {
		t.Fatalf("session launcher should not be called when worktree is disabled")
		return nil, nil
	}
	exploreRunInDirFn = func(string, func() error) error {
		t.Fatalf("exploreRunInDir should not be invoked when worktree is disabled")
		return nil
	}

	targetGromit := filepath.Join(t.TempDir(), ".gromit")
	p, _ := newTestExplorePipeline(t, targetGromit)
	cfg := &config.Config{Worktree: config.WorktreeConfig{Enabled: boolPtr(false)}}

	result, err := runExploreInSession(context.Background(), cfg, targetGromit, p, pipeline.ExploreInput{Topic: "test"})
	if err != nil {
		t.Fatalf("runExploreInSession error = %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestExplorePhaseConfigSelectsAgent_Reclassified(t *testing.T) {
	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Definitions: map[string]config.AgentDefinition{
				"test-agent": {
					Binary: "echo",
				},
			},
			Phases: config.PhasesConfig{
				Explore: "test-agent",
			},
		},
	}

	resolver := &cmdAgentResolver{cfg: cfg}
	agent, err := resolver.Resolve(exploreSessionCommand, "", false)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if agent.Name() != "test-agent" {
		t.Fatalf("agent.Name() = %q, want %q", agent.Name(), "test-agent")
	}
}

func newTestExplorePipeline(t *testing.T, gromitDir string) (*pipeline.Pipeline, *stubPipelineAgent) {
	t.Helper()
	specsDir := filepath.Join(gromitDir, "specs")
	epicsDir := filepath.Join(gromitDir, "epics")
	for _, dir := range []string{gromitDir, specsDir, epicsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	agent := &stubPipelineAgent{}
	deps := &pipeline.Deps{
		AgentResolver:  &stubAgentResolver{agent: agent},
		ExploreRenderer: stubExploreRenderer{},
		BacklogClient:  stubBacklogClient{},
	}
	paths := &pipeline.Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		EpicsDir:  epicsDir,
	}
	return pipeline.New(deps, paths), agent
}

type stubExploreRenderer struct{}

func (stubExploreRenderer) RenderExplore(input *pipeline.ExplorePromptInput) (string, error) {
	return "prompt", nil
}

type stubBacklogClient struct{}

func (stubBacklogClient) List() ([]*pipeline.Idea, error)       { return nil, nil }
func (stubBacklogClient) Get(id string) (*pipeline.Idea, error) { return nil, nil }
func (stubBacklogClient) Add(item *pipeline.Idea) error         { return nil }
func (stubBacklogClient) Update(id string, fn func(*pipeline.Idea)) error {
	return nil
}

type stubAgentResolver struct {
	agent pipeline.Agent
}

func (r *stubAgentResolver) Resolve(phase string, flagOverride string, choosePicker bool) (pipeline.Agent, error) {
	return r.agent, nil
}

type stubPipelineAgent struct {
	lastLaunchDir string
}

func (a *stubPipelineAgent) Name() string { return "explore-stub" }
func (a *stubPipelineAgent) Launch(promptPath string) error {
	a.lastLaunchDir = ""
	return nil
}
func (a *stubPipelineAgent) LaunchInDir(promptPath, dir string) error {
	a.lastLaunchDir = dir
	return nil
}
