package benchmark

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	stdstrings "strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/worktree"
)

func TestGitBaseCommitResolver_ResolvesHintWithRevParse(t *testing.T) {
	t.Parallel()

	calls := make([][]string, 0, 1)
	resolver := NewGitBaseCommitResolver(func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		return "abc123\n", nil
	})

	got, err := resolver.ResolveBaseCommit(context.Background(), "feature-branch")
	if err != nil {
		t.Fatalf("ResolveBaseCommit() error = %v", err)
	}
	if got != "abc123" {
		t.Fatalf("ResolveBaseCommit() = %q, want %q", got, "abc123")
	}
	if len(calls) != 1 {
		t.Fatalf("git call count = %d, want 1", len(calls))
	}
	if len(calls[0]) != 3 || calls[0][0] != "rev-parse" || calls[0][1] != "--verify" || calls[0][2] != "feature-branch" {
		t.Fatalf("git args = %v, want [rev-parse --verify feature-branch]", calls[0])
	}
}

func TestGitBaseCommitResolver_IncludesGitOutputOnResolveFailure(t *testing.T) {
	t.Parallel()

	resolver := NewGitBaseCommitResolver(func(_ context.Context, _ ...string) (string, error) {
		return "fatal: Needed a single revision\n", errors.New("exit status 128")
	})

	_, err := resolver.ResolveBaseCommit(context.Background(), "abc123")
	if err == nil {
		t.Fatal("ResolveBaseCommit() error = nil, want non-nil")
	}
	if !stdstrings.Contains(err.Error(), `resolve base commit "abc123"`) {
		t.Fatalf("error = %q, want base commit context", err)
	}
	if !stdstrings.Contains(err.Error(), "fatal: Needed a single revision") {
		t.Fatalf("error = %q, want git stderr output", err)
	}
}

func TestSessionModeWorktreeRunner_RunModeExecutesInSessionAndReturnsCleanup(t *testing.T) {
	t.Parallel()

	session := &worktree.SessionWorktree{
		BranchName:  "gromit/benchmark-single_pass-1",
		WorktreeDir: "/tmp/repo-wt",
	}
	var executedReq ModeWorktreeRequest
	executedDir := ""
	cleaned := false

	runner := NewSessionModeWorktreeRunner(SessionModeWorktreeRunnerOptions{
		MainDir: "/tmp/repo",
		CreateSessionWorktree: func(command string) (*worktree.SessionWorktree, error) {
			if command != "benchmark-single_pass" {
				t.Fatalf("command = %q, want %q", command, "benchmark-single_pass")
			}
			return session, nil
		},
		CheckoutBaseCommitInWorktree: func(_ context.Context, _, _ string) error { return nil },
		RunModeInWorktree: func(_ context.Context, worktreeDir string, req ModeWorktreeRequest) error {
			executedDir = worktreeDir
			executedReq = req
			return nil
		},
		CleanupSession: func(mainDir, sessionDir string) error {
			if mainDir != "/tmp/repo" {
				t.Fatalf("cleanup mainDir = %q, want %q", mainDir, "/tmp/repo")
			}
			if sessionDir != session.WorktreeDir {
				t.Fatalf("cleanup sessionDir = %q, want %q", sessionDir, session.WorktreeDir)
			}
			cleaned = true
			return nil
		},
	})

	req := ModeWorktreeRequest{
		Mode:          "single_pass",
		BaseCommit:    "abc123",
		SelectedBeads: []string{"gromit-1", "gromit-2"},
	}
	run, err := runner.RunMode(context.Background(), req)
	if err != nil {
		t.Fatalf("RunMode() error = %v", err)
	}
	if executedDir != session.WorktreeDir {
		t.Fatalf("executed worktreeDir = %q, want %q", executedDir, session.WorktreeDir)
	}
	if executedReq.Mode != req.Mode || executedReq.BaseCommit != req.BaseCommit {
		t.Fatalf("executed req = %+v, want mode/base commit from request", executedReq)
	}
	if len(executedReq.SelectedBeads) != 2 || executedReq.SelectedBeads[0] != "gromit-1" || executedReq.SelectedBeads[1] != "gromit-2" {
		t.Fatalf("executed selected beads = %v, want [gromit-1 gromit-2]", executedReq.SelectedBeads)
	}
	if cleaned {
		t.Fatal("cleanup called before cleanup callback execution")
	}
	if run.Cleanup == nil {
		t.Fatal("Cleanup callback = nil, want non-nil")
	}
	if err := run.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if !cleaned {
		t.Fatal("cleanup callback was not invoked")
	}
}

