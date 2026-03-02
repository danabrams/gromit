package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner"
	"github.com/danabrams/gromit/internal/specflow"
)

// setupRunSpecTestEnv creates a minimal test environment for run command spec flag tests.
func setupRunSpecTestEnv(t *testing.T) (specsDir string, cleanup func()) {
	t.Helper()

	tempDir := t.TempDir()
	gromitDir := filepath.Join(tempDir, ".gromit")
	specsDir = filepath.Join(gromitDir, "specs")

	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Write minimal gromit.yaml (seed explicit profile to match current resolver expectations)
	cfgPath := filepath.Join(tempDir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(configForProfile("go")), 0644); err != nil {
		t.Fatalf("Failed to write gromit.yaml: %v", err)
	}

	origConfigPath := configPath
	origRunSpec := runSpecFlag
	origRunEpic := runEpicFlag
	origRunHasOpenBeadsForLabel := runHasOpenBeadsForLabelFn

	t.Chdir(tempDir)

	configPath = "gromit.yaml"

	cleanup = func() {
		configPath = origConfigPath
		runSpecFlag = origRunSpec
		runEpicFlag = origRunEpic
		runHasOpenBeadsForLabelFn = origRunHasOpenBeadsForLabel
	}

	return specsDir, cleanup
}

func TestRunSpecTestEnvSeedsExplicitProfile(t *testing.T) {
	_, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config fixture: %v", err)
	}
	if !strings.Contains(string(data), "project:\n  profile: \"go\"") {
		t.Fatalf("expected seeded gromit.yaml to declare explicit profile for profile-aware paths, got:\n%s", string(data))
	}
}

func TestRunSpecTestEnvGoProfileValidationCommands(t *testing.T) {
	_, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config fixture: %v", err)
	}
	cfg := string(data)
	if !strings.Contains(cfg, "- \"go test\"") {
		t.Fatalf("expected go test command in seeded config, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "- \"go build\"") {
		t.Fatalf("expected go build command in seeded config, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "- \"go vet\"") {
		t.Fatalf("expected go vet command in seeded config, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "compile_command: \"go build\"") {
		t.Fatalf("expected go build compile command in seeded config, got:\n%s", cfg)
	}
}

func TestRunSpecTestEnvRestoresWorkingDirectory(t *testing.T) {
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory before setup: %v", err)
	}

	_, cleanup := setupRunSpecTestEnv(t)
	cleanup()

	currentWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory after cleanup: %v", err)
	}
	if currentWD != originalWD {
		t.Fatalf("expected working directory %q after cleanup, got %q", originalWD, currentWD)
	}
}

