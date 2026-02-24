package benchmark

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildModeOverlay_PinsProviderAndModelFamilyAcrossModes(t *testing.T) {
	manifest := HarnessManifest{
		Provider:        "openai",
		ModelFamily:     "gpt-5",
		LowTierModel:    "gpt-5-mini",
		MediumTierModel: "gpt-5.3-codex",
		HighTierModel:   "gpt-5.3-codex",
	}

	for _, mode := range []string{"single_pass", "tdd_shared_context", "tdd_fresh_context"} {
		overlay, err := BuildModeOverlay(manifest, mode)
		if err != nil {
			t.Fatalf("BuildModeOverlay(%q) error = %v", mode, err)
		}
		if overlay.Provider != manifest.Provider {
			t.Fatalf("mode %q provider = %q, want %q", mode, overlay.Provider, manifest.Provider)
		}
		if overlay.ModelFamily != manifest.ModelFamily {
			t.Fatalf("mode %q model_family = %q, want %q", mode, overlay.ModelFamily, manifest.ModelFamily)
		}
		if overlay.TierModels.Low != manifest.LowTierModel {
			t.Fatalf("mode %q low tier model = %q, want %q", mode, overlay.TierModels.Low, manifest.LowTierModel)
		}
		if overlay.TierModels.Medium != manifest.MediumTierModel {
			t.Fatalf("mode %q medium tier model = %q, want %q", mode, overlay.TierModels.Medium, manifest.MediumTierModel)
		}
		if overlay.TierModels.High != manifest.HighTierModel {
			t.Fatalf("mode %q high tier model = %q, want %q", mode, overlay.TierModels.High, manifest.HighTierModel)
		}
	}
}

func TestBuildModeOverlay_UsesLowTierDefaultsForBuildAndValidation(t *testing.T) {
	manifest := HarnessManifest{
		Provider:        "openai",
		ModelFamily:     "gpt-5",
		LowTierModel:    "gpt-5-mini",
		MediumTierModel: "gpt-5.3-codex",
		HighTierModel:   "gpt-5.3-codex",
	}

	for _, mode := range []string{"single_pass", "tdd_shared_context", "tdd_fresh_context"} {
		overlay, err := BuildModeOverlay(manifest, mode)
		if err != nil {
			t.Fatalf("BuildModeOverlay(%q) error = %v", mode, err)
		}
		if overlay.BuildTierDefault != "low" {
			t.Fatalf("mode %q build tier default = %q, want %q", mode, overlay.BuildTierDefault, "low")
		}
		if overlay.ValidationTierDefault != "low" {
			t.Fatalf("mode %q validation tier default = %q, want %q", mode, overlay.ValidationTierDefault, "low")
		}
	}
}

func TestBuildModeOverlay_ConfiguresFinalHighTierReviewWithApplyFixes(t *testing.T) {
	manifest := HarnessManifest{
		Provider:        "openai",
		ModelFamily:     "gpt-5",
		LowTierModel:    "gpt-5-mini",
		MediumTierModel: "gpt-5.3-codex",
		HighTierModel:   "gpt-5.3-codex",
	}

	overlay, err := BuildModeOverlay(manifest, "single_pass")
	if err != nil {
		t.Fatalf("BuildModeOverlay() error = %v", err)
	}
	if !overlay.FinalReview.Enabled {
		t.Fatal("final review enabled = false, want true")
	}
	if !overlay.FinalReview.NonInteractive {
		t.Fatal("final review non_interactive = false, want true")
	}
	if overlay.FinalReview.Tier != "high" {
		t.Fatalf("final review tier = %q, want %q", overlay.FinalReview.Tier, "high")
	}
	if !overlay.FinalReview.ApplyFixes {
		t.Fatal("final review apply_fixes = false, want true")
	}
}

func TestBuildModeOverlay_ModeDifferencesAreMethodologyOnly(t *testing.T) {
	manifest := HarnessManifest{
		Provider:        "openai",
		ModelFamily:     "gpt-5",
		LowTierModel:    "gpt-5-mini",
		MediumTierModel: "gpt-5.3-codex",
		HighTierModel:   "gpt-5.3-codex",
	}

	single, err := BuildModeOverlay(manifest, "single_pass")
	if err != nil {
		t.Fatalf("single_pass overlay error = %v", err)
	}
	shared, err := BuildModeOverlay(manifest, "tdd_shared_context")
	if err != nil {
		t.Fatalf("tdd_shared_context overlay error = %v", err)
	}
	fresh, err := BuildModeOverlay(manifest, "tdd_fresh_context")
	if err != nil {
		t.Fatalf("tdd_fresh_context overlay error = %v", err)
	}

	if single.BuildStrategy != "single_pass" {
		t.Fatalf("single_pass build strategy = %q, want %q", single.BuildStrategy, "single_pass")
	}
	if shared.BuildStrategy != "tdd" || fresh.BuildStrategy != "tdd" {
		t.Fatalf("tdd build strategy mismatch: shared=%q fresh=%q", shared.BuildStrategy, fresh.BuildStrategy)
	}
	if shared.FreshContextPerCycle {
		t.Fatal("tdd_shared_context fresh_context_per_cycle = true, want false")
	}
	if !fresh.FreshContextPerCycle {
		t.Fatal("tdd_fresh_context fresh_context_per_cycle = false, want true")
	}
}

