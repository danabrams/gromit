package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type captureAddBacklogClient struct {
	added *Idea
}

func (m *captureAddBacklogClient) List() ([]*Idea, error) { return nil, nil }
func (m *captureAddBacklogClient) Get(id string) (*Idea, error) { return nil, nil }
func (m *captureAddBacklogClient) Add(item *Idea) error {
	m.added = item
	return nil
}
func (m *captureAddBacklogClient) Update(id string, fn func(*Idea)) error { return nil }

func TestRefine_BlankSessionAutoCreatedIdeaHasCreatedAt(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	backlogClient := &captureAddBacklogClient{}
	agent := &refineLaunchInDirTestAgent{
		launchInDirFn: func(promptPath, dir string) error {
			return os.WriteFile(filepath.Join(specsDir, "new-spec.md"), []byte("# New Spec\n"), 0o644)
		},
	}

	p := New(&Deps{
		AgentResolver: &refineLaunchInDirTestResolver{agent: agent},
		BacklogClient: backlogClient,
	}, &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
	})

	if _, err := p.Refine(context.Background(), RefineInput{}); err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}
	if backlogClient.added == nil {
		t.Fatal("expected backlog item to be auto-created")
	}
	if backlogClient.added.CreatedAt.IsZero() {
		t.Fatal("auto-created backlog idea missing CreatedAt timestamp")
	}
}