func TestSessionModeWorktreeRunner_RunModeChecksOutBaseCommitBeforeExecution(t *testing.T) {
	t.Parallel()

	session := &worktree.SessionWorktree{
		BranchName:  "gromit/benchmark-single_pass-2",
		WorktreeDir: "/tmp/repo-wt",
	}
	sequence := make([]string, 0, 2)

	runner := NewSessionModeWorktreeRunner(SessionModeWorktreeRunnerOptions{
		MainDir: "/tmp/repo",
		CreateSessionWorktree: func(command string) (*worktree.SessionWorktree, error) {
			return session, nil
		},
		CheckoutBaseCommitInWorktree: func(_ context.Context, worktreeDir, baseCommit string) error {
			if worktreeDir != session.WorktreeDir {
				t.Fatalf("checkout worktreeDir = %q, want %q", worktreeDir, session.WorktreeDir)
			}
			if baseCommit != "abc123" {
				t.Fatalf("checkout baseCommit = %q, want %q", baseCommit, "abc123")
			}
			sequence = append(sequence, "checkout")
			return nil
		},
		RunModeInWorktree: func(_ context.Context, _ string, _ ModeWorktreeRequest) error {
			sequence = append(sequence, "run")
			return nil
		},
		CleanupSession: func(_, _ string) error { return nil },
	})

	_, err := runner.RunMode(context.Background(), ModeWorktreeRequest{
		Mode:       "single_pass",
		BaseCommit: "abc123",
	})
	if err != nil {
		t.Fatalf("RunMode() error = %v", err)
	}
	if len(sequence) != 2 || sequence[0] != "checkout" || sequence[1] != "run" {
		t.Fatalf("execution sequence = %v, want [checkout run]", sequence)
	}
}

func TestDefaultSessionCleanup_RetriesAfterPermissionDenied(t *testing.T) {
	origRemove := sessionCleanupRemoveFn
	origNormalize := sessionCleanupNormalizePermissionsFn
	t.Cleanup(func() {
		sessionCleanupRemoveFn = origRemove
		sessionCleanupNormalizePermissionsFn = origNormalize
	})

	removeCalls := 0
	normalized := false
	sessionCleanupRemoveFn = func(_, _ string) error {
		removeCalls++
		if removeCalls == 1 {
			return fmt.Errorf("remove session worktree %q: %w: %s", "/tmp/wt", errors.New("exit status 255"), "failed to delete '/tmp/wt': Permission denied")
		}
		return nil
	}
	sessionCleanupNormalizePermissionsFn = func(sessionDir string) error {
		if sessionDir != "/tmp/wt" {
			t.Fatalf("sessionDir = %q, want %q", sessionDir, "/tmp/wt")
		}
		normalized = true
		return nil
	}

	if err := defaultSessionCleanup("/tmp/repo", "/tmp/wt"); err != nil {
		t.Fatalf("defaultSessionCleanup() error = %v", err)
	}
	if !normalized {
		t.Fatal("expected permission normalization on permission denied failure")
	}
	if removeCalls != 2 {
		t.Fatalf("remove calls = %d, want 2", removeCalls)
	}
}

