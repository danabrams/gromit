package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type refineCreatedAtTestBacklogClient struct {
	added *Idea
}

func (m *refineCreatedAtTestBacklogClient) List() ([]*Idea, error) { return nil, nil }
func (m *refineCreatedAtTestBacklogClient) Get(id string) (*Idea, error) { return nil, nil }
func (m *refineCreatedAtTestBacklogClient) Add(item *Idea) error {
	m.added = item
	return nil
}
func (m *refineCreatedAtTestBacklogClient) Update(id string, fn func(*Idea)) error { return nil }

func TestRefine_BlankSessionAutoCreatedIdeaSetsCreatedAt(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	agent := &refineLaunchInDirTestAgent{
		launchInDirFn: func(promptPath, dir string) error {
			return os.WriteFile(filepath.Join(specsDir, "cache-layer.md"), []byte("# Cache Layer\n"), 0o644)
		},
	}
	backlogClient := &refineCreatedAtTestBacklogClient{}

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
		t.Fatal("expected backlog Add to be called")
	}
	if backlogClient.added.CreatedAt.IsZero() {
		t.Fatal("auto-created backlog idea CreatedAt is zero, want non-zero timestamp")
	}
}
