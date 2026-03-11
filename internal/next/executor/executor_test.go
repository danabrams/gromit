package executor

import (
	"context"
	"testing"
)

type fakeAgent struct {
	output string
}

func (f *fakeAgent) Invoke(ctx context.Context, prompt string, tier string) (AgentResult, error) {
	return AgentResult{Output: f.output, TokensIn: 100, TokensOut: 50, Cost: 0.01, Model: "fake"}, nil
}

func TestExecutor_RunTask_PopulatesFilesChanged(t *testing.T) {
	agent := &fakeAgent{output: "done"}
	git := &fakeGit{files: []string{"a.go", "b.go"}}
	exec := NewExecutor(agent)

	result, err := exec.RunTask(context.Background(), RunTaskInput{
		Packet:    "do stuff",
		WorkDir:   t.TempDir(),
		ModelTier: "medium",
		GitClient: git,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.FilesChanged) != 2 {
		t.Fatalf("want 2 files changed, got %d", len(result.FilesChanged))
	}
	if result.FilesChanged[0] != "a.go" || result.FilesChanged[1] != "b.go" {
		t.Fatalf("unexpected files: %v", result.FilesChanged)
	}
}

func TestExecutor_RunTask_NilGitClient_LeavesFilesChangedEmpty(t *testing.T) {
	agent := &fakeAgent{output: "done"}
	exec := NewExecutor(agent)

	result, err := exec.RunTask(context.Background(), RunTaskInput{
		Packet:    "do stuff",
		WorkDir:   t.TempDir(),
		ModelTier: "medium",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.FilesChanged != nil {
		t.Fatalf("want nil FilesChanged when no GitClient, got %v", result.FilesChanged)
	}
}

func TestExecutor_RunTask_Success(t *testing.T) {
	agent := &fakeAgent{output: "Done. Changed files: parser.go"}
	exec := NewExecutor(agent)

	result, err := exec.RunTask(context.Background(), RunTaskInput{
		Packet:    "implement parser",
		WorkDir:   t.TempDir(),
		ModelTier: "medium",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.AgentOutput == "" {
		t.Fatal("expected agent output")
	}
	if result.TokensUsed != 150 {
		t.Fatalf("want 150 tokens, got %d", result.TokensUsed)
	}
	if result.Cost != 0.01 {
		t.Fatalf("want 0.01 cost, got %f", result.Cost)
	}
	if result.Model != "fake" {
		t.Fatalf("want model fake, got %s", result.Model)
	}
	if result.Tier != "medium" {
		t.Fatalf("want tier medium, got %s", result.Tier)
	}
}
