package loop

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagevalidate "github.com/danabrams/gromit/internal/v2/stage/validate"
	"github.com/danabrams/gromit/internal/v2/testutil"
)

func TestCommandValidationRunnerExecutesShellCommand(t *testing.T) {
	t.Parallel()

	runner := NewCommandValidationRunner(".")
	if runner == nil {
		t.Fatal("CommandValidationRunner should not be nil")
	}

	// Test successful command
	err := runner.Run(context.Background(), "true", ".")
	if err != nil {
		t.Fatalf("true command failed: %v", err)
	}

	// Test failing command
	err = runner.Run(context.Background(), "false", ".")
	if err == nil {
		t.Fatal("false command should have failed")
	}
}

func TestCommandValidationRunnerCanExecuteEcho(t *testing.T) {
	t.Parallel()

	runner := NewCommandValidationRunner(".")

	// Test echo command
	err := runner.Run(context.Background(), "echo 'hello'", ".")
	if err != nil {
		t.Fatalf("echo command failed: %v", err)
	}
}

func TestValidateStageWithCommandRunnerRejectsFailing(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Validation.Commands = []string{"false"}

	runner := NewCommandValidationRunner(".")
	stage, err := stagevalidate.New(cfg, runner)
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}

	req := &stagepkg.Request{Config: cfg}
	res, err := stage.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}

	// Verify the validate stage rejected the failing command
	if res == nil || res.Decision != stagepkg.DecisionFail {
		t.Fatalf("expected DecisionFail, got %v", res)
	}
}

func TestValidateStageWithNoopRunnerIgnoresFailure(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Validation.Commands = []string{"false"}

	// The noop runner always returns nil, so even a "false" command succeeds
	runner := &noopValidationRunner{}
	stage, err := stagevalidate.New(cfg, runner)
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}

	req := &stagepkg.Request{Config: cfg}
	res, err := stage.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}

	// The noop runner always succeeds, even with failing commands
	if res == nil || res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("expected DecisionProceed with noop, got %v", res)
	}
}

func TestLoadProjectContextFromCLAUDEMD(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create mock CLAUDE.md
	claudeContent := "# Project Context\nProject-specific instructions for Claude"
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(claudeContent), 0644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}

	// loadProjectContext should read and return CLAUDE.md content
	content, err := loadProjectContext(tmpDir)
	if err != nil {
		t.Fatalf("loadProjectContext error: %v", err)
	}

	if content == "" {
		t.Fatal("expected non-empty content from CLAUDE.md, got empty string")
	}

	if !strings.Contains(content, claudeContent) {
		t.Fatalf("expected content to match CLAUDE.md, got: %q", content)
	}
}

func TestLoadBaseInstructionsFromRULES(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create mock RULES.md
	rulesContent := "# Base Rules\nBase instructions for build phase"
	if err := os.WriteFile(filepath.Join(tmpDir, "RULES.md"), []byte(rulesContent), 0644); err != nil {
		t.Fatalf("write RULES.md: %v", err)
	}

	// loadBaseInstructions should read and return RULES.md content
	content, err := loadBaseInstructions(tmpDir)
	if err != nil {
		t.Fatalf("loadBaseInstructions error: %v", err)
	}

	if content == "" {
		t.Fatal("expected non-empty content from RULES.md, got empty string")
	}

	if !strings.Contains(content, rulesContent) {
		t.Fatalf("expected content to match RULES.md, got: %q", content)
	}
}

