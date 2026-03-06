package loop

import (
	"context"
	"testing"
)

func TestCommandValidationRunnerExecutesShellCommand(t *testing.T) {
	t.Parallel()

	runner := NewCommandValidationRunner(".")
	if runner == nil {
		t.Fatal("CommandValidationRunner should not be nil")
	}

	// Test successful command
	err := runner.Run(context.Background(), "true")
	if err != nil {
		t.Fatalf("true command failed: %v", err)
	}

	// Test failing command
	err = runner.Run(context.Background(), "false")
	if err == nil {
		t.Fatal("false command should have failed")
	}
}

func TestCommandValidationRunnerCanExecuteEcho(t *testing.T) {
	t.Parallel()

	runner := NewCommandValidationRunner(".")

	// Test echo command
	err := runner.Run(context.Background(), "echo 'hello'")
	if err != nil {
		t.Fatalf("echo command failed: %v", err)
	}
}