func TestFinalizeModeRunResult_RecordsFinalValidationAfterReview(t *testing.T) {
	manifest := HarnessManifest{
		Provider:        "openai",
		ModelFamily:     "gpt-5",
		LowTierModel:    "gpt-5-mini",
		MediumTierModel: "gpt-5.3-codex",
		HighTierModel:   "gpt-5.3-codex",
	}
	overlay, err := BuildModeOverlay(manifest, "single_pass")
	if err != nil {
		t.Fatalf("BuildModeOverlay() error = %v", err)
	}

	result := FinalizeModeRunResult(overlay, false)
	if !result.FinalReviewRan {
		t.Fatal("final review ran = false, want true")
	}
	if !result.FinalValidationRan {
		t.Fatal("final validation ran = false, want true")
	}
	if !result.FinalValidationAfterReview {
		t.Fatal("final validation after review = false, want true")
	}
	if result.FinalValidationPassed {
		t.Fatal("final validation passed = true, want false")
	}
}

func TestRunModesInIsolatedWorktrees_UsesOneResolvedBaseCommitAndSameSelectedBeads(t *testing.T) {
	resolver := &stubBaseCommitResolver{resolved: "abc123"}
	runner := &recordingModeRunner{}
	manifest := HarnessManifest{
		Provider:        "openai",
		ModelFamily:     "gpt-5",
		LowTierModel:    "gpt-5-mini",
		MediumTierModel: "gpt-5.3-codex",
		HighTierModel:   "gpt-5.3-codex",
	}
	selected := []string{"gromit-1", "gromit-2", "gromit-3"}

	_, baseCommit, err := RunModesInIsolatedWorktrees(context.Background(), RunModesInput{
		Manifest:       manifest,
		SelectedBeads:  selected,
		BaseCommitHint: "HEAD",
		Resolver:       resolver,
		Runner:         runner,
	})
	if err != nil {
		t.Fatalf("RunModesInIsolatedWorktrees() error = %v", err)
	}
	if baseCommit != "abc123" {
		t.Fatalf("base commit = %q, want %q", baseCommit, "abc123")
	}

	if len(runner.requests) != 3 {
		t.Fatalf("mode run requests = %d, want 3", len(runner.requests))
	}
	for _, req := range runner.requests {
		if req.BaseCommit != "abc123" {
			t.Fatalf("mode %q base commit = %q, want %q", req.Mode, req.BaseCommit, "abc123")
		}
		if len(req.SelectedBeads) != len(selected) {
			t.Fatalf("mode %q selected bead count = %d, want %d", req.Mode, len(req.SelectedBeads), len(selected))
		}
		for i := range selected {
			if req.SelectedBeads[i] != selected[i] {
				t.Fatalf("mode %q selected beads[%d] = %q, want %q", req.Mode, i, req.SelectedBeads[i], selected[i])
			}
		}
	}
}

func TestRunModesInIsolatedWorktrees_CleansUpEveryModeAfterSuccessfulRun(t *testing.T) {
	resolver := &stubBaseCommitResolver{resolved: "abc123"}
	runner := &cleanupRecordingModeRunner{}

	_, _, err := RunModesInIsolatedWorktrees(context.Background(), RunModesInput{
		Manifest: HarnessManifest{
			Provider:        "openai",
			ModelFamily:     "gpt-5",
			LowTierModel:    "gpt-5-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
		},
		SelectedBeads:  []string{"gromit-1", "gromit-2", "gromit-3"},
		BaseCommitHint: "HEAD",
		Resolver:       resolver,
		Runner:         runner,
	})
	if err != nil {
		t.Fatalf("RunModesInIsolatedWorktrees() error = %v", err)
	}

	if len(runner.cleanupModes) != 3 {
		t.Fatalf("cleanup calls = %d, want 3", len(runner.cleanupModes))
	}
}

func TestRunModesInIsolatedWorktrees_CleansUpFailedModeBeforeReturningError(t *testing.T) {
	resolver := &stubBaseCommitResolver{resolved: "abc123"}
	runner := &cleanupOnFailureModeRunner{}

	_, _, err := RunModesInIsolatedWorktrees(context.Background(), RunModesInput{
		Manifest: HarnessManifest{
			Provider:        "openai",
			ModelFamily:     "gpt-5",
			LowTierModel:    "gpt-5-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
		},
		SelectedBeads:  []string{"gromit-1", "gromit-2", "gromit-3"},
		BaseCommitHint: "HEAD",
		Resolver:       resolver,
		Runner:         runner,
	})
	if err == nil {
		t.Fatal("RunModesInIsolatedWorktrees() error = nil, want failure")
	}
	if runner.failedModeCleanupCalls != 1 {
		t.Fatalf("failed mode cleanup calls = %d, want 1", runner.failedModeCleanupCalls)
	}
}