func TestLoadMethodologyFragments(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create mock methodology fragment files
	standardContent := "# Standard methodology\nStandard build approach"
	tddContent := "# TDD methodology\nRed-green-refactor approach"
	refactorContent := "# Refactor methodology\nCode quality improvement approach"

	if err := os.WriteFile(filepath.Join(tmpDir, "build_standard.md"), []byte(standardContent), 0644); err != nil {
		t.Fatalf("write build_standard.md: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "build_tdd.md"), []byte(tddContent), 0644); err != nil {
		t.Fatalf("write build_tdd.md: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "build_refactor.md"), []byte(refactorContent), 0644); err != nil {
		t.Fatalf("write build_refactor.md: %v", err)
	}

	// loadMethodologyFragments should read and return all fragments
	fragments, err := loadMethodologyFragments(tmpDir)
	if err != nil {
		t.Fatalf("loadMethodologyFragments error: %v", err)
	}

	if fragments.Standard == "" {
		t.Fatal("expected non-empty Standard fragment, got empty string")
	}
	if !strings.Contains(fragments.Standard, standardContent) {
		t.Fatalf("expected Standard to match build_standard.md, got: %q", fragments.Standard)
	}

	if fragments.TDD == "" {
		t.Fatal("expected non-empty TDD fragment, got empty string")
	}
	if !strings.Contains(fragments.TDD, tddContent) {
		t.Fatalf("expected TDD to match build_tdd.md, got: %q", fragments.TDD)
	}

	if fragments.Refactor == "" {
		t.Fatal("expected non-empty Refactor fragment, got empty string")
	}
	if !strings.Contains(fragments.Refactor, refactorContent) {
		t.Fatalf("expected Refactor to match build_refactor.md, got: %q", fragments.Refactor)
	}
}

func TestNewRun2LoopComponentsWiring(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	cfg := &config.Config{
		ProjectRoot: tmpDir,
	}

	adapters := adapter.AdapterSet{
		Git:         testutil.NewFakeGit(),
		LLM:         newFakeLLMAdapter(),
		TaskTracker: testutil.NewFakeTaskTracker(),
		Presenter:   testutil.NewFakePresenter(),
	}

	legacyEmitter := events.NewEmitter()
	defer legacyEmitter.Close()

	var output bytes.Buffer

	components, err := NewRun2LoopComponents(cfg, adapters, legacyEmitter, &output)
	if err != nil {
		t.Fatalf("NewRun2LoopComponents returned error: %v", err)
	}
	if components == nil {
		t.Fatal("expected non-nil components")
	}

	// Verify all stages are wired (non-nil).
	if components.PlanStage == nil {
		t.Error("PlanStage is nil")
	}
	if components.PresentStage == nil {
		t.Error("PresentStage is nil")
	}
	if components.PresentSummaryContext == nil {
		t.Error("PresentSummaryContext is nil")
	}
	if components.DecomposeStage == nil {
		t.Error("DecomposeStage is nil")
	}
	if components.BeadLoop == nil {
		t.Error("BeadLoop is nil")
	}
	if components.AcceptStage == nil {
		t.Error("AcceptStage is nil")
	}
	if components.RemediationRunner == nil {
		t.Error("RemediationRunner is nil")
	}
	if components.Emitter == nil {
		t.Error("Emitter is nil")
	}

	// Verify triage and decompose stages are wired into the bead loop.
	if components.BeadLoop.triage == nil {
		t.Error("BeadLoop.triage is nil")
	}
	if components.BeadLoop.decompose == nil {
		t.Error("BeadLoop.decompose is nil")
	}
}

func TestLoadPlanFragment(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	planContent := "# Plan Instructions\nProduce an implementation plan"
	if err := os.WriteFile(filepath.Join(tmpDir, "plan_v2.md"), []byte(planContent), 0644); err != nil {
		t.Fatalf("write plan_v2.md: %v", err)
	}

	content, err := loadPlanFragment(tmpDir)
	if err != nil {
		t.Fatalf("loadPlanFragment error: %v", err)
	}

	if content == "" {
		t.Fatal("expected non-empty content from plan_v2.md, got empty string")
	}

	if !strings.Contains(content, planContent) {
		t.Fatalf("expected content to match plan_v2.md, got: %q", content)
	}
}

func TestLoadPlanFragmentReturnsEmptyOnMissingFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	content, err := loadPlanFragment(tmpDir)
	if err != nil {
		t.Fatalf("loadPlanFragment error: %v", err)
	}

	if content != "" {
		t.Fatalf("expected empty string for missing plan_v2.md, got: %q", content)
	}
}

func TestLoadMethodologyFragmentsReturnsFuncZeroOnMissingFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// No files created, all are missing

	// loadMethodologyFragments should gracefully handle missing files
	fragments, err := loadMethodologyFragments(tmpDir)
	if err != nil {
		t.Fatalf("loadMethodologyFragments error: %v", err)
	}

	// Should return zero-valued PromptFragments
	if fragments.Standard != "" || fragments.TDD != "" || fragments.Refactor != "" {
		t.Fatalf("expected empty fragments for missing files, got: %+v", fragments)
	}
}