// RED: We should be able to swap the specflow and branch factories for spec runs via
// the injection globals so the run loop stays testable even if the real factories
// are expensive or rely on git state.
func TestRunLoop_SpecFlagInjectableFactories(t *testing.T) {
	_, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	fakeStore := &fakeSpecflowStore{stageErr: specflow.ErrStageNotFound}
	origStoreFn := newSpecflowStoreFn
	newSpecflowStoreFn = func(gromitDir string) (specflow.SpecStore, error) {
		if gromitDir == "" {
			t.Fatalf("expected gromitDir to be set")
		}
		return fakeStore, nil
	}
	defer func() { newSpecflowStoreFn = origStoreFn }()

	var branches []string
	origBranchFn := newSpecBranchCreatorFn
	newSpecBranchCreatorFn = func(repoDir string, cfg *config.Config) (runner.SpecBranchCreator, error) {
		return &fakeBranchCreator{branches: &branches}, nil
	}
	defer func() { newSpecBranchCreatorFn = origBranchFn }()

	var stageCtxCalled bool
	origRunnerFn := newRunnerWithStageContextFn
	newRunnerWithStageContextFn = func(cfg *config.Config, output io.Writer, stageCtx *runner.StageContext, labels ...string) (*runner.Orchestrator, error) {
		stageCtxCalled = true
		if stageCtx == nil || stageCtx.SpecName != "auth" {
			t.Fatalf("expected auth stage context, got %+v", stageCtx)
		}
		return nil, fmt.Errorf("runner stub")
	}
	defer func() { newRunnerWithStageContextFn = origRunnerFn }()

	runSpecFlag = "auth"
	runEpicFlag = ""

	specPath := filepath.Join(".gromit", "specs", "auth.md")
	if err := os.WriteFile(specPath, []byte("# auth spec"), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	err := runLoop(runCmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "runner stub") {
		t.Fatalf("expected runner stub, got %v", err)
	}
	if !stageCtxCalled {
		t.Fatal("expected new runner to see the injected stage context")
	}
	if len(branches) != 1 {
		t.Fatalf("branch creator called %d times, want 1", len(branches))
	}
	if branches[0] != "gromit/spec-auth" {
		t.Fatalf("unexpected branch created: %s", branches[0])
	}
}

// RED: When --spec is provided for the first time, the run-loop should bootstrap
// specflow stage context, branch creation, and pass the context into the runner.
func TestRunLoop_SpecFlagFreshStartBootstrapsStageAndBranch(t *testing.T) {
	_, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	fakeStore := &fakeSpecflowStore{stageErr: specflow.ErrStageNotFound}
	origStoreFactory := runner.SpecflowStoreFactory
	runner.SpecflowStoreFactory = func(gromitDir string) (specflow.SpecStore, error) {
		return fakeStore, nil
	}
	defer func() { runner.SpecflowStoreFactory = origStoreFactory }()

	var capturedStageCtx *runner.StageContext
	origRunnerFn := newRunnerWithStageContextFn
	newRunnerWithStageContextFn = func(cfg *config.Config, output io.Writer, stageCtx *runner.StageContext, labels ...string) (*runner.Orchestrator, error) {
		capturedStageCtx = stageCtx
		return nil, fmt.Errorf("runner stub")
	}
	defer func() { newRunnerWithStageContextFn = origRunnerFn }()

	var branches []string
	origBranchFactory := runner.SpecBranchCreatorFactory
	runner.SpecBranchCreatorFactory = func(repoDir string, cfg *config.Config) (runner.SpecBranchCreator, error) {
		return &fakeBranchCreator{branches: &branches}, nil
	}
	defer func() { runner.SpecBranchCreatorFactory = origBranchFactory }()

	runSpecFlag = "auth"
	runEpicFlag = ""

	if err := os.WriteFile(filepath.Join(".gromit", "specs", "auth.md"), []byte("# auth spec"), 0644); err != nil {
		t.Fatalf("Failed to write spec file for resume test: %v", err)
	}
	if info, err := os.Stat(filepath.Join(".gromit", "specs", "auth.md")); err != nil {
		t.Fatalf("spec file missing after creation: %v", err)
	} else if info.IsDir() {
		t.Fatalf("spec path is a directory, expected file")
	}

	specPath := filepath.Join(".gromit", "specs", "auth.md")
	if err := os.WriteFile(specPath, []byte("# auth spec"), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	err := runLoop(runCmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "runner stub") {
		t.Fatalf("expected runner stub error, got %v", err)
	}
	if capturedStageCtx == nil {
		t.Fatal("expected stage context passed to runner")
	}
	if !capturedStageCtx.FreshStart {
		t.Fatalf("expected fresh start stage context, got %+v", capturedStageCtx)
	}
	if capturedStageCtx.SpecName != "auth" {
		t.Fatalf("expected spec name auth, got %q", capturedStageCtx.SpecName)
	}
	if len(branches) != 1 {
		t.Fatalf("expected branch creation once, got %d", len(branches))
	}
	if branches[0] != "gromit/spec-auth" {
		t.Fatalf("unexpected branch: %s", branches[0])
	}
}

// TestRunCmd_ScopeFlagsRegistered verifies that --spec and --epic flags are registered.
func TestRunCmd_ScopeFlagsRegistered(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "spec"},
		{name: "epic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := runCmd.Flags().Lookup(tt.name)
			if flag == nil {
				t.Fatalf("run command should have --%s flag", tt.name)
			}
			if flag.Value.Type() != "string" {
				t.Errorf("--%s flag should be string, got %s", tt.name, flag.Value.Type())
			}
		})
	}
}

func TestRunLoop_ScopeFlagsMutuallyExclusive(t *testing.T) {
	_, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	runSpecFlag = "auth"
	runEpicFlag = "gromit-123"

	err := runLoop(runCmd, []string{})
	if err == nil {
		t.Fatal("runLoop should fail when --spec and --epic are both set")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected mutually exclusive error, got: %v", err)
	}
}

// TestRunLoop_SpecFlagNonexistentSpec verifies that --spec with a nonexistent spec
// returns a validation error before attempting to run.
func TestRunLoop_SpecFlagNonexistentSpec(t *testing.T) {
	specsDir, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	// Create some spec files so the error can list them
	for _, name := range []string{"auth", "profile"} {
		path := filepath.Join(specsDir, name+".md")
		if err := os.WriteFile(path, []byte("# "+name), 0644); err != nil {
			t.Fatalf("Failed to write spec: %v", err)
		}
	}

	runSpecFlag = "nonexistent-spec"
	runEpicFlag = ""

	err := runLoop(runCmd, []string{})

	if err == nil {
		t.Fatal("runLoop with nonexistent --spec should return error")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "not found") {
		t.Errorf("Error should say 'not found', got: %v", err)
	}
	if !strings.Contains(errMsg, "nonexistent-spec") {
		t.Errorf("Error should mention the requested spec name, got: %v", err)
	}
}

