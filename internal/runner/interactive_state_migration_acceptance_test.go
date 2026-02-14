//go:build acceptance

package runner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/state"
)

type mockWorktreeManager struct {
	EnsureWorktreeFn  func() (string, error)
	CreateBranchFn    func(command string) (string, error)
	MergeBackFn       func(branch string) error
	PendingBranchesFn func() ([]string, error)
	CleanupFn         func() error
}

func (m *mockWorktreeManager) EnsureWorktree() (string, error) {
	if m.EnsureWorktreeFn != nil {
		return m.EnsureWorktreeFn()
	}
	return "", nil
}

func (m *mockWorktreeManager) CreateBranch(command string) (string, error) {
	if m.CreateBranchFn != nil {
		return m.CreateBranchFn(command)
	}
	return "", nil
}

func (m *mockWorktreeManager) MergeBack(branch string) error {
	if m.MergeBackFn != nil {
		return m.MergeBackFn(branch)
	}
	return nil
}

func (m *mockWorktreeManager) PendingBranches() ([]string, error) {
	if m.PendingBranchesFn != nil {
		return m.PendingBranchesFn()
	}
	return []string{}, nil
}

func (m *mockWorktreeManager) Cleanup() error {
	if m.CleanupFn != nil {
		return m.CleanupFn()
	}
	return nil
}

func TestRunnerRun_ReviewBaselineUsesInteractiveState(t *testing.T) {
	// Expected failure: Runner.Run initializes review baseline using state.File,
	// not InteractiveFile, so interactive-state.json remains empty even when
	// state.json already has a LastReviewCommit.
	var _ WorktreeManager = (*mockWorktreeManager)(nil)

	gromitDir := setupInteractiveStateGromitDir(t)
	writeStateFile(t, gromitDir, state.State{LastReviewCommit: "already-set"})
	writeInteractiveStateFile(t, gromitDir, state.InteractiveState{})

	cfg := &config.Config{
		Paths: config.PathsConfig{
			Logs: filepath.Join(gromitDir, "logs"),
		},
	}
	if err := os.MkdirAll(cfg.Paths.Logs, 0755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(
		cfg,
		&buf,
		gromitDir,
		Deps{
			Beads:    &mockBeadClient{},
			Router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
			Analyzer: &mockFailureAnalyzer{},
			Renderer: &mockPromptRenderer{},
			Logger:   &mockIterationLogger{},
		},
	)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps: %v", err)
	}

	if err := r.Run(context.Background(), 1, time.Now().Add(-time.Minute), false); err != nil {
		t.Fatalf("Run: %v", err)
	}

	interactiveFile, err := state.NewInteractiveFile(gromitDir)
	if err != nil {
		t.Fatalf("NewInteractiveFile: %v", err)
	}
	if err := interactiveFile.Load(); err != nil {
		t.Fatalf("Load interactive file: %v", err)
	}

	if interactiveFile.LastReviewCommit() == "" || interactiveFile.LastReviewCommit() == "already-set" {
		t.Errorf("expected interactive-state.json LastReviewCommit to be initialized from git HEAD, got %q", interactiveFile.LastReviewCommit())
	}
}

func TestRunnerRun_RetroSuggestionUsesInteractiveState(t *testing.T) {
	// Expected failure: Runner.checkRetroSuggestion reads LastRetro from state.json,
	// so an old LastRetro there triggers a suggestion even when interactive-state.json
	// has a recent LastRetro.
	var _ WorktreeManager = (*mockWorktreeManager)(nil)

	gromitDir := setupInteractiveStateGromitDir(t)
	writeStateFile(t, gromitDir, state.State{LastRetro: time.Now().Add(-8 * 24 * time.Hour)})
	writeInteractiveStateFile(t, gromitDir, state.InteractiveState{LastRetro: time.Now().Add(-2 * 24 * time.Hour)})

	cfg := &config.Config{
		Paths: config.PathsConfig{
			Logs: filepath.Join(gromitDir, "logs"),
		},
	}
	if err := os.MkdirAll(cfg.Paths.Logs, 0755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(
		cfg,
		&buf,
		gromitDir,
		Deps{
			Beads:    &mockBeadClient{},
			Router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
			Analyzer: &mockFailureAnalyzer{},
			Renderer: &mockPromptRenderer{},
			Logger:   &mockIterationLogger{},
		},
	)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps: %v", err)
	}

	if err := r.Run(context.Background(), 1, time.Now().Add(-time.Minute), false); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if strings.Contains(buf.String(), "Retro suggested:") {
		t.Fatalf("expected no retro suggestion when interactive-state.json has recent LastRetro, got output:\n%s", buf.String())
	}
}

func setupInteractiveStateGromitDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("mkdir gromit dir: %v", err)
	}
	return gromitDir
}

func writeStateFile(t *testing.T, gromitDir string, st state.State) {
	t.Helper()
	path := filepath.Join(gromitDir, "state.json")
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write state file: %v", err)
	}
}

func writeInteractiveStateFile(t *testing.T, gromitDir string, st state.InteractiveState) {
	t.Helper()
	path := filepath.Join(gromitDir, "interactive-state.json")
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		t.Fatalf("marshal interactive state: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write interactive state file: %v", err)
	}
}
