package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner"
)

// setupRunSpecTestEnv creates a minimal test environment for run command spec flag tests.
func setupRunSpecTestEnv(t *testing.T) (specsDir string, deps *runDeps, cleanup func()) {
	t.Helper()

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory before setup: %v", err)
	}

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

	t.Chdir(tempDir)

	configPath = "gromit.yaml"
	depsValue := newRunDeps()
	depsValue.runInDedicatedWorktree = func(_ context.Context, _ string, fn func() error) error {
		return fn()
	}
	deps = &depsValue

	var cleanupOnce sync.Once
	cleanup = func() {
		cleanupOnce.Do(func() {
			configPath = origConfigPath
			runSpecFlag = origRunSpec
			runEpicFlag = origRunEpic
			if err := os.Chdir(origWD); err != nil {
				t.Fatalf("restoring working directory: %v", err)
			}
		})
	}

	return specsDir, deps, cleanup
}

func TestRunSpecTestEnvSeedsExplicitProfile(t *testing.T) {
	_, _, cleanup := setupRunSpecTestEnv(t)
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
	_, _, cleanup := setupRunSpecTestEnv(t)
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

	_, _, cleanup := setupRunSpecTestEnv(t)
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
	_, deps, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	deps.newBuildSpecStageContext = func(ctx context.Context, cfg *config.Config, specName, gromitDir string) (*runner.StageContext, error) {
		if specName != "auth" {
			t.Fatalf("expected spec name auth, got %q", specName)
		}
		if gromitDir == "" {
			t.Fatalf("expected gromitDir to be set")
		}
		return &runner.StageContext{SpecName: "auth"}, nil
	}

	var branches []string
	deps.newSpecBranchCreator = func(repoDir string, cfg *config.Config) (runner.SpecBranchCreator, error) {
		return &fakeBranchCreator{branches: &branches}, nil
	}

	var stageCtxCalled bool
	deps.newRunnerWithStageContext = func(cfg *config.Config, output io.Writer, stageCtx *runner.StageContext, labels ...string) (*runner.Orchestrator, error) {
		stageCtxCalled = true
		if stageCtx == nil || stageCtx.SpecName != "auth" {
			t.Fatalf("expected auth stage context, got %+v", stageCtx)
		}
		return nil, fmt.Errorf("runner stub")
	}

	runSpecFlag = "auth"
	runEpicFlag = ""

	specPath := filepath.Join(".gromit", "specs", "auth.md")
	if err := os.WriteFile(specPath, []byte("# auth spec"), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	err := deps.runLoop(runCmd, []string{})
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
	_, deps, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	deps.newBuildSpecStageContext = func(ctx context.Context, cfg *config.Config, specName, gromitDir string) (*runner.StageContext, error) {
		return &runner.StageContext{SpecName: specName, FreshStart: true}, nil
	}

	var capturedStageCtx *runner.StageContext
	deps.newRunnerWithStageContext = func(cfg *config.Config, output io.Writer, stageCtx *runner.StageContext, labels ...string) (*runner.Orchestrator, error) {
		capturedStageCtx = stageCtx
		return nil, fmt.Errorf("runner stub")
	}

	var branches []string
	deps.newSpecBranchCreator = func(repoDir string, cfg *config.Config) (runner.SpecBranchCreator, error) {
		return &fakeBranchCreator{branches: &branches}, nil
	}

	runSpecFlag = "auth"
	runEpicFlag = ""

	specPath := filepath.Join(".gromit", "specs", "auth.md")
	if err := os.WriteFile(specPath, []byte("# auth spec"), 0644); err != nil {
		t.Fatalf("Failed to write spec file for resume test: %v", err)
	}
	if info, err := os.Stat(specPath); err != nil {
		t.Fatalf("spec file missing after creation: %v", err)
	} else if info.IsDir() {
		t.Fatalf("spec path is a directory, expected file")
	}

	err := deps.runLoop(runCmd, []string{})
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
	_, deps, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	runSpecFlag = "auth"
	runEpicFlag = "gromit-123"

	err := deps.runLoop(runCmd, []string{})
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
	specsDir, deps, cleanup := setupRunSpecTestEnv(t)
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

	err := deps.runLoop(runCmd, []string{})

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
	specsDir, deps, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	// Create the spec file
	specPath := filepath.Join(specsDir, "auth.md")
	if err := os.WriteFile(specPath, []byte("# auth spec"), 0644); err != nil {
		t.Fatalf("Failed to write spec: %v", err)
	}

	runSpecFlag = "auth"
	runEpicFlag = ""

	err := deps.runLoop(runCmd, []string{})

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

func TestRunLoop_SpecFlagMissingSpecDoesNotFallbackToLegacyLabel(t *testing.T) {
	_, deps, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	deps.runHasOpenBeadsForLabel = func(label string) (bool, error) {
		return label == "spec:review-revisions", nil
	}

	runSpecFlag = "review-revisions"
	runEpicFlag = ""

	err := deps.runLoop(runCmd, []string{})
	if err == nil {
		t.Fatal("runLoop with missing --spec should return validation error")
	}
	if !strings.Contains(err.Error(), "validating spec:") {
		t.Fatalf("expected spec validation error for missing spec, got: %v", err)
	}
}

func TestRunLoop_SpecFlagMissingSpecDoesNotFallbackWhenStrictLegacyFallbackEnabled(t *testing.T) {
	_, deps, cleanup := setupRunSpecTestEnv(t)
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

	deps.runHasOpenBeadsForLabel = func(label string) (bool, error) {
		return label == "spec:review-revisions", nil
	}

	runSpecFlag = "review-revisions"
	runEpicFlag = ""

	err := deps.runLoop(runCmd, []string{})
	if err == nil {
		t.Fatal("runLoop with strict legacy fallback should return spec validation error for missing spec")
	}
	if !strings.Contains(err.Error(), "validating spec:") {
		t.Fatalf("expected spec validation error under strict compatibility, got: %v", err)
	}
}

func TestRunLoop_NoSpecFlagSkipsSpecflowBootstrapping(t *testing.T) {
	_, deps, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	var storeCalls int
	deps.newBuildSpecStageContext = func(ctx context.Context, cfg *config.Config, specName, gromitDir string) (*runner.StageContext, error) {
		storeCalls++
		return &runner.StageContext{}, nil
	}

	var capturedStageCtx *runner.StageContext
	deps.newRunnerWithStageContext = func(cfg *config.Config, output io.Writer, stageCtx *runner.StageContext, labels ...string) (*runner.Orchestrator, error) {
		capturedStageCtx = stageCtx
		return nil, fmt.Errorf("runner stub")
	}

	var branches []string
	deps.newSpecBranchCreator = func(repoDir string, cfg *config.Config) (runner.SpecBranchCreator, error) {
		return &fakeBranchCreator{branches: &branches}, nil
	}

	runSpecFlag = ""
	runEpicFlag = ""

	err := deps.runLoop(runCmd, []string{})
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
	_, deps, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	deps.newBuildSpecStageContext = func(ctx context.Context, cfg *config.Config, specName, gromitDir string) (*runner.StageContext, error) {
		return &runner.StageContext{SpecName: specName, FreshStart: false}, nil
	}

	var capturedStageCtx *runner.StageContext
	deps.newRunnerWithStageContext = func(cfg *config.Config, output io.Writer, stageCtx *runner.StageContext, labels ...string) (*runner.Orchestrator, error) {
		capturedStageCtx = stageCtx
		return nil, fmt.Errorf("runner stub")
	}

	var branches []string
	deps.newSpecBranchCreator = func(repoDir string, cfg *config.Config) (runner.SpecBranchCreator, error) {
		return &fakeBranchCreator{branches: &branches}, nil
	}

	runSpecFlag = "auth"
	runEpicFlag = ""

	specPath := filepath.Join(".gromit", "specs", "auth.md")
	if err := os.WriteFile(specPath, []byte("# auth spec"), 0644); err != nil {
		t.Fatalf("Failed to write spec file for resume test: %v", err)
	}
	if info, err := os.Stat(specPath); err != nil {
		t.Fatalf("spec file missing after creation: %v", err)
	} else if info.IsDir() {
		t.Fatalf("spec path is a directory, expected file")
	}

	err := deps.runLoop(runCmd, []string{})
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
	_, deps, cleanup := setupRunSpecTestEnv(t)
	defer cleanup()

	// Remove .gromit/specs to force ResolveEpic error path.
	if err := os.RemoveAll(filepath.Join(".gromit", "specs")); err != nil {
		t.Fatalf("failed removing specs dir: %v", err)
	}

	runSpecFlag = ""
	runEpicFlag = "gromit-123"

	err := deps.runLoop(runCmd, []string{})
	if err == nil {
		t.Fatal("runLoop with --epic and missing specs dir should return error")
	}
	if !strings.Contains(err.Error(), "resolving epic scope:") {
		t.Fatalf("expected epic resolution error, got: %v", err)
	}
}

func TestRunLoop_EpicFlagValidScope(t *testing.T) {
	specsDir, deps, cleanup := setupRunSpecTestEnv(t)
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

	err := deps.runLoop(runCmd, []string{})
	if err != nil && strings.Contains(err.Error(), "resolving epic scope:") {
		t.Fatalf("runLoop should not fail epic scope resolution for a valid epic: %v", err)
	}
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