// TestRunLoop_SpecFlagValidSpec verifies that a valid --spec passes validation
// and that the error is NOT a spec validation error (i.e., we proceed to the runner).
func TestRunLoop_SpecFlagValidSpec(t *testing.T) {
	specsDir, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	// Create the spec file
	specPath := filepath.Join(specsDir, "auth.md")
	if err := os.WriteFile(specPath, []byte("# auth spec"), 0644); err != nil {
		t.Fatalf("Failed to write spec: %v", err)
	}

	runSpecFlag = "auth"
	runEpicFlag = ""

	err := runLoop(runCmd, []string{})

	// May fail for other reasons (no bd cli, etc), but must NOT fail due to spec validation
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "validating spec:") {
			t.Errorf("Error should not be spec validation error for existing spec, got: %v", err)
		}
		if strings.Contains(errMsg, "Available specs") {
			t.Errorf("Error should not list available specs for valid spec, got: %v", err)
		}
	}
}

func TestRunLoop_SpecFlagMissingSpecFallsBackToLegacyLabelWhenOpenBeadsExist(t *testing.T) {
	_, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	runHasOpenBeadsForLabelFn = func(label string) (bool, error) {
		return label == "spec:review-revisions", nil
	}

	runSpecFlag = "review-revisions"
	runEpicFlag = ""

	err := runLoop(runCmd, []string{})
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "validating spec:") {
			t.Errorf("Error should not be spec validation error when legacy fallback applies, got: %v", err)
		}
		if strings.Contains(errMsg, "Available specs") {
			t.Errorf("Error should not list available specs when legacy fallback applies, got: %v", err)
		}
	}
}

func TestRunLoop_SpecFlagMissingSpecDoesNotFallbackWhenStrictLegacyFallbackEnabled(t *testing.T) {
	_, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	strictCfg := `project:
  profile: go
tracker:
  backend: bd
methodology:
  adapter: go
compatibility:
  strict_legacy_fallback: true
`
	if err := os.WriteFile(configPath, []byte(strictCfg), 0644); err != nil {
		t.Fatalf("Failed to write strict compatibility config: %v", err)
	}

	runHasOpenBeadsForLabelFn = func(label string) (bool, error) {
		return label == "spec:review-revisions", nil
	}

	runSpecFlag = "review-revisions"
	runEpicFlag = ""

	err := runLoop(runCmd, []string{})
	if err == nil {
		t.Fatal("runLoop with strict legacy fallback should return spec validation error for missing spec")
	}
	if !strings.Contains(err.Error(), "validating spec:") {
		t.Fatalf("expected spec validation error under strict compatibility, got: %v", err)
	}
}

func TestRunLoop_NoSpecFlagSkipsSpecflowBootstrapping(t *testing.T) {
	_, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	var storeCalls int
	origStoreFactory := runner.SpecflowStoreFactory
	runner.SpecflowStoreFactory = func(gromitDir string) (specflow.SpecStore, error) {
		storeCalls++
		return &fakeSpecflowStore{}, nil
	}
	defer func() { runner.SpecflowStoreFactory = origStoreFactory }()

	var capturedStageCtx *runner.StageContext
	origRunnerFn := newRunnerWithStageContextFn
	newRunnerWithStageContextFn = func(cfg *config.Config, output io.Writer, stageCtx *runner.StageContext, labels ...string) (*runner.Orchestrator, error) {
		capturedStageCtx = stageCtx
		return nil, fmt.Errorf("runner stub")
	}
	defer func() { newRunnerWithStageContextFn = origRunnerFn }()

	var branches []string
	origBranchFactory := runner.SpecBranchCreatorFactory
	runner.SpecBranchCreatorFactory = func(repoDir string, cfg *config.Config) (runner.SpecBranchCreator, error) {
		return &fakeBranchCreator{branches: &branches}, nil
	}
	defer func() { runner.SpecBranchCreatorFactory = origBranchFactory }()

	runSpecFlag = ""
	runEpicFlag = ""

	err := runLoop(runCmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "runner stub") {
		t.Fatalf("expected runner stub error, got %v", err)
	}
	if storeCalls != 0 {
		t.Fatalf("expected specflow store not to be created, got %d", storeCalls)
	}
	if capturedStageCtx != nil {
		t.Fatalf("expected no stage context for non-spec run, got %+v", capturedStageCtx)
	}
	if len(branches) != 0 {
		t.Fatalf("expected no branch creation for non-spec run, got %d", len(branches))
	}
}