func TestDefaultSessionCleanup_DoesNotRetryOnNonPermissionFailure(t *testing.T) {
	origRemove := sessionCleanupRemoveFn
	origNormalize := sessionCleanupNormalizePermissionsFn
	t.Cleanup(func() {
		sessionCleanupRemoveFn = origRemove
		sessionCleanupNormalizePermissionsFn = origNormalize
	})

	removeCalls := 0
	sessionCleanupRemoveFn = func(_, _ string) error {
		removeCalls++
		return fmt.Errorf("remove session worktree %q: %w: %s", "/tmp/wt", errors.New("exit status 255"), "invalid argument")
	}
	sessionCleanupNormalizePermissionsFn = func(string) error {
		t.Fatal("normalize should not run for non-permission failures")
		return nil
	}

	err := defaultSessionCleanup("/tmp/repo", "/tmp/wt")
	if err == nil {
		t.Fatal("defaultSessionCleanup() error = nil, want non-nil")
	}
	if removeCalls != 1 {
		t.Fatalf("remove calls = %d, want 1", removeCalls)
	}
}

func TestDefaultSessionCleanup_ReturnsCombinedErrorWhenNormalizationFails(t *testing.T) {
	origRemove := sessionCleanupRemoveFn
	origNormalize := sessionCleanupNormalizePermissionsFn
	t.Cleanup(func() {
		sessionCleanupRemoveFn = origRemove
		sessionCleanupNormalizePermissionsFn = origNormalize
	})

	sessionCleanupRemoveFn = func(_, _ string) error {
		return fmt.Errorf("remove session worktree %q: %w: %s", "/tmp/wt", errors.New("exit status 255"), "Permission denied")
	}
	sessionCleanupNormalizePermissionsFn = func(string) error {
		return errors.New("chmod failed")
	}

	err := defaultSessionCleanup("/tmp/repo", "/tmp/wt")
	if err == nil {
		t.Fatal("defaultSessionCleanup() error = nil, want non-nil")
	}
	if !stdstrings.Contains(err.Error(), "normalize permissions for session worktree") {
		t.Fatalf("error = %q, want normalization context", err)
	}
	if !stdstrings.Contains(err.Error(), "chmod failed") {
		t.Fatalf("error = %q, want chmod failure context", err)
	}
}

func TestDefaultSessionCleanup_RemovesOrphanedDirectoryWhenWorktreeMetadataDropped(t *testing.T) {
	origRemove := sessionCleanupRemoveFn
	origNormalize := sessionCleanupNormalizePermissionsFn
	origRemoveAll := sessionCleanupRemoveAllFn
	t.Cleanup(func() {
		sessionCleanupRemoveFn = origRemove
		sessionCleanupNormalizePermissionsFn = origNormalize
		sessionCleanupRemoveAllFn = origRemoveAll
	})

	removeCalls := 0
	removeAllCalls := 0
	sessionCleanupRemoveFn = func(_, _ string) error {
		removeCalls++
		if removeCalls == 1 {
			return fmt.Errorf("remove session worktree %q: %w: %s", "/tmp/wt", errors.New("exit status 255"), "failed to delete '/tmp/wt': Permission denied")
		}
		return fmt.Errorf("remove session worktree %q: %w: %s", "/tmp/wt", errors.New("exit status 128"), "fatal: '/tmp/wt' is not a working tree")
	}
	sessionCleanupNormalizePermissionsFn = func(string) error { return nil }
	sessionCleanupRemoveAllFn = func(path string) error {
		removeAllCalls++
		if path != "/tmp/wt" {
			t.Fatalf("removeAll path = %q, want %q", path, "/tmp/wt")
		}
		return nil
	}

	if err := defaultSessionCleanup("/tmp/repo", "/tmp/wt"); err != nil {
		t.Fatalf("defaultSessionCleanup() error = %v", err)
	}
	if removeCalls != 2 {
		t.Fatalf("remove calls = %d, want 2", removeCalls)
	}
	if removeAllCalls != 1 {
		t.Fatalf("removeAll calls = %d, want 1", removeAllCalls)
	}
}

