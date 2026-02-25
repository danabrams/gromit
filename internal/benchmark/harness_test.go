package benchmark

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	origEnsureOpen := ensureBenchmarkBeadsOpenFn
	ensureBenchmarkBeadsOpenFn = func(context.Context, []string) error { return nil }
	code := m.Run()
	ensureBenchmarkBeadsOpenFn = origEnsureOpen
	os.Exit(code)
}

func TestBuildModeOverlay_PinsProviderAndModelFamilyAcrossModes(t *testing.T) {
	manifest := HarnessManifest{
		Provider:        "openai",
		ModelFamily:     "gpt-5",
		LowTierModel:    "gpt-5.1-codex-mini",
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

func TestBuildModeOverlay_UsesExpectedTierDefaultsForBuildAndValidation(t *testing.T) {
	manifest := HarnessManifest{
		Provider:        "openai",
		ModelFamily:     "gpt-5",
		LowTierModel:    "gpt-5.1-codex-mini",
		MediumTierModel: "gpt-5.3-codex",
		HighTierModel:   "gpt-5.3-codex",
	}

	cases := map[string]string{
		"single_pass":       "low",
		"tdd_shared_context": "low",
		"tdd_fresh_context":  "low",
	}
	for mode, expectedTier := range cases {
		overlay, err := BuildModeOverlay(manifest, mode)
		if err != nil {
			t.Fatalf("BuildModeOverlay(%q) error = %v", mode, err)
		}
		if overlay.BuildTierDefault != expectedTier {
			t.Fatalf("mode %q build tier default = %q, want %q", mode, overlay.BuildTierDefault, expectedTier)
		}
		if overlay.ValidationTierDefault != expectedTier {
			t.Fatalf("mode %q validation tier default = %q, want %q", mode, overlay.ValidationTierDefault, expectedTier)
		}
	}
}

func TestBuildModeOverlay_ConfiguresFinalHighTierReviewWithApplyFixes(t *testing.T) {
	manifest := HarnessManifest{
		Provider:        "openai",
		ModelFamily:     "gpt-5",
		LowTierModel:    "gpt-5.1-codex-mini",
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
		LowTierModel:    "gpt-5.1-codex-mini",
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
		LowTierModel:    "gpt-5.1-codex-mini",
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
		LowTierModel:    "gpt-5.1-codex-mini",
		MediumTierModel: "gpt-5.3-codex",
		HighTierModel:   "gpt-5.3-codex",
		Modes:           []string{"single_pass", "tdd_shared_context", "tdd_fresh_context"},
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
			LowTierModel:    "gpt-5.1-codex-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
			Modes:           []string{"single_pass", "tdd_shared_context", "tdd_fresh_context"},
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
			LowTierModel:    "gpt-5.1-codex-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
			Modes:           []string{"single_pass", "tdd_shared_context", "tdd_fresh_context"},
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
			LowTierModel:    "gpt-5.1-codex-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
			Modes:           []string{"single_pass", "tdd_shared_context", "tdd_fresh_context"},
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

func TestRunModesInIsolatedWorktrees_ExecutesOnlyRequestedManifestModes(t *testing.T) {
	resolver := &stubBaseCommitResolver{resolved: "abc123"}
	runner := &recordingModeRunner{}

	_, _, err := RunModesInIsolatedWorktrees(context.Background(), RunModesInput{
		Manifest: HarnessManifest{
			Provider:        "openai",
			ModelFamily:     "gpt-5",
			LowTierModel:    "gpt-5.1-codex-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
			Modes:           []string{"single_pass"},
		},
		SelectedBeads:  []string{"gromit-1", "gromit-2", "gromit-3"},
		BaseCommitHint: "HEAD",
		Resolver:       resolver,
		Runner:         runner,
	})
	if err != nil {
		t.Fatalf("RunModesInIsolatedWorktrees() error = %v", err)
	}
	if len(runner.requests) != 1 {
		t.Fatalf("mode run requests = %d, want 1", len(runner.requests))
	}
	if runner.requests[0].Mode != "single_pass" {
		t.Fatalf("executed mode = %q, want %q", runner.requests[0].Mode, "single_pass")
	}
}

func TestRunModesInIsolatedWorktrees_ReopensSelectedBeadsBeforeEachMode(t *testing.T) {
	origEnsureOpen := ensureBenchmarkBeadsOpenFn
	t.Cleanup(func() { ensureBenchmarkBeadsOpenFn = origEnsureOpen })

	resolver := &stubBaseCommitResolver{resolved: "abc123"}
	runner := &recordingModeRunner{}
	calls := 0
	ensureBenchmarkBeadsOpenFn = func(_ context.Context, ids []string) error {
		calls++
		if len(ids) != 2 || ids[0] != "gromit-1" || ids[1] != "gromit-2" {
			t.Fatalf("ensure open bead IDs = %v, want [gromit-1 gromit-2]", ids)
		}
		return nil
	}

	_, _, err := RunModesInIsolatedWorktrees(context.Background(), RunModesInput{
		Manifest: HarnessManifest{
			Provider:        "openai",
			ModelFamily:     "gpt-5",
			LowTierModel:    "gpt-5.1-codex-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
			Modes:           []string{"single_pass", "tdd_shared_context", "tdd_fresh_context"},
		},
		SelectedBeads: []string{"gromit-1", "gromit-2"},
		Resolver:      resolver,
		Runner:        runner,
	})
	if err != nil {
		t.Fatalf("RunModesInIsolatedWorktrees() error = %v", err)
	}
	if calls != 3 {
		t.Fatalf("ensure open calls = %d, want 3 (once per mode)", calls)
	}
}

func TestRunModesInIsolatedWorktrees_ReturnsOpenBeadErrorBeforeModeRun(t *testing.T) {
	origEnsureOpen := ensureBenchmarkBeadsOpenFn
	t.Cleanup(func() { ensureBenchmarkBeadsOpenFn = origEnsureOpen })

	resolver := &stubBaseCommitResolver{resolved: "abc123"}
	runner := &recordingModeRunner{}
	ensureBenchmarkBeadsOpenFn = func(_ context.Context, _ []string) error {
		return errors.New("bd update failed")
	}

	_, _, err := RunModesInIsolatedWorktrees(context.Background(), RunModesInput{
		Manifest: HarnessManifest{
			Provider:        "openai",
			ModelFamily:     "gpt-5",
			LowTierModel:    "gpt-5.1-codex-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
			Modes:           []string{"single_pass"},
		},
		SelectedBeads: []string{"gromit-1"},
		Resolver:      resolver,
		Runner:        runner,
	})
	if err == nil {
		t.Fatal("RunModesInIsolatedWorktrees() error = nil, want non-nil")
	}
	if len(runner.requests) != 0 {
		t.Fatalf("runner requests = %d, want 0 when reopening beads fails", len(runner.requests))
	}
}

func TestRunModesInIsolatedWorktrees_RejectsEmptyModes(t *testing.T) {
	resolver := &stubBaseCommitResolver{resolved: "abc123"}
	runner := &recordingModeRunner{}

	_, _, err := RunModesInIsolatedWorktrees(context.Background(), RunModesInput{
		Manifest: HarnessManifest{
			Provider:        "openai",
			ModelFamily:     "gpt-5",
			LowTierModel:    "gpt-5.1-codex-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
		},
		Modes:          nil,
		SelectedBeads:  []string{"gromit-1", "gromit-2", "gromit-3"},
		BaseCommitHint: "HEAD",
		Resolver:       resolver,
		Runner:         runner,
	})
	if err == nil {
		t.Fatal("RunModesInIsolatedWorktrees() error = nil, want non-nil")
	}
}

func TestRunModesInIsolatedWorktrees_ExecutesOnlyManifestDeclaredModes(t *testing.T) {
	resolver := &stubBaseCommitResolver{resolved: "abc123"}
	runner := &recordingModeRunner{}

	_, _, err := RunModesInIsolatedWorktrees(context.Background(), RunModesInput{
		Manifest: HarnessManifest{
			Provider:        "openai",
			ModelFamily:     "gpt-5",
			LowTierModel:    "gpt-5.1-codex-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
			Modes:           []string{"single_pass", "tdd_fresh_context"},
		},
		SelectedBeads:  []string{"gromit-1", "gromit-2", "gromit-3"},
		BaseCommitHint: "HEAD",
		Resolver:       resolver,
		Runner:         runner,
	})
	if err != nil {
		t.Fatalf("RunModesInIsolatedWorktrees() error = %v", err)
	}
	if len(runner.requests) != 2 {
		t.Fatalf("mode run requests = %d, want 2 (only manifest declared modes)", len(runner.requests))
	}
	if runner.requests[0].Mode != "single_pass" {
		t.Fatalf("first mode = %q, want %q", runner.requests[0].Mode, "single_pass")
	}
	if runner.requests[1].Mode != "tdd_fresh_context" {
		t.Fatalf("second mode = %q, want %q", runner.requests[1].Mode, "tdd_fresh_context")
	}
}

func TestRunModesInIsolatedWorktrees_ExecutesSingleModeManifest(t *testing.T) {
	resolver := &stubBaseCommitResolver{resolved: "abc123"}
	runner := &recordingModeRunner{}

	_, _, err := RunModesInIsolatedWorktrees(context.Background(), RunModesInput{
		Manifest: HarnessManifest{
			Provider:        "openai",
			ModelFamily:     "gpt-5",
			LowTierModel:    "gpt-5.1-codex-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
			Modes:           []string{"tdd_fresh_context"},
		},
		SelectedBeads:  []string{"gromit-1", "gromit-2", "gromit-3"},
		BaseCommitHint: "HEAD",
		Resolver:       resolver,
		Runner:         runner,
	})
	if err != nil {
		t.Fatalf("RunModesInIsolatedWorktrees() error = %v", err)
	}
	if len(runner.requests) != 1 {
		t.Fatalf("mode run requests = %d, want 1 (single mode manifest)", len(runner.requests))
	}
	if runner.requests[0].Mode != "tdd_fresh_context" {
		t.Fatalf("executed mode = %q, want %q", runner.requests[0].Mode, "tdd_fresh_context")
	}
}

func TestRunModesInIsolatedWorktrees_ExecutesFullModeManifest(t *testing.T) {
	resolver := &stubBaseCommitResolver{resolved: "abc123"}
	runner := &recordingModeRunner{}

	_, _, err := RunModesInIsolatedWorktrees(context.Background(), RunModesInput{
		Manifest: HarnessManifest{
			Provider:        "openai",
			ModelFamily:     "gpt-5",
			LowTierModel:    "gpt-5.1-codex-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
			Modes:           []string{"single_pass", "tdd_shared_context", "tdd_fresh_context"},
		},
		SelectedBeads:  []string{"gromit-1", "gromit-2", "gromit-3"},
		BaseCommitHint: "HEAD",
		Resolver:       resolver,
		Runner:         runner,
	})
	if err != nil {
		t.Fatalf("RunModesInIsolatedWorktrees() error = %v", err)
	}
	if len(runner.requests) != 3 {
		t.Fatalf("mode run requests = %d, want 3 (all three modes)", len(runner.requests))
	}
	expectedModes := []string{"single_pass", "tdd_shared_context", "tdd_fresh_context"}
	for i, expectedMode := range expectedModes {
		if runner.requests[i].Mode != expectedMode {
			t.Fatalf("mode[%d] = %q, want %q", i, runner.requests[i].Mode, expectedMode)
		}
	}
}

func TestRunModesInIsolatedWorktrees_IgnoresRunModesInputModesField(t *testing.T) {
	// Regression test: ensure that RunModesInput.Modes field is ignored
	// and manifest.Modes is the sole source of truth
	resolver := &stubBaseCommitResolver{resolved: "abc123"}
	runner := &recordingModeRunner{}

	_, _, err := RunModesInIsolatedWorktrees(context.Background(), RunModesInput{
		Manifest: HarnessManifest{
			Provider:        "openai",
			ModelFamily:     "gpt-5",
			LowTierModel:    "gpt-5.1-codex-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
			Modes:           []string{"single_pass"},
		},
		Modes:          []string{"tdd_shared_context", "tdd_fresh_context"},
		SelectedBeads:  []string{"gromit-1", "gromit-2", "gromit-3"},
		BaseCommitHint: "HEAD",
		Resolver:       resolver,
		Runner:         runner,
	})
	if err != nil {
		t.Fatalf("RunModesInIsolatedWorktrees() error = %v", err)
	}
	// Should execute only the manifest-declared mode, not the Modes field
	if len(runner.requests) != 1 {
		t.Fatalf("mode run requests = %d, want 1 (manifest mode, not input modes field)", len(runner.requests))
	}
	if runner.requests[0].Mode != "single_pass" {
		t.Fatalf("executed mode = %q, want %q (manifest declared)", runner.requests[0].Mode, "single_pass")
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
	callIndex              int
	failedModeCleanupCalls int
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
