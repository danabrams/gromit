package specloop

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestShellTaskInspector_NoProofChecks(t *testing.T) {
	dir := t.TempDir()
	inspector := NewShellTaskInspector(func() string { return dir })
	task := runstore.Task{TaskID: "t1", ProofChecks: []string{}}

	result := inspector.Inspect(context.Background(), task)

	if !result.Pass {
		t.Errorf("expected Pass=true for task with no proof checks, got false")
	}
	if len(result.Failures) != 0 {
		t.Errorf("expected no failures, got %v", result.Failures)
	}
}

func TestShellTaskInspector_NilProofChecks(t *testing.T) {
	dir := t.TempDir()
	inspector := NewShellTaskInspector(func() string { return dir })
	task := runstore.Task{TaskID: "t1", ProofChecks: nil}

	result := inspector.Inspect(context.Background(), task)

	if !result.Pass {
		t.Errorf("expected Pass=true for task with nil proof checks, got false")
	}
}

func TestShellTaskInspector_AllCheckPass(t *testing.T) {
	dir := t.TempDir()
	inspector := NewShellTaskInspector(func() string { return dir })
	task := runstore.Task{
		TaskID:      "t1",
		ProofChecks: []string{"true", "true"},
	}

	result := inspector.Inspect(context.Background(), task)

	if !result.Pass {
		t.Errorf("expected Pass=true when all checks pass, got false")
	}
	if len(result.Failures) != 0 {
		t.Errorf("expected no failures, got %v", result.Failures)
	}
}

func TestShellTaskInspector_SomeChecksFail(t *testing.T) {
	dir := t.TempDir()
	inspector := NewShellTaskInspector(func() string { return dir })
	task := runstore.Task{
		TaskID:      "t1",
		ProofChecks: []string{"true", "false"},
	}

	result := inspector.Inspect(context.Background(), task)

	if result.Pass {
		t.Errorf("expected Pass=false when some checks fail, got true")
	}
	if len(result.Failures) == 0 {
		t.Errorf("expected non-empty Failures, got none")
	}
}

func TestShellTaskInspector_AllChecksFail(t *testing.T) {
	dir := t.TempDir()
	inspector := NewShellTaskInspector(func() string { return dir })
	task := runstore.Task{
		TaskID:      "t1",
		ProofChecks: []string{"false", "false"},
	}

	result := inspector.Inspect(context.Background(), task)

	if result.Pass {
		t.Errorf("expected Pass=false when all checks fail, got true")
	}
	if len(result.Failures) != 2 {
		t.Errorf("expected 2 failures, got %d: %v", len(result.Failures), result.Failures)
	}
}

func TestShellTaskInspector_SatisfiesInterface(t *testing.T) {
	// Compile-time check: *ShellTaskInspector must satisfy TaskInspector.
	var _ TaskInspector = NewShellTaskInspector(func() string { return "/tmp" })
}