func TestDefaultSessionCleanup_ReturnsErrorWhenOrphanedDirectoryRemovalFails(t *testing.T) {
	origRemove := sessionCleanupRemoveFn
	origNormalize := sessionCleanupNormalizePermissionsFn
	origRemoveAll := sessionCleanupRemoveAllFn
	t.Cleanup(func() {
		sessionCleanupRemoveFn = origRemove
		sessionCleanupNormalizePermissionsFn = origNormalize
		sessionCleanupRemoveAllFn = origRemoveAll
	})

	removeCalls := 0
	sessionCleanupRemoveFn = func(_, _ string) error {
		removeCalls++
		if removeCalls == 1 {
			return fmt.Errorf("remove session worktree %q: %w: %s", "/tmp/wt", errors.New("exit status 255"), "failed to delete '/tmp/wt': Permission denied")
		}
		return fmt.Errorf("remove session worktree %q: %w: %s", "/tmp/wt", errors.New("exit status 128"), "fatal: '/tmp/wt' is not a working tree")
	}
	sessionCleanupNormalizePermissionsFn = func(string) error { return nil }
	sessionCleanupRemoveAllFn = func(string) error { return errors.New("removeall failed") }

	err := defaultSessionCleanup("/tmp/repo", "/tmp/wt")
	if err == nil {
		t.Fatal("defaultSessionCleanup() error = nil, want non-nil")
	}
	if !stdstrings.Contains(err.Error(), "remove orphaned session worktree directory") {
		t.Fatalf("error = %q, want orphaned-directory context", err)
	}
	if !stdstrings.Contains(err.Error(), "removeall failed") {
		t.Fatalf("error = %q, want removeall failure context", err)
	}
}

func TestEnsureRemovablePermissions_GrantsOwnerPermissionsRecursively(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	subDir := filepath.Join(root, "nested")
	if err := os.MkdirAll(subDir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	filePath := filepath.Join(subDir, "data.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Chmod(filePath, 0o000); err != nil {
		t.Fatalf("Chmod(filePath) error = %v", err)
	}
	if err := os.Chmod(subDir, 0o000); err != nil {
		t.Fatalf("Chmod(subDir) error = %v", err)
	}

	if err := ensureRemovablePermissions(root); err != nil {
		t.Fatalf("ensureRemovablePermissions() error = %v", err)
	}

	subInfo, err := os.Stat(subDir)
	if err != nil {
		t.Fatalf("Stat(subDir) error = %v", err)
	}
	if subInfo.Mode().Perm()&0o700 != 0o700 {
		t.Fatalf("subDir perms = %o, want owner rwx set", subInfo.Mode().Perm())
	}
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat(filePath) error = %v", err)
	}
	if fileInfo.Mode().Perm()&0o600 != 0o600 {
		t.Fatalf("file perms = %o, want owner rw set", fileInfo.Mode().Perm())
	}
}

