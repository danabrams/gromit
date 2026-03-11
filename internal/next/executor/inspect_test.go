package executor

import (
	"context"
	"fmt"
	"testing"

	"github.com/danabrams/gromit/internal/next/validator"
)

type fakeGit struct {
	files []string
	err   error
}

func (f *fakeGit) DiffFiles(workDir string) ([]string, error) {
	return f.files, f.err
}

type fakeCheckRunner struct {
	targetedResult validator.CheckResults
	targetedErr    error
	alwaysResult   validator.CheckResults
	alwaysErr      error
}

func (f *fakeCheckRunner) RunTargeted(ctx context.Context, proofChecks []string, workDir string) (validator.CheckResults, error) {
	return f.targetedResult, f.targetedErr
}

func (f *fakeCheckRunner) RunAlwaysRun(ctx context.Context, checks []validator.Check, workDir string) (validator.CheckResults, error) {
	return f.alwaysResult, f.alwaysErr
}

func TestInspectChanges_ReturnsModifiedFiles(t *testing.T) {
	git := &fakeGit{files: []string{"pkg/parser/parser.go", "pkg/parser/parser_test.go"}}
	result, err := InspectChanges(context.Background(), InspectInput{
		GitClient: git,
		WorkDir:   t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.FilesChanged) != 2 {
		t.Fatalf("want 2 files, got %d", len(result.FilesChanged))
	}
}

func TestInspectChanges_EmptyDiff(t *testing.T) {
	git := &fakeGit{files: []string{}}
	result, err := InspectChanges(context.Background(), InspectInput{
		GitClient: git,
		WorkDir:   t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.FilesChanged) != 0 {
		t.Fatalf("want 0 files, got %d", len(result.FilesChanged))
	}
}

func TestInspectChanges_RunsTargetedAndAlwaysRunChecks(t *testing.T) {
	git := &fakeGit{files: []string{"a.go"}}
	runner := &fakeCheckRunner{
		targetedResult: validator.CheckResults{
			Results: []validator.CheckResult{
				{Name: "proof-1", Pass: true, Type: "proof"},
			},
		},
		alwaysResult: validator.CheckResults{
			Results: []validator.CheckResult{
				{Name: "vet", Pass: true, Type: "lint"},
				{Name: "fmt", Pass: false, Type: "lint"},
			},
		},
	}

	result, err := InspectChanges(context.Background(), InspectInput{
		GitClient:      git,
		WorkDir:        t.TempDir(),
		CheckRunner:    runner,
		TargetedChecks: []string{"go test ./..."},
		AlwaysRun:      []validator.Check{{Name: "vet", Command: "go vet", Type: "lint"}, {Name: "fmt", Command: "gofmt", Type: "lint"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.FilesChanged) != 1 {
		t.Fatalf("want 1 file, got %d", len(result.FilesChanged))
	}
	if len(result.TargetedResult.Results) != 1 {
		t.Fatalf("want 1 targeted result, got %d", len(result.TargetedResult.Results))
	}
	if !result.TargetedResult.AllPass() {
		t.Fatal("expected targeted checks to pass")
	}
	if len(result.AlwaysRunResult.Results) != 2 {
		t.Fatalf("want 2 always-run results, got %d", len(result.AlwaysRunResult.Results))
	}
	if result.AlwaysRunResult.AllPass() {
		t.Fatal("expected always-run to have failures")
	}
	if result.AlwaysRunResult.FailCount() != 1 {
		t.Fatalf("want 1 always-run failure, got %d", result.AlwaysRunResult.FailCount())
	}
}

func TestInspectChanges_NoCheckRunner_SkipsChecks(t *testing.T) {
	git := &fakeGit{files: []string{"b.go"}}
	result, err := InspectChanges(context.Background(), InspectInput{
		GitClient:      git,
		WorkDir:        t.TempDir(),
		TargetedChecks: []string{"go test ./..."},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.FilesChanged) != 1 {
		t.Fatalf("want 1 file, got %d", len(result.FilesChanged))
	}
	if len(result.TargetedResult.Results) != 0 {
		t.Fatal("expected no targeted results without runner")
	}
}

func TestInspectChanges_GitError(t *testing.T) {
	git := &fakeGit{err: fmt.Errorf("git broken")}
	_, err := InspectChanges(context.Background(), InspectInput{
		GitClient: git,
		WorkDir:   t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
