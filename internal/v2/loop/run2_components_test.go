package loop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagevalidate "github.com/danabrams/gromit/internal/v2/stage/validate"
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