func TestRunModesInIsolatedWorktrees_PersistsModeLogsToDeterministicPathBeforeCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	logContent := "" +
		"{\"iteration\":1,\"actual_tier\":\"low\",\"input_tokens\":100,\"output_tokens\":20,\"cost_usd\":0.10}\n" +
		"{\"type\":\"review\",\"review_type\":\"light\",\"iteration\":1,\"fixes_applied\":1,\"beads_created\":1,\"backlog_created\":0}\n"

	resolver := &stubBaseCommitResolver{resolved: "abc123"}
	runner := &persistingLogModeRunner{
		baseDir:    tmpDir,
		logContent: logContent,
	}

	runs, _, err := RunModesInIsolatedWorktrees(context.Background(), RunModesInput{
		Manifest: HarnessManifest{
			Provider:        "openai",
			ModelFamily:     "gpt-5",
			LowTierModel:    "gpt-5-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
		},
		SelectedBeads:  []string{"gromit-1", "gromit-2", "gromit-3"},
		BaseCommitHint: "HEAD",
		Resolver:       resolver,
		Runner:         runner,
	})
	if err != nil {
		t.Fatalf("RunModesInIsolatedWorktrees() error = %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("run count = %d, want 3", len(runs))
	}

	for _, run := range runs {
		wantPath := filepath.Join(".gromit", "benchmarks", "logs", run.Mode+".jsonl")
		if run.LogPath != wantPath {
			t.Fatalf("mode %q log path = %q, want %q", run.Mode, run.LogPath, wantPath)
		}

		content, err := os.ReadFile(run.LogPath)
		if err != nil {
			t.Fatalf("read persisted mode log %q: %v", run.LogPath, err)
		}
		if string(content) != logContent {
			t.Fatalf("mode %q persisted log content mismatch\nwant:\n%s\ngot:\n%s", run.Mode, logContent, string(content))
		}
	}
}

type stubBaseCommitResolver struct {
	resolved string
	err      error
}

func (s *stubBaseCommitResolver) ResolveBaseCommit(_ context.Context, _ string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.resolved, nil
}

type recordingModeRunner struct {
	requests []ModeWorktreeRequest
}

func (r *recordingModeRunner) RunMode(_ context.Context, req ModeWorktreeRequest) (ModeWorktreeRun, error) {
	r.requests = append(r.requests, req)
	return ModeWorktreeRun{
		Mode: req.Mode,
		Cleanup: func() error {
			return nil
		},
	}, nil
}

var _ BaseCommitResolver = (*stubBaseCommitResolver)(nil)
var _ ModeWorktreeRunner = (*recordingModeRunner)(nil)

type cleanupRecordingModeRunner struct {
	cleanupModes []string
}

func (r *cleanupRecordingModeRunner) RunMode(_ context.Context, req ModeWorktreeRequest) (ModeWorktreeRun, error) {
	return ModeWorktreeRun{
		Mode: req.Mode,
		Cleanup: func() error {
			r.cleanupModes = append(r.cleanupModes, req.Mode)
			return nil
		},
	}, nil
}

type cleanupOnFailureModeRunner struct {
	callIndex               int
	failedModeCleanupCalls  int
}

type persistingLogModeRunner struct {
	baseDir    string
	logContent string
}

func (r *persistingLogModeRunner) RunMode(_ context.Context, req ModeWorktreeRequest) (ModeWorktreeRun, error) {
	sourceLogPath := filepath.Join(r.baseDir, req.Mode+"-session-run.jsonl")
	if err := os.WriteFile(sourceLogPath, []byte(r.logContent), 0o644); err != nil {
		return ModeWorktreeRun{}, err
	}
	return ModeWorktreeRun{
		Mode:    req.Mode,
		LogPath: sourceLogPath,
		Cleanup: func() error {
			return os.Remove(sourceLogPath)
		},
	}, nil
}

func (r *cleanupOnFailureModeRunner) RunMode(_ context.Context, req ModeWorktreeRequest) (ModeWorktreeRun, error) {
	r.callIndex++
	if r.callIndex == 2 {
		return ModeWorktreeRun{
			Mode: req.Mode,
			Cleanup: func() error {
				r.failedModeCleanupCalls++
				return nil
			},
		}, errors.New("mode execution failed")
	}
	return ModeWorktreeRun{
		Mode: req.Mode,
		Cleanup: func() error {
			return nil
		},
	}, nil
}
