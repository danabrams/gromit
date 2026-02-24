package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type refineLaunchInDirTestAgent struct {
	launchInDirFn func(promptPath, dir string) error
}

func (a *refineLaunchInDirTestAgent) Name() string { return "refine-test-agent" }

func (a *refineLaunchInDirTestAgent) Launch(promptPath string) error {
	return a.LaunchInDir(promptPath, "")
}

func (a *refineLaunchInDirTestAgent) LaunchInDir(promptPath, dir string) error {
	if a != nil && a.launchInDirFn != nil {
		return a.launchInDirFn(promptPath, dir)
	}
	return nil
}

type refineLaunchInDirTestResolver struct {
	agent Agent
}

func (r *refineLaunchInDirTestResolver) Resolve(phase string, flagOverride string, choosePicker bool) (Agent, error) {
	return r.agent, nil
}

type refineCreatedAtBacklogMock struct {
	added *Idea
}

func (m *refineCreatedAtBacklogMock) List() ([]*Idea, error) { return []*Idea{}, nil }

func (m *refineCreatedAtBacklogMock) Get(id string) (*Idea, error) { return nil, nil }

func (m *refineCreatedAtBacklogMock) Add(item *Idea) error {
	m.added = item
	return nil
}

func (m *refineCreatedAtBacklogMock) Update(id string, fn func(*Idea)) error { return nil }

func TestPipeline_RefineLaunchesAgentWithLaunchInDir(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	launchInDirCalled := false
	launchDir := "unexpected"
	agent := &refineLaunchInDirTestAgent{
		launchInDirFn: func(promptPath, dir string) error {
			launchInDirCalled = true
			launchDir = dir
			return nil
		},
	}

	p := New(&Deps{
		AgentResolver: &refineLaunchInDirTestResolver{agent: agent},
	}, &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
	})

	if _, err := p.Refine(context.Background(), RefineInput{IdeaText: "idea"}); err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}
	if !launchInDirCalled {
		t.Fatal("expected LaunchInDir to be called")
	}
	if launchDir != "" {
		t.Fatalf("launch dir = %q, want empty string", launchDir)
	}
}

func TestPipeline_RefineBlankSessionBacklogIdeaHasCreatedAt(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	agent := &refineLaunchInDirTestAgent{
		launchInDirFn: func(promptPath, dir string) error {
			specPath := filepath.Join(specsDir, "new-spec.md")
			return os.WriteFile(specPath, []byte("# New Spec\n"), 0o644)
		},
	}
	backlogMock := &refineCreatedAtBacklogMock{}

	p := New(&Deps{
		AgentResolver: &refineLaunchInDirTestResolver{agent: agent},
		BacklogClient: backlogMock,
	}, &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
	})

	if _, err := p.Refine(context.Background(), RefineInput{}); err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}
	if backlogMock.added == nil {
		t.Fatal("expected backlog Add to be called")
	}
	if backlogMock.added.CreatedAt.IsZero() {
		t.Fatal("CreatedAt is zero, want timestamp set")
	}
}