func TestRunLoop_SpecFlagResumeChecksOutBranch(t *testing.T) {
	_, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	fakeStore := &fakeSpecflowStore{stage: specflow.StageDrafting}
	origStoreFactory := runner.SpecflowStoreFactory
	runner.SpecflowStoreFactory = func(gromitDir string) (specflow.SpecStore, error) {
		return fakeStore, nil
	}
	defer func() { runner.SpecflowStoreFactory = origStoreFactory }()

	var capturedStageCtx *runner.StageContext
	origRunnerFn := newRunnerWithStageContextFn
	newRunnerWithStageContextFn = func(cfg *config.Config, output io.Writer, stageCtx *runner.StageContext, labels ...string) (*runner.Orchestrator, error) {
		capturedStageCtx = stageCtx
		return nil, fmt.Errorf("runner stub")
	}
	defer func() { newRunnerWithStageContextFn = origRunnerFn }()

	var branches []string
	origBranchFactory := runner.SpecBranchCreatorFactory
	runner.SpecBranchCreatorFactory = func(repoDir string, cfg *config.Config) (runner.SpecBranchCreator, error) {
		return &fakeBranchCreator{branches: &branches}, nil
	}
	defer func() { runner.SpecBranchCreatorFactory = origBranchFactory }()

	runSpecFlag = "auth"
	runEpicFlag = ""

	if err := os.WriteFile(filepath.Join(".gromit", "specs", "auth.md"), []byte("# auth spec"), 0644); err != nil {
		t.Fatalf("Failed to write spec file for resume test: %v", err)
	}
	if info, err := os.Stat(filepath.Join(".gromit", "specs", "auth.md")); err != nil {
		t.Fatalf("spec file missing after creation: %v", err)
	} else if info.IsDir() {
		t.Fatalf("spec path is a directory, expected file")
	}

	err := runLoop(runCmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "runner stub") {
		t.Fatalf("expected runner stub error, got %v", err)
	}
	if capturedStageCtx == nil {
		t.Fatal("expected stage context for resumed spec run")
	}
	if capturedStageCtx.FreshStart {
		t.Fatalf("expected resumed stage context, got fresh start: %+v", capturedStageCtx)
	}
	if len(branches) != 1 {
		t.Fatalf("expected branch checkout once for resume, got %d", len(branches))
	}
	if branches[0] != "gromit/spec-auth" {
		t.Fatalf("unexpected branch: %s", branches[0])
	}
}

func TestRunLoop_EpicFlagMissingSpecsDir(t *testing.T) {
	_, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	// Remove .gromit/specs to force ResolveEpic error path.
	if err := os.RemoveAll(filepath.Join(".gromit", "specs")); err != nil {
		t.Fatalf("failed removing specs dir: %v", err)
	}

	runSpecFlag = ""
	runEpicFlag = "gromit-123"

	err := runLoop(runCmd, []string{})
	if err == nil {
		t.Fatal("runLoop with --epic and missing specs dir should return error")
	}
	if !strings.Contains(err.Error(), "resolving epic scope:") {
		t.Fatalf("expected epic resolution error, got: %v", err)
	}
}

func TestRunLoop_EpicFlagValidScope(t *testing.T) {
	specsDir, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	specContent := `---
id: auth
epic: gromit-123
---

# Auth spec
`
	if err := os.WriteFile(filepath.Join(specsDir, "auth.md"), []byte(specContent), 0644); err != nil {
		t.Fatalf("failed writing spec file: %v", err)
	}

	runSpecFlag = ""
	runEpicFlag = "gromit-123"

	err := runLoop(runCmd, []string{})
	if err != nil && strings.Contains(err.Error(), "resolving epic scope:") {
		t.Fatalf("runLoop should not fail epic scope resolution for a valid epic: %v", err)
	}
}

type fakeSpecflowStore struct {
	stage    specflow.Stage
	stageErr error
}

func (f *fakeSpecflowStore) Stage(_ context.Context, _ string) (specflow.Stage, error) {
	if f == nil {
		return "", specflow.ErrStageNotFound
	}
	if f.stageErr != nil {
		return "", f.stageErr
	}
	return f.stage, nil
}

func (f *fakeSpecflowStore) StoreStage(_ context.Context, _ string, stage specflow.Stage) error {
	if f == nil {
		return fmt.Errorf("fake store nil")
	}
	f.stage = stage
	return nil
}

type fakeBranchCreator struct {
	branches *[]string
}

func (f *fakeBranchCreator) CreateOrCheckoutSpecBranch(_ context.Context, specBranchName string) error {
	if f == nil || f.branches == nil {
		return fmt.Errorf("fake branch creator not initialized")
	}
	*f.branches = append(*f.branches, specBranchName)
	return nil
}
