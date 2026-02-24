package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type refineCreatedAtTestAgent struct {
	launchInDirFn func(promptPath, dir string) error
}

func (a *refineCreatedAtTestAgent) Name() string { return "refine-created-at-test-agent" }
func (a *refineCreatedAtTestAgent) Launch(promptPath string) error {
	return a.LaunchInDir(promptPath, "")
}
func (a *refineCreatedAtTestAgent) LaunchInDir(promptPath, dir string) error {
	if a != nil && a.launchInDirFn != nil {
		return a.launchInDirFn(promptPath, dir)
	}
	return nil
}

type refineCreatedAtTestResolver struct{ agent Agent }

func (r *refineCreatedAtTestResolver) Resolve(phase string, flagOverride string, choosePicker bool) (Agent, error) {
	return r.agent, nil
}

type refineCreatedAtTestBacklog struct{ added *Idea }

func (b *refineCreatedAtTestBacklog) List() ([]*Idea, error)               { return nil, nil }
func (b *refineCreatedAtTestBacklog) Get(id string) (*Idea, error)         { return nil, nil }
func (b *refineCreatedAtTestBacklog) Update(id string, fn func(*Idea)) error { return nil }
func (b *refineCreatedAtTestBacklog) Add(item *Idea) error {
	if item == nil {
		b.added = nil
		return nil
	}
	copy := *item
	b.added = &copy
	return nil
}

func TestRefine_BlankSessionCreatedBacklogIdeaHasCreatedAt(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}

	agent := &refineCreatedAtTestAgent{
		launchInDirFn: func(promptPath, dir string) error {
			return os.WriteFile(filepath.Join(specsDir, "new-spec.md"), []byte("# New spec\n"), 0o644)
		},
	}
	backlog := &refineCreatedAtTestBacklog{}

	p := New(&Deps{
		AgentResolver: &refineCreatedAtTestResolver{agent: agent},
		BacklogClient: backlog,
	}, &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
	})

	if _, err := p.Refine(context.Background(), RefineInput{}); err != nil {
		t.Fatalf("Refine() error = %v", err)
	}
	if backlog.added == nil {
		t.Fatal("BacklogClient.Add() was not called")
	}
	if backlog.added.CreatedAt.IsZero() {
		t.Fatal("CreatedAt is zero, want non-zero timestamp")
	}
}
