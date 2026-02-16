package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
)

func TestRunnerRun_MergeInteractiveBranchesStopsOnFailure(t *testing.T) {
	cfg := baseWorktreeMergeConfig()
	enabled := true
	autoMerge := true
	cfg.Worktree.Enabled = &enabled
	cfg.Worktree.AutoMerge = &autoMerge
	cfg.Worktree.MergeFailure = "stop"

	mockWorktrees := &mockWorktreeManager{
		PendingBranchesFn: func() ([]string, error) {
			return []string{"gromit/review-999"}, nil
		},
		MergeBackFn: func(branch string) error {
			return errors.New("merge conflict")
		},
	}

	r := setupRunnerForWorktreeMerge(t, cfg)
	r.worktreeManager = mockWorktrees

	err := r.Run(context.Background(), 1, time.Now().Add(time.Minute), nil, false)
	if err == nil {
		t.Fatal("expected merge failure to stop the loop, got nil error")
	}
}

func TestRunnerRun_SkipsMergeWhenAutoMergeDisabled(t *testing.T) {
	cfg := baseWorktreeMergeConfig()
	enabled := true
	autoMerge := false
	cfg.Worktree.Enabled = &enabled
	cfg.Worktree.AutoMerge = &autoMerge
	cfg.Worktree.MergeFailure = "warn"

	mergeCalls := 0
	mockWorktrees := &mockWorktreeManager{
		PendingBranchesFn: func() ([]string, error) {
			return []string{"gromit/review-001"}, nil
		},
		MergeBackFn: func(branch string) error {
			mergeCalls++
			return nil
		},
	}

	r := setupRunnerForWorktreeMerge(t, cfg)
	r.worktreeManager = mockWorktrees

	err := r.Run(context.Background(), 1, time.Now().Add(time.Minute), nil, false)
	if err != nil {
		t.Fatalf("unexpected error from Run: %v", err)
	}

	if mergeCalls != 0 {
		t.Fatalf("expected MergeBack not to be called when auto_merge is disabled, got %d calls", mergeCalls)
	}
}

func TestNewRunner_WiresWorktreeManagerWhenEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}

	enabled := true
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates: filepath.Join(tmpDir, ".gromit", "templates"),
			Logs:      logsDir,
		},
		Worktree: config.WorktreeConfig{
			Enabled: &enabled,
		},
	}

	r, err := NewRunner(cfg, &strings.Builder{})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	if r.worktreeManager == nil {
		t.Fatal("expected NewRunner to wire a worktree manager when worktree is enabled")
	}
}

func setupRunnerForWorktreeMerge(t *testing.T, cfg *config.Config) *Runner {
	t.Helper()

	tmpDir := t.TempDir()
	cfg.Paths.Logs = filepath.Join(tmpDir, "logs")
	if err := os.MkdirAll(cfg.Paths.Logs, 0755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}

	readyCalls := 0
	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			if readyCalls == 0 {
				readyCalls++
				return &bead.Bead{
					ID:              "bead-1",
					Title:           "Test merge-back",
					Priority:        1,
					Labels:          []string{},
					ExpectedOutputs: []string{},
				}, nil
			}
			return nil, nil
		},
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(
		cfg,
		&buf,
		tmpDir,
		Deps{
			Beads:    mockBeads,
			Router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
			Analyzer: &mockFailureAnalyzer{},
			Renderer: &mockPromptRenderer{},
			Logger:   &mockIterationLogger{},
		},
	)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps: %v", err)
	}

	r.gitDiffFn = func(startCommit string) (string, error) {
		return "", nil
	}

	return r
}

func baseWorktreeMergeConfig() *config.Config {
	cfg := &config.Config{}

	autoPush := false
	cfg.Git.AutoPush = &autoPush

	precheck := false
	cfg.Precheck.Enabled = &precheck

	return cfg
}