func TestApplyBenchmarkOverlayToConfig_PinsProviderAndEnforcesTierPolicies(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"openai": {
				Models: map[string]string{
					"low":    "placeholder-low",
					"medium": "placeholder-medium",
					"high":   "placeholder-high",
				},
			},
			"other": {
				Models: map[string]string{
					"low":    "other-low",
					"medium": "other-medium",
					"high":   "other-high",
				},
			},
		},
	}
	overlay, err := BuildModeOverlay(HarnessManifest{
		Provider:        "openai",
		ModelFamily:     "gpt-5",
		LowTierModel:    "gpt-5-mini",
		MediumTierModel: "gpt-5.3-codex",
		HighTierModel:   "gpt-5.3-codex",
	}, "tdd_fresh_context")
	if err != nil {
		t.Fatalf("BuildModeOverlay() error = %v", err)
	}

	got, err := applyBenchmarkOverlayToConfig(cfg, overlay)
	if err != nil {
		t.Fatalf("applyBenchmarkOverlayToConfig() error = %v", err)
	}

	if got.Methodology.BuildStrategy != "tdd" {
		t.Fatalf("build_strategy = %q, want %q", got.Methodology.BuildStrategy, "tdd")
	}
	if !got.Methodology.FreshContextPerCycle {
		t.Fatal("fresh_context_per_cycle = false, want true")
	}
	if got.Methodology.PhaseModels.Build != "low" {
		t.Fatalf("phase_models.build = %q, want %q", got.Methodology.PhaseModels.Build, "low")
	}
	if got.Review.Tier != "high" {
		t.Fatalf("review.tier = %q, want %q", got.Review.Tier, "high")
	}
	if got.Review.Thorough.Tier != "high" {
		t.Fatalf("review.thorough.tier = %q, want %q", got.Review.Thorough.Tier, "high")
	}
	if !got.Validation.IsNonInteractive() {
		t.Fatal("validation.non_interactive = false, want true")
	}
	if got.Providers["openai"].Models["low"] != "gpt-5-mini" {
		t.Fatalf("provider low model = %q, want %q", got.Providers["openai"].Models["low"], "gpt-5-mini")
	}
	if got.Providers["openai"].Models["medium"] != "gpt-5.3-codex" {
		t.Fatalf("provider medium model = %q, want %q", got.Providers["openai"].Models["medium"], "gpt-5.3-codex")
	}
	if got.Providers["openai"].Models["high"] != "gpt-5.3-codex" {
		t.Fatalf("provider high model = %q, want %q", got.Providers["openai"].Models["high"], "gpt-5.3-codex")
	}
	if _, exists := got.Providers["other"]; exists {
		t.Fatal("unexpected non-pinned provider retained in overlay config")
	}

	// Verify the original config's models were NOT mutated by the overlay.
	origModels := cfg.Providers["openai"].Models
	if origModels["low"] != "placeholder-low" {
		t.Fatalf("original config openai low model = %q, want %q (shared map mutation)", origModels["low"], "placeholder-low")
	}
	if origModels["medium"] != "placeholder-medium" {
		t.Fatalf("original config openai medium model = %q, want %q (shared map mutation)", origModels["medium"], "placeholder-medium")
	}
	if origModels["high"] != "placeholder-high" {
		t.Fatalf("original config openai high model = %q, want %q (shared map mutation)", origModels["high"], "placeholder-high")
	}
}

func TestApplyBenchmarkOverlay_ManifestModelsReachProviderConstructor(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"openai": {
				Binary: "codex",
				Flags:  []string{"--dangerously-bypass-approvals-and-sandbox"},
				Models: map[string]string{
					"high":   "gpt-5.3-codex",
					"medium": "gpt-5.3-codex",
					"low":    "gpt-5-mini",
				},
			},
		},
	}

	overlay, err := BuildModeOverlay(HarnessManifest{
		Provider:        "openai",
		ModelFamily:     "gpt-5",
		LowTierModel:    "manifest-low",
		MediumTierModel: "manifest-medium",
		HighTierModel:   "manifest-high",
	}, "single_pass")
	if err != nil {
		t.Fatalf("BuildModeOverlay() error = %v", err)
	}

	got, err := applyBenchmarkOverlayToConfig(cfg, overlay)
	if err != nil {
		t.Fatalf("applyBenchmarkOverlayToConfig() error = %v", err)
	}

	// Simulate what buildRouterAndLearningsProvider does before constructing providers.
	got.SetDefaults()
	got.NormalizeNilFields()

	def := got.Providers["openai"]
	if def.Models["low"] != "manifest-low" {
		t.Fatalf("after defaults, provider low model = %q, want %q", def.Models["low"], "manifest-low")
	}
	if def.Models["medium"] != "manifest-medium" {
		t.Fatalf("after defaults, provider medium model = %q, want %q", def.Models["medium"], "manifest-medium")
	}
	if def.Models["high"] != "manifest-high" {
		t.Fatalf("after defaults, provider high model = %q, want %q", def.Models["high"], "manifest-high")
	}
	// Verify the existing provider fields survived the overlay.
	if def.Binary != "codex" {
		t.Fatalf("provider binary = %q, want %q", def.Binary, "codex")
	}
}
